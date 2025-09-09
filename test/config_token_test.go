package test

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"copilot-api/pkg/config"
)

// Helper to create a temporary apps.json file for testing
func writeTempAppsJSON(t *testing.T, content string) (string, func()) {
	t.Helper()
	dir := t.TempDir()
	var configPath string
	if runtime.GOOS == "windows" {
		configPath = filepath.Join(dir, "github-copilot", "apps.json")
	} else {
		configPath = filepath.Join(dir, ".config", "github-copilot", "apps.json")
	}
	if err := os.MkdirAll(filepath.Dir(configPath), 0o755); err != nil {
		t.Fatalf("failed to create config dir: %v", err)
	}
	if err := os.WriteFile(configPath, []byte(content), 0o600); err != nil {
		t.Fatalf("failed to write apps.json: %v", err)
	}
	return configPath, func() { os.RemoveAll(dir) }
}

func TestFindCopilotToken_EnvVar(t *testing.T) {
	const wantToken = "env-token-123"
	os.Setenv("COPILOT_OAUTH_TOKEN", wantToken)
	defer os.Unsetenv("COPILOT_OAUTH_TOKEN")

	cfg, err := config.Load()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.CopilotOAuthToken != wantToken {
		t.Errorf("expected token from env var, got %q", cfg.CopilotOAuthToken)
	}
}

func TestFindCopilotToken_AppsJSON(t *testing.T) {
	const wantToken = "json-token-456"
	appsJSON := `{
		"github.com:Iv1.something": {
			"user": "testuser",
			"oauth_token": "` + wantToken + `",
			"githubAppId": "Iv1.something"
		}
	}`

	configPath, cleanup := writeTempAppsJSON(t, appsJSON)
	defer cleanup()

	// Unset env var to force file lookup
	os.Unsetenv("COPILOT_OAUTH_TOKEN")

	// Patch HOME or LOCALAPPDATA to point to our temp dir
	var restoreEnv func()
	if runtime.GOOS == "windows" {
		old := os.Getenv("LOCALAPPDATA")
		os.Setenv("LOCALAPPDATA", filepath.Dir(filepath.Dir(configPath)))
		restoreEnv = func() { os.Setenv("LOCALAPPDATA", old) }
	} else {
		old := os.Getenv("HOME")
		os.Setenv("HOME", filepath.Dir(filepath.Dir(filepath.Dir(configPath))))
		restoreEnv = func() { os.Setenv("HOME", old) }
	}
	defer restoreEnv()

	cfg, err := config.Load()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.CopilotOAuthToken != wantToken {
		t.Errorf("expected token from apps.json, got %q", cfg.CopilotOAuthToken)
	}
}

func TestFindCopilotToken_None(t *testing.T) {
	os.Unsetenv("COPILOT_OAUTH_TOKEN")
	// Patch HOME/LOCALAPPDATA to a temp dir with no apps.json
	dir := t.TempDir()
	var restoreEnv func()
	if runtime.GOOS == "windows" {
		old := os.Getenv("LOCALAPPDATA")
		os.Setenv("LOCALAPPDATA", dir)
		restoreEnv = func() { os.Setenv("LOCALAPPDATA", old) }
	} else {
		old := os.Getenv("HOME")
		os.Setenv("HOME", dir)
		restoreEnv = func() { os.Setenv("HOME", old) }
	}
	defer restoreEnv()

	cfg, err := config.Load()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.CopilotOAuthToken != "" {
		t.Errorf("expected empty token, got %q", cfg.CopilotOAuthToken)
	}
}
