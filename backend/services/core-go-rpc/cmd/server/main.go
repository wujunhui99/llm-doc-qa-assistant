package main

import (
	"context"
	"log"
	"net"
	"os"
	"path/filepath"
	"strings"

	authapp "llm-doc-qa-assistant/backend/internal/application/auth"
	"llm-doc-qa-assistant/backend/internal/infrastructure/minio"
	"llm-doc-qa-assistant/backend/internal/infrastructure/mysql"
	"llm-doc-qa-assistant/backend/internal/infrastructure/security"
	"llm-doc-qa-assistant/backend/internal/store"
	qav1 "llm-doc-qa-assistant/backend/proto/gen/go/qa/v1"
	"llm-doc-qa-assistant/backend/services/core-go-rpc/internal/rpc"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

type config struct {
	CoreRPCAddr          string
	LLMRPCAddr           string
	DataDir              string
	MySQLDSN             string
	MinIOEndpoint        string
	MinIOAccessKey       string
	MinIOSecretKey       string
	MinIOBucket          string
	MinIOUseSSL          bool
	InternalServiceToken string
}

func main() {
	logger := log.New(log.Writer(), "[core-go-rpc] ", log.LstdFlags|log.LUTC)
	cfg := loadConfig()

	statePath := filepath.Join(cfg.DataDir, "state.json")
	auditPath := filepath.Join(cfg.DataDir, "audit.log")
	s, err := store.New(statePath, auditPath)
	if err != nil {
		logger.Fatalf("init store failed: %v", err)
	}

	db, err := mysql.Open(cfg.MySQLDSN)
	if err != nil {
		logger.Fatalf("init mysql failed: %v", err)
	}
	defer db.Close()

	objectStore, err := minio.New(context.Background(), cfg.MinIOEndpoint, cfg.MinIOAccessKey, cfg.MinIOSecretKey, cfg.MinIOUseSSL, cfg.MinIOBucket)
	if err != nil {
		logger.Fatalf("init minio failed: %v", err)
	}

	userRepo := mysql.NewUserRepository(db)
	sessionRepo := mysql.NewSessionRepository(db)
	authService := authapp.NewService(userRepo, sessionRepo, security.PasswordHasher{}, security.TokenGenerator{}, s)

	llmConn, err := grpc.NewClient(cfg.LLMRPCAddr, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		logger.Fatalf("dial llm rpc failed: %v", err)
	}
	defer llmConn.Close()
	llmClient := qav1.NewLlmServiceClient(llmConn)

	server := grpc.NewServer()
	qav1.RegisterCoreServiceServer(server, rpc.NewServer(s, authService, objectStore, llmClient, cfg.InternalServiceToken, logger))

	listener, err := net.Listen("tcp", cfg.CoreRPCAddr)
	if err != nil {
		logger.Fatalf("listen failed: %v", err)
	}

	logger.Printf("core grpc listening on %s", cfg.CoreRPCAddr)
	if err := server.Serve(listener); err != nil {
		logger.Fatalf("server failed: %v", err)
	}
}

func loadConfig() config {
	return config{
		CoreRPCAddr:          getenv("CORE_RPC_ADDR", ":19090"),
		LLMRPCAddr:           getenv("LLM_RPC_ADDR", "127.0.0.1:19091"),
		DataDir:              getenv("DATA_DIR", "./data"),
		MySQLDSN:             getenv("MYSQL_DSN", "app:app123456@tcp(127.0.0.1:3306)/llm_doc_qa?parseTime=true&charset=utf8mb4&loc=Local"),
		MinIOEndpoint:        getenv("MINIO_ENDPOINT", "127.0.0.1:9000"),
		MinIOAccessKey:       getenv("MINIO_ACCESS_KEY", "minioadmin"),
		MinIOSecretKey:       getenv("MINIO_SECRET_KEY", "minioadmin123"),
		MinIOBucket:          getenv("MINIO_BUCKET", "qa-documents"),
		MinIOUseSSL:          parseBool(getenv("MINIO_USE_SSL", "false")),
		InternalServiceToken: strings.TrimSpace(getenv("INTERNAL_SERVICE_TOKEN", "")),
	}
}

func getenv(key, def string) string {
	if v, ok := os.LookupEnv(key); ok && strings.TrimSpace(v) != "" {
		return v
	}
	return def
}

func parseBool(raw string) bool {
	raw = strings.TrimSpace(strings.ToLower(raw))
	if raw == "1" || raw == "true" || raw == "yes" {
		return true
	}
	return false
}
