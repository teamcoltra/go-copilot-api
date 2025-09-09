package api

import (
	"encoding/json"
	"io"
	"net/http"
	"strings"

	"copilot-api/internal/copilot"
	"copilot-api/pkg/config"
)

// NewRouter creates and returns the main HTTP handler (router) for the API.
// Accepts a TokenManager for Copilot token management and a ModelsCache for model listing.
func NewRouter(cfg *config.Config, tokenManager *copilot.TokenManager, modelsCache *copilot.ModelsCache) http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("/healthz", healthHandler)
	mux.HandleFunc("/v1/chat/completions", chatCompletionsHandler(cfg, tokenManager))
	mux.HandleFunc("/v1/embeddings", embeddingsHandler(cfg, tokenManager))
	mux.HandleFunc("/v1/messages", anthropicHandler(cfg, tokenManager))
	mux.HandleFunc("/v1/models", modelsHandler(modelsCache))

	handler := loggingMiddleware(AuthMiddleware(cfg, CORS(cfg, mux)))
	return handler
}

// healthHandler provides a simple health check endpoint.
func healthHandler(w http.ResponseWriter, r *http.Request) {
	resp := map[string]string{"status": "ok"}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(resp)
}

// loggingMiddleware is a simple request logger for demonstration.
func loggingMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// In production, use a structured logger (e.g., slog, zap, zerolog)
		// Here, we use the standard library for simplicity.
		// log.Printf("%s %s %s", r.RemoteAddr, r.Method, r.URL.Path)
		next.ServeHTTP(w, r)
	})
}

// chatCompletionsHandler handles /v1/chat/completions requests (proxy to Copilot, streaming support).
func chatCompletionsHandler(cfg *config.Config, tokenManager *copilot.TokenManager) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()
		copilotToken, err := tokenManager.GetToken(ctx)
		if err != nil {
			http.Error(w, "Failed to get Copilot token: "+err.Error(), http.StatusInternalServerError)
			return
		}

		// Read and possibly inject default model
		var reqBody map[string]interface{}
		if err := json.NewDecoder(r.Body).Decode(&reqBody); err != nil {
			http.Error(w, "Invalid JSON: "+err.Error(), http.StatusBadRequest)
			return
		}
		if reqBody["model"] == nil || reqBody["model"] == "" {
			if cfg.DefaultModel != "" {
				reqBody["model"] = cfg.DefaultModel
			} else {
				delete(reqBody, "model")
			}
		}
		bodyBytes, err := json.Marshal(reqBody)
		if err != nil {
			http.Error(w, "Failed to marshal request: "+err.Error(), http.StatusInternalServerError)
			return
		}

		// Prepare request to Copilot API
		req, err := http.NewRequestWithContext(ctx, r.Method, "https://api.githubcopilot.com/chat/completions", strings.NewReader(string(bodyBytes)))
		if err != nil {
			http.Error(w, "Failed to create request: "+err.Error(), http.StatusInternalServerError)
			return
		}
		// Copy headers except for hop-by-hop and auth
		for k, v := range r.Header {
			if strings.ToLower(k) == "authorization" || strings.ToLower(k) == "host" || strings.ToLower(k) == "connection" || strings.ToLower(k) == "content-length" {
				continue
			}
			for _, vv := range v {
				req.Header.Add(k, vv)
			}
		}
		req.Header.Set("Authorization", "Bearer "+copilotToken)
		req.Header.Set("Copilot-Integration-Id", "vscode-chat")
		req.Header.Set("Editor-Version", "Go/1.21+")
		req.Header.Set("Content-Type", "application/json")

		client := &http.Client{}
		resp, err := client.Do(req)
		if err != nil {
			http.Error(w, "Failed to contact Copilot API: "+err.Error(), http.StatusBadGateway)
			return
		}
		defer resp.Body.Close()

		// Propagate status code and headers
		for k, v := range resp.Header {
			for _, vv := range v {
				w.Header().Add(k, vv)
			}
		}
		w.WriteHeader(resp.StatusCode)

		// If streaming, copy as stream
		if strings.Contains(resp.Header.Get("Content-Type"), "text/event-stream") {
			// Stream response
			buf := make([]byte, 4096)
			for {
				n, err := resp.Body.Read(buf)
				if n > 0 {
					_, _ = w.Write(buf[:n])
					w.(http.Flusher).Flush()
				}
				if err != nil {
					break
				}
			}
			return
		}

		// Otherwise, copy the full response
		_, _ = io.Copy(w, resp.Body)
	}
}

