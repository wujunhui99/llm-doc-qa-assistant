package main

import (
	"log"
	"net/http"
	"os"
	"time"

	"llm-doc-qa-assistant/backend/services/api-go/internal/handler"
	"llm-doc-qa-assistant/backend/services/api-go/internal/svc"
	"llm-doc-qa-assistant/backend/services/api-go/rpc/coreclient"
)

func main() {
	logger := log.New(log.Writer(), "[api-go] ", log.LstdFlags|log.LUTC)
	cfgPath := os.Getenv("API_CONFIG_FILE")
	if cfgPath == "" {
		cfgPath = "./services/api-go/config/api.yaml"
	}
	cfg := svc.LoadConfig(cfgPath)

	coreClient, err := coreclient.New(cfg.CoreRPCAddr)
	if err != nil {
		logger.Fatalf("dial core rpc failed: %v", err)
	}
	defer coreClient.Close()

	svcCtx := svc.NewServiceContext(cfg, coreClient.Core, logger)
	server := handler.NewServerWithContext(svcCtx)

	httpServer := &http.Server{
		Addr:              ":" + cfg.Port,
		Handler:           server.Routes(),
		ReadHeaderTimeout: time.Duration(cfg.ReadHeaderTimeoutSeconds) * time.Second,
		ReadTimeout:       time.Duration(cfg.ReadTimeoutSeconds) * time.Second,
		WriteTimeout:      time.Duration(cfg.WriteTimeoutSeconds) * time.Second,
		IdleTimeout:       time.Duration(cfg.IdleTimeoutSeconds) * time.Second,
	}

	logger.Printf("api gateway listening on :%s (core rpc: %s)", cfg.Port, cfg.CoreRPCAddr)
	if err := httpServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		logger.Fatalf("api server terminated: %v", err)
	}
}
