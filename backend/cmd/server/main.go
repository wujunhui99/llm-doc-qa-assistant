package main

import (
	"log"
	"net/http"
	"os"
	"path/filepath"
	"time"

	"llm-doc-qa-assistant/backend/internal/api"
	"llm-doc-qa-assistant/backend/internal/store"
)

func main() {
	logger := log.New(os.Stdout, "[backend] ", log.LstdFlags|log.LUTC)

	port := getenv("PORT", "8080")
	dataDir := getenv("DATA_DIR", "./data")
	statePath := filepath.Join(dataDir, "state.json")
	auditPath := filepath.Join(dataDir, "audit.log")
	fileRoot := filepath.Join(dataDir, "files")

	s, err := store.New(statePath, auditPath)
	if err != nil {
		logger.Fatalf("failed to initialize store: %v", err)
	}

	server := api.NewServer(s, fileRoot, logger)
	httpServer := &http.Server{
		Addr:              ":" + port,
		Handler:           server.Routes(),
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       15 * time.Second,
		WriteTimeout:      30 * time.Second,
		IdleTimeout:       60 * time.Second,
	}

	logger.Printf("listening on :%s", port)
	if err := httpServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		logger.Fatalf("server terminated: %v", err)
	}
}

func getenv(key, def string) string {
	if value, ok := os.LookupEnv(key); ok && value != "" {
		return value
	}
	return def
}