// embeddingsHandler handles /v1/embeddings requests (proxy to Copilot).
func embeddingsHandler(cfg *config.Config, tokenManager *copilot.TokenManager) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()
		copilotToken, err := tokenManager.GetToken(ctx)
		if err != nil {
			http.Error(w, "Failed to get Copilot token: "+err.Error(), http.StatusInternalServerError)
			return
		}

		// Read and possibly inject default model
		var reqBody map[string]interface{}
		if err := json.NewDecoder(r.Body).Decode(&reqBody); err != nil {
			http.Error(w, "Invalid JSON: "+err.Error(), http.StatusBadRequest)
			return
		}
		if reqBody["model"] == nil || reqBody["model"] == "" {
			if cfg.DefaultModel != "" {
				reqBody["model"] = cfg.DefaultModel
			} else {
				delete(reqBody, "model")
			}
		}
		bodyBytes, err := json.Marshal(reqBody)
		if err != nil {
			http.Error(w, "Failed to marshal request: "+err.Error(), http.StatusInternalServerError)
			return
		}

		// Prepare request to Copilot API
		req, err := http.NewRequestWithContext(ctx, r.Method, "https://api.githubcopilot.com/embeddings", strings.NewReader(string(bodyBytes)))
		if err != nil {
			http.Error(w, "Failed to create request: "+err.Error(), http.StatusInternalServerError)
			return
		}
		// Copy headers except for hop-by-hop and auth
		for k, v := range r.Header {
			if strings.ToLower(k) == "authorization" || strings.ToLower(k) == "host" || strings.ToLower(k) == "connection" || strings.ToLower(k) == "content-length" {
				continue
			}
			for _, vv := range v {
				req.Header.Add(k, vv)
			}
		}
		req.Header.Set("Authorization", "Bearer "+copilotToken)
		req.Header.Set("Copilot-Integration-Id", "vscode-chat")
		req.Header.Set("Editor-Version", "Go/1.21+")
		req.Header.Set("Content-Type", "application/json")

		client := &http.Client{}
		resp, err := client.Do(req)
		if err != nil {
			http.Error(w, "Failed to contact Copilot API: "+err.Error(), http.StatusBadGateway)
			return
		}
		defer resp.Body.Close()

		// Propagate status code and headers
		for k, v := range resp.Header {
			for _, vv := range v {
				w.Header().Add(k, vv)
			}
		}
		w.WriteHeader(resp.StatusCode)

		// Copy the full response
		_, _ = io.Copy(w, resp.Body)
	}
}

// anthropicHandler handles /v1/messages requests (Anthropic compatibility).
func anthropicHandler(cfg *config.Config, tokenManager *copilot.TokenManager) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()
		copilotToken, err := tokenManager.GetToken(ctx)
		if err != nil {
			http.Error(w, "Failed to get Copilot token: "+err.Error(), http.StatusInternalServerError)
			return
		}

		// Decode Anthropic-style request and convert to OpenAI/Copilot format
		var anthropicReq map[string]interface{}
		if err := json.NewDecoder(r.Body).Decode(&anthropicReq); err != nil {
			http.Error(w, "Invalid JSON: "+err.Error(), http.StatusBadRequest)
			return
		}
		// Inject default model if missing
		if anthropicReq["model"] == nil || anthropicReq["model"] == "" {
			if cfg.DefaultModel != "" {
				anthropicReq["model"] = cfg.DefaultModel
			} else {
				delete(anthropicReq, "model")
			}
		}
		openaiReq := convertAnthropicToOpenAI(anthropicReq)
		bodyBytes, err := json.Marshal(openaiReq)
		if err != nil {
			http.Error(w, "Failed to marshal request: "+err.Error(), http.StatusInternalServerError)
			return
		}

		// Prepare request to Copilot API
		req, err := http.NewRequestWithContext(ctx, http.MethodPost, "https://api.githubcopilot.com/chat/completions", strings.NewReader(string(bodyBytes)))
		if err != nil {
			http.Error(w, "Failed to create request: "+err.Error(), http.StatusInternalServerError)
			return
		}
		for k, v := range r.Header {
			if strings.ToLower(k) == "authorization" || strings.ToLower(k) == "host" || strings.ToLower(k) == "connection" || strings.ToLower(k) == "content-length" {
				continue
			}
			for _, vv := range v {
				req.Header.Add(k, vv)
			}
		}
		req.Header.Set("Authorization", "Bearer "+copilotToken)
		req.Header.Set("Copilot-Integration-Id", "vscode-chat")
		req.Header.Set("Editor-Version", "Go/1.21+")
		req.Header.Set("Content-Type", "application/json")

		client := &http.Client{}
		resp, err := client.Do(req)
		if err != nil {
			http.Error(w, "Failed to contact Copilot API: "+err.Error(), http.StatusBadGateway)
			return
		}
		defer resp.Body.Close()

		// Propagate status code and headers
		for k, v := range resp.Header {
			for _, vv := range v {
				w.Header().Add(k, vv)
			}
		}
		w.WriteHeader(resp.StatusCode)

		// If streaming, convert stream to Anthropic format
		if strings.Contains(resp.Header.Get("Content-Type"), "text/event-stream") {
			convertOpenAIStreamToAnthropic(w, resp.Body)
			return
		}

		// Otherwise, convert full response to Anthropic format
		var openaiResp map[string]interface{}
		respBytes, _ := io.ReadAll(resp.Body)
		if err := json.Unmarshal(respBytes, &openaiResp); err != nil {
			http.Error(w, "Failed to decode Copilot response: "+err.Error(), http.StatusBadGateway)
			return
		}
		anthropicResp := convertOpenAIToAnthropic(openaiResp)
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(anthropicResp)
	}
}

