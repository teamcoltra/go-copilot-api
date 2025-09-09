package test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"context"
	"copilot-api/internal/api"
	"copilot-api/internal/copilot"
	"copilot-api/pkg/config"
	"time"
)

func TestHealthzEndpoint(t *testing.T) {
	cfg := &config.Config{}
	// Provide a dummy TokenManager and dummy ModelsCache for testing
	dummyTokenManager, _ := copilot.NewTokenManager(context.Background())
	dummyModelsCache, _ := copilot.NewModelsCache(context.Background(), "dummy", 1*time.Hour)
	handler := api.NewRouter(cfg, dummyTokenManager, dummyModelsCache)

	tests := []struct {
		name           string
		method         string
		target         string
		wantStatusCode int
		wantBody       map[string]string
	}{
		{
			name:           "GET healthz",
			method:         http.MethodGet,
			target:         "/healthz",
			wantStatusCode: http.StatusOK,
			wantBody:       map[string]string{"status": "ok"},
		},
		{
			name:           "POST healthz (should still work)",
			method:         http.MethodPost,
			target:         "/healthz",
			wantStatusCode: http.StatusOK,
			wantBody:       map[string]string{"status": "ok"},
		},
		{
			name:           "GET unknown route",
			method:         http.MethodGet,
			target:         "/notfound",
			wantStatusCode: http.StatusUnauthorized, // Middleware returns 401 for missing auth on unknown routes
			wantBody:       nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(tt.method, tt.target, nil)
			rr := httptest.NewRecorder()
			handler.ServeHTTP(rr, req)

			if rr.Code != tt.wantStatusCode {
				t.Errorf("expected status %d, got %d", tt.wantStatusCode, rr.Code)
			}

			if tt.wantBody != nil {
				var got map[string]string
				if err := json.Unmarshal(rr.Body.Bytes(), &got); err != nil {
					t.Fatalf("failed to unmarshal response: %v", err)
				}
				for k, v := range tt.wantBody {
					if got[k] != v {
						t.Errorf("expected body[%q]=%q, got %q", k, v, got[k])
					}
				}
			}
		})
	}
}
