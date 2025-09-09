package copilot

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"time"
)

// CopilotToken holds the structure of the Copilot token as stored in token.json.
type CopilotToken struct {
	Token     string  `json:"token"`
	ExpiresAt float64 `json:"expires_at"`
}

// TokenManager manages Copilot OAuth and GitHub tokens, handles refresh, file lock, and concurrency.
type TokenManager struct {
	mu            sync.RWMutex
	oauthToken    string
	githubToken   *CopilotToken
	configDir     string
	tokenFile     string
	authURL       string
	refreshCancel context.CancelFunc
	refreshWG     sync.WaitGroup
	isSelfWriting bool
}

// NewTokenManager creates a new TokenManager and initializes it.
func NewTokenManager(ctx context.Context) (*TokenManager, error) {
	configDir := getConfigDir()
	tokenFile := filepath.Join(configDir, "github-copilot", "token.json")
	authURL := "https://api.github.com/copilot_internal/v2/token"

	tm := &TokenManager{
		configDir: configDir,
		tokenFile: tokenFile,
		authURL:   authURL,
	}

	// Load OAuth token from config files
	oauthToken, err := tm.loadOAuthToken()
	if err != nil {
		return nil, fmt.Errorf("failed to load Copilot OAuth token: %w", err)
	}
	tm.oauthToken = oauthToken

	// Load GitHub token from file (if exists)
	_ = tm.loadTokenFromFile()

	// Start background refresh and file watcher
	refreshCtx, cancel := context.WithCancel(ctx)
	tm.refreshCancel = cancel
	tm.refreshWG.Add(2)
	go tm.refreshLoop(refreshCtx)
	go tm.watchTokenFile(refreshCtx)

	return tm, nil
}

// Close cleans up background goroutines.
func (tm *TokenManager) Close() {
	if tm.refreshCancel != nil {
		tm.refreshCancel()
	}
	tm.refreshWG.Wait()
}

// GetToken returns the current valid Copilot token, refreshing if needed.
func (tm *TokenManager) GetToken(ctx context.Context) (string, error) {
	tm.mu.RLock()
	token := tm.githubToken
	tm.mu.RUnlock()

	if token != nil && token.ExpiresAt > float64(time.Now().Unix())+120 {
		return token.Token, nil
	}

	// Refresh synchronously if token is missing or expired
	if err := tm.refreshToken(ctx, true); err != nil {
		return "", err
	}
	tm.mu.RLock()
	defer tm.mu.RUnlock()
	if tm.githubToken == nil {
		return "", errors.New("copilot token unavailable after refresh")
	}
	return tm.githubToken.Token, nil
}

// loadOAuthToken loads the OAuth token from apps.json or hosts.json.
func (tm *TokenManager) loadOAuthToken() (string, error) {
	for _, fname := range []string{"apps.json", "hosts.json"} {
		path := filepath.Join(tm.configDir, "github-copilot", fname)
		data, err := os.ReadFile(path)
		if err != nil {
			continue
		}
		var hosts map[string]struct {
			OAuthToken string `json:"oauth_token"`
		}
		if err := json.Unmarshal(data, &hosts); err != nil {
			continue
		}
		for host, v := range hosts {
			if strings.Contains(host, "github.com") && v.OAuthToken != "" {
				return v.OAuthToken, nil
			}
		}
	}
	return "", errors.New("GitHub OAuth token not found in config")
}

// loadTokenFromFile loads the GitHub token from token.json.
func (tm *TokenManager) loadTokenFromFile() error {
	tm.mu.Lock()
	defer tm.mu.Unlock()
	data, err := os.ReadFile(tm.tokenFile)
	if err != nil {
		return err
	}
	var token CopilotToken
	if err := json.Unmarshal(data, &token); err != nil {
		return err
	}
	tm.githubToken = &token
	return nil
}

// saveTokenToFile saves the GitHub token to token.json atomically.
func (tm *TokenManager) saveTokenToFile() error {
	tm.mu.RLock()
	token := tm.githubToken
	tm.mu.RUnlock()
	if token == nil {
		return errors.New("no token to save")
	}
	tempFile := tm.tokenFile + ".tmp"
	tm.isSelfWriting = true
	defer func() { tm.isSelfWriting = false }()
	data, err := json.Marshal(token)
	if err != nil {
		return err
	}
	if err := os.WriteFile(tempFile, data, 0600); err != nil {
		return err
	}
	return os.Rename(tempFile, tm.tokenFile)
}