// convertAnthropicToOpenAI converts Anthropic-style request to OpenAI/Copilot format.
func convertAnthropicToOpenAI(body map[string]interface{}) map[string]interface{} {
	// Minimal conversion: map "messages", "model", "max_tokens", "temperature", "stream"
	out := map[string]interface{}{
		"messages":    body["messages"],
		"model":       body["model"],
		"max_tokens":  body["max_tokens"],
		"temperature": body["temperature"],
		"stream":      body["stream"],
	}
	if tools, ok := body["tools"]; ok {
		out["tools"] = tools
	}
	if toolChoice, ok := body["tool_choice"]; ok {
		out["tool_choice"] = toolChoice
	}
	return out
}

// convertOpenAIToAnthropic converts OpenAI/Copilot response to Anthropic-style response.
func convertOpenAIToAnthropic(body map[string]interface{}) map[string]interface{} {
	// Minimal conversion: wrap OpenAI response in Anthropic-like structure
	return map[string]interface{}{
		"id":      body["id"],
		"type":    "message",
		"role":    "assistant",
		"model":   body["model"],
		"content": body["choices"],
		"usage":   body["usage"],
		"stop_reason": func() interface{} {
			if choices, ok := body["choices"].([]interface{}); ok && len(choices) > 0 {
				if m, ok := choices[0].(map[string]interface{}); ok {
					return m["finish_reason"]
				}
			}
			return nil
		}(),
	}
}

// convertOpenAIStreamToAnthropic converts OpenAI/Copilot streaming response to Anthropic-style SSE.
func convertOpenAIStreamToAnthropic(w http.ResponseWriter, body io.Reader) {
	// This is a minimal passthrough for now; real implementation would parse and reformat SSE events.
	buf := make([]byte, 4096)
	for {
		n, err := body.Read(buf)
		if n > 0 {
			_, _ = w.Write(buf[:n])
			if f, ok := w.(http.Flusher); ok {
				f.Flush()
			}
		}
		if err != nil {
			break
		}
	}
}

// modelsHandler serves the cached models JSON at /v1/models.
func modelsHandler(modelsCache *copilot.ModelsCache) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()
		models, err := modelsCache.GetModels(ctx)
		if err != nil {
			http.Error(w, "Failed to fetch models: "+err.Error(), http.StatusServiceUnavailable)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write(models)
	}
}

// AuthMiddleware checks for Bearer token in Authorization header.
func AuthMiddleware(cfg *config.Config, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Allow unauthenticated access to healthz and /v1/models
		if r.URL.Path == "/healthz" || r.URL.Path == "/v1/models" {
			next.ServeHTTP(w, r)
			return
		}
		auth := r.Header.Get("Authorization")
		if !strings.HasPrefix(auth, "Bearer ") {
			http.Error(w, "Unauthorized: missing or invalid Authorization header", http.StatusUnauthorized)
			return
		}
		token := strings.TrimPrefix(auth, "Bearer ")
		if token == "" || token != cfg.CopilotToken {
			http.Error(w, "Forbidden: invalid access token", http.StatusForbidden)
			return
		}
		next.ServeHTTP(w, r)
	})
}

// CORS middleware adds CORS headers based on config.
func CORS(cfg *config.Config, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		origins := strings.Split(cfg.CORSAllowedOrigins, ",")
		origin := r.Header.Get("Origin")
		allowed := false
		for _, o := range origins {
			if o == "*" || strings.TrimSpace(o) == origin {
				allowed = true
				break
			}
		}
		if allowed {
			w.Header().Set("Access-Control-Allow-Origin", origin)
		} else if cfg.CORSAllowedOrigins == "*" {
			w.Header().Set("Access-Control-Allow-Origin", "*")
		}
		w.Header().Set("Access-Control-Allow-Methods", "GET,POST,OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Authorization,Content-Type")
		w.Header().Set("Access-Control-Allow-Credentials", "true")
		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		next.ServeHTTP(w, r)
	})
}
