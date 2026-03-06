package main

import (
	"context"
	"log"
	"net/http"
	"path/filepath"
	"time"

	"llm-doc-qa-assistant/backend/internal/api"
	authapp "llm-doc-qa-assistant/backend/internal/application/auth"
	"llm-doc-qa-assistant/backend/internal/config"
	minioinfra "llm-doc-qa-assistant/backend/internal/infrastructure/minio"
	"llm-doc-qa-assistant/backend/internal/infrastructure/mysql"
	"llm-doc-qa-assistant/backend/internal/infrastructure/security"
	"llm-doc-qa-assistant/backend/internal/store"
)

func main() {
	logger := log.New(log.Writer(), "[backend] ", log.LstdFlags|log.LUTC)
	cfg := config.Load()

	statePath := filepath.Join(cfg.DataDir, "state.json")
	auditPath := filepath.Join(cfg.DataDir, "audit.log")

	s, err := store.New(statePath, auditPath)
	if err != nil {
		logger.Fatalf("failed to initialize store: %v", err)
	}

	db, err := mysql.Open(cfg.MySQLDSN)
	if err != nil {
		logger.Fatalf("failed to initialize mysql: %v", err)
	}
	defer db.Close()

	userRepo := mysql.NewUserRepository(db)
	sessionRepo := mysql.NewSessionRepository(db)
	authService := authapp.NewService(userRepo, sessionRepo, security.PasswordHasher{}, security.TokenGenerator{}, s)

	objectStore, err := minioinfra.New(
		context.Background(),
		cfg.MinIOEndpoint,
		cfg.MinIOAccessKey,
		cfg.MinIOSecretKey,
		cfg.MinIOUseSSL,
		cfg.MinIOBucket,
	)
	if err != nil {
		logger.Fatalf("failed to initialize minio object store: %v", err)
	}

	server := api.NewServer(s, authService, objectStore, logger)
	httpServer := &http.Server{
		Addr:              ":" + cfg.Port,
		Handler:           server.Routes(),
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       15 * time.Second,
		WriteTimeout:      30 * time.Second,
		IdleTimeout:       60 * time.Second,
	}

	logger.Printf("listening on :%s", cfg.Port)
	if err := httpServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		logger.Fatalf("server terminated: %v", err)
	}
}