// isTokenValid checks if the current token is valid (not expiring soon).
func (tm *TokenManager) isTokenValid() bool {
	tm.mu.RLock()
	defer tm.mu.RUnlock()
	return tm.githubToken != nil && tm.githubToken.ExpiresAt > float64(time.Now().Unix())+120
}

// refreshToken refreshes the Copilot token from the API, with file lock for concurrency.
func (tm *TokenManager) refreshToken(ctx context.Context, force bool) error {
	// If not forced, skip if token is valid
	if !force && tm.isTokenValid() {
		return nil
	}

	// Try to acquire file lock
	lockPath := tm.tokenFile + ".lock"
	lockAcquired := false
	for i := 0; i < 5; i++ {
		err := acquireLock(lockPath)
		if err == nil {
			lockAcquired = true
			break
		}
		time.Sleep(1 * time.Second)
	}
	if !lockAcquired {
		// Wait for another process to refresh
		time.Sleep(5 * time.Second)
		_ = tm.loadTokenFromFile()
		if tm.isTokenValid() {
			return nil
		}
		return errors.New("could not acquire lock to refresh Copilot token")
	}
	defer releaseLock(lockPath)

	// Send authentication request
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, tm.authURL, nil)
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "token "+tm.oauthToken)
	req.Header.Set("Accept", "application/json")
	req.Header.Set("Editor-Plugin-Version", "copilot.go")

	client := &http.Client{Timeout: 15 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("failed to refresh Copilot token: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		return fmt.Errorf("token refresh failed: %s", resp.Status)
	}
	var token CopilotToken
	if err := json.NewDecoder(resp.Body).Decode(&token); err != nil {
		return fmt.Errorf("failed to decode Copilot token: %w", err)
	}
	tm.mu.Lock()
	tm.githubToken = &token
	tm.mu.Unlock()
	if err := tm.saveTokenToFile(); err != nil {
		return fmt.Errorf("failed to save Copilot token: %w", err)
	}
	return nil
}

// refreshLoop periodically refreshes the Copilot token.
func (tm *TokenManager) refreshLoop(ctx context.Context) {
	defer tm.refreshWG.Done()
	for {
		select {
		case <-ctx.Done():
			return
		default:
			// Refresh token if needed
			_ = tm.refreshToken(ctx, false)
			// Sleep until 2 minutes before expiration, or 5 minutes if unknown
			tm.mu.RLock()
			var sleep time.Duration = 5 * time.Minute
			if tm.githubToken != nil {
				exp := int64(tm.githubToken.ExpiresAt)
				now := time.Now().Unix()
				if exp > now+120 {
					sleep = time.Duration(exp-now-120) * time.Second
				}
			}
			tm.mu.RUnlock()
			select {
			case <-ctx.Done():
				return
			case <-time.After(sleep):
			}
		}
	}
}

// watchTokenFile watches token.json for changes and reloads it.
func (tm *TokenManager) watchTokenFile(ctx context.Context) {
	defer tm.refreshWG.Done()
	lastMod := int64(0)
	for {
		select {
		case <-ctx.Done():
			return
		default:
			info, err := os.Stat(tm.tokenFile)
			if err == nil {
				mod := info.ModTime().Unix()
				if mod != lastMod {
					lastMod = mod
					if !tm.isSelfWriting {
						_ = tm.loadTokenFromFile()
					}
				}
			}
			// Clean up stale lock files
			lockPath := tm.tokenFile + ".lock"
			if info, err := os.Stat(lockPath); err == nil {
				age := time.Now().Unix() - info.ModTime().Unix()
				if age > 300 {
					_ = os.Remove(lockPath)
				}
			}
			time.Sleep(2 * time.Second)
		}
	}
}

// acquireLock tries to create a lock file for token refresh.
func acquireLock(lockPath string) error {
	f, err := os.OpenFile(lockPath, os.O_CREATE|os.O_EXCL, 0600)
	if err != nil {
		return err
	}
	return f.Close()
}

// releaseLock removes the lock file.
func releaseLock(lockPath string) {
	_ = os.Remove(lockPath)
}

// getConfigDir returns the OS-specific config directory.
func getConfigDir() string {
	if runtime.GOOS == "windows" {
		localAppData := os.Getenv("LOCALAPPDATA")
		if localAppData != "" {
			return localAppData
		}
		// fallback to home dir
		home, _ := os.UserHomeDir()
		return filepath.Join(home, "AppData", "Local")
	}
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".config")
}
