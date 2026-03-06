package main

import (
	"log"
	"net/http"
	"os"
	"strings"
	"time"

	"llm-doc-qa-assistant/backend/services/api-go/internal/grpcclient"
	"llm-doc-qa-assistant/backend/services/api-go/internal/httpapi"
)

type config struct {
	Port        string
	CoreRPCAddr string
}

func main() {
	logger := log.New(log.Writer(), "[api-go] ", log.LstdFlags|log.LUTC)
	cfg := loadConfig()

	coreClient, err := grpcclient.New(cfg.CoreRPCAddr)
	if err != nil {
		logger.Fatalf("dial core rpc failed: %v", err)
	}
	defer coreClient.Close()

	server := httpapi.NewServer(coreClient.Core, logger)
	httpServer := &http.Server{
		Addr:              ":" + cfg.Port,
		Handler:           server.Routes(),
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       20 * time.Second,
		WriteTimeout:      60 * time.Second,
		IdleTimeout:       60 * time.Second,
	}

	logger.Printf("api gateway listening on :%s (core rpc: %s)", cfg.Port, cfg.CoreRPCAddr)
	if err := httpServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		logger.Fatalf("api server terminated: %v", err)
	}
}

func loadConfig() config {
	return config{
		Port:        getenv("PORT", "8080"),
		CoreRPCAddr: getenv("CORE_RPC_ADDR", "127.0.0.1:19090"),
	}
}

func getenv(key, def string) string {
	if v, ok := os.LookupEnv(key); ok && strings.TrimSpace(v) != "" {
		return v
	}
	return def
}
