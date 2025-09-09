package main

import (
	"context"
	"errors"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"

	"github.com/joho/godotenv"

	"copilot-api/internal/api"
	"copilot-api/internal/copilot"
	"copilot-api/pkg/config"
	"time"
)

func main() {
	// Load environment variables from .env if present
	_ = godotenv.Load()

	// Load configuration
	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("failed to load config: %v", err)
	}
	// If COPILOT_TOKEN was randomly generated, print it for the user
	if os.Getenv("COPILOT_TOKEN") == "" {
		log.Printf("COPILOT_TOKEN was not set. Generated random token: %s", cfg.CopilotToken)
	}

	// Set up root context with cancellation
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	// Set up ModelsCache (fetch models at startup, refresh every 6 hours)
	var modelsCache *copilot.ModelsCache
	modelsCache, err = copilot.NewModelsCache(ctx, cfg.CopilotToken, 6*time.Hour)
	if err != nil {
		log.Printf("Warning: failed to fetch models list at startup: %v", err)
	}

	// Set up Copilot TokenManager (handles token refresh, concurrency, etc.)
	tokenManager, err := copilot.NewTokenManager(ctx)
	if err != nil {
		log.Fatalf("failed to initialize Copilot token manager: %v", err)
	}
	defer tokenManager.Close()

	// Set up root context with cancellation
	// Use COPILOT_SERVER_PORT for listening address if set, otherwise fallback to ServerAddr
	addr := cfg.ServerAddr
	if cfg.ServerPort != "" {
		addr = ":" + cfg.ServerPort
	}

	// Set up HTTP server, inject TokenManager and ModelsCache into API router
	server := &http.Server{
		Addr:         addr,
		Handler:      api.NewRouter(cfg, tokenManager, modelsCache),
		ReadTimeout:  10 * time.Second,
		WriteTimeout: 15 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	// Start server in a goroutine
	go func() {
		log.Printf("Starting server on %s", addr)
		if err := server.ListenAndServe(); !errors.Is(err, http.ErrServerClosed) {
			log.Fatalf("server error: %v", err)
		}
	}()

	// Wait for shutdown signal
	<-ctx.Done()
	log.Println("Shutdown signal received")

	// Graceful shutdown
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := server.Shutdown(shutdownCtx); err != nil {
		log.Printf("graceful shutdown failed: %v", err)
	} else {
		log.Println("server shut down gracefully")
	}
}
