package main

import (
	"context"
	"fmt"
	"log"
	"net"
	"os"
	"path/filepath"
	"strings"
	"time"

	authapp "llm-doc-qa-assistant/backend/apps/core-go-rpc/internal/application/auth"
	"llm-doc-qa-assistant/backend/apps/core-go-rpc/internal/infrastructure/minio"
	"llm-doc-qa-assistant/backend/apps/core-go-rpc/internal/infrastructure/mysql"
	"llm-doc-qa-assistant/backend/apps/core-go-rpc/internal/infrastructure/qdrant"
	"llm-doc-qa-assistant/backend/apps/core-go-rpc/internal/infrastructure/security"
	"llm-doc-qa-assistant/backend/apps/core-go-rpc/internal/llmrpc"
	"llm-doc-qa-assistant/backend/apps/core-go-rpc/internal/rpc"
	"llm-doc-qa-assistant/backend/apps/core-go-rpc/internal/store"
	qav1 "llm-doc-qa-assistant/backend/proto/gen/go/qa/v1"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

type config struct {
	CoreRPCAddr          string
	LLMRPCAddr           string
	LLMRPCDialTimeoutSec int
	DataDir              string
	MySQLDSN             string
	MinIOEndpoint        string
	MinIOAccessKey       string
	MinIOSecretKey       string
	MinIOBucket          string
	MinIOUseSSL          bool
	VectorSearchEnabled  bool
	QdrantEndpoint       string
	QdrantAPIKey         string
	QdrantCollection     string
	QdrantTimeoutSeconds int
}

func main() {
	logger := log.New(log.Writer(), "[core-go-rpc] ", log.LstdFlags|log.LUTC)
	loadDotEnv("./apps/core-go-rpc/.env", logger)
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

	llmRPCAddr := cfg.LLMRPCAddr
	llmConn, err := dialLLMRPC(llmRPCAddr, time.Duration(cfg.LLMRPCDialTimeoutSec)*time.Second)
	if err != nil && !strings.HasPrefix(llmRPCAddr, "unix://") {
		fallbackAddr := "unix:///tmp/llm-python-rpc.sock"
		if _, statErr := os.Stat(strings.TrimPrefix(fallbackAddr, "unix://")); statErr == nil {
			llmConn, err = dialLLMRPC(fallbackAddr, time.Duration(cfg.LLMRPCDialTimeoutSec)*time.Second)
			if err == nil {
				logger.Printf("llm rpc dial fallback: %s -> %s", llmRPCAddr, fallbackAddr)
				llmRPCAddr = fallbackAddr
			}
		}
	}
	if err != nil {
		logger.Fatalf("connect llm rpc failed (%s): %v", cfg.LLMRPCAddr, err)
	}
	defer llmConn.Close()

	llmClient := llmrpc.New(qav1.NewLlmServiceClient(llmConn))
	coreServer := rpc.NewServer(s, authService, objectStore, logger).WithLLMService(llmClient)
	if cfg.VectorSearchEnabled {
		vectorClient := qdrant.NewClient(
			cfg.QdrantEndpoint,
			cfg.QdrantAPIKey,
			cfg.QdrantCollection,
			time.Duration(cfg.QdrantTimeoutSeconds)*time.Second,
		)
		if !vectorClient.Enabled() {
			logger.Printf("vector search disabled: QDRANT_ENDPOINT or QDRANT_COLLECTION is empty")
		} else {
			coreServer = coreServer.WithVectorSearch(vectorClient)
			logger.Printf("vector search enabled: collection=%s endpoint=%s llm_rpc=%s", cfg.QdrantCollection, cfg.QdrantEndpoint, llmRPCAddr)
		}
	}

	server := grpc.NewServer()
	qav1.RegisterCoreServiceServer(server, coreServer)

	listener, err := net.Listen("tcp", cfg.CoreRPCAddr)
	if err != nil {
		logger.Fatalf("listen failed: %v", err)
	}

	logger.Printf("core grpc listening on %s", cfg.CoreRPCAddr)
	if err := server.Serve(listener); err != nil {
		logger.Fatalf("server failed: %v", err)
	}
}

func loadDotEnv(path string, logger *log.Logger) {
	data, err := os.ReadFile(path)
	if err != nil {
		if !os.IsNotExist(err) {
			logger.Printf("load .env failed (%s): %v", path, err)
		}
		return
	}

	lines := strings.Split(string(data), "\n")
	for i, line := range lines {
		raw := strings.TrimSpace(line)
		if raw == "" || strings.HasPrefix(raw, "#") {
			continue
		}
		if strings.HasPrefix(raw, "export ") {
			raw = strings.TrimSpace(strings.TrimPrefix(raw, "export "))
		}

		idx := strings.Index(raw, "=")
		if idx <= 0 {
			logger.Printf("ignore invalid .env line %d in %s", i+1, path)
			continue
		}

		key := strings.TrimSpace(raw[:idx])
		val := strings.TrimSpace(raw[idx+1:])
		if key == "" {
			continue
		}
		if len(val) >= 2 {
			if (strings.HasPrefix(val, "\"") && strings.HasSuffix(val, "\"")) ||
				(strings.HasPrefix(val, "'") && strings.HasSuffix(val, "'")) {
				val = val[1 : len(val)-1]
			}
		}

		if _, exists := os.LookupEnv(key); exists {
			continue
		}
		if err := os.Setenv(key, val); err != nil {
			logger.Printf("set env from .env failed for %s: %v", key, err)
		}
	}
}

func loadConfig() config {
	return config{
		CoreRPCAddr:          getenv("CORE_RPC_ADDR", ":19090"),
		LLMRPCAddr:           getenv("LLM_RPC_ADDR", "127.0.0.1:51000"),
		LLMRPCDialTimeoutSec: parseInt(getenv("LLM_RPC_DIAL_TIMEOUT_SECONDS", "8"), 8),
		DataDir:              getenv("DATA_DIR", "./data"),
		MySQLDSN:             getenv("MYSQL_DSN", "app:app123456@tcp(127.0.0.1:3306)/llm_doc_qa?parseTime=true&charset=utf8mb4&loc=Local"),
		MinIOEndpoint:        getenv("MINIO_ENDPOINT", "127.0.0.1:9000"),
		MinIOAccessKey:       getenv("MINIO_ACCESS_KEY", "minioadmin"),
		MinIOSecretKey:       getenv("MINIO_SECRET_KEY", "minioadmin123"),
		MinIOBucket:          getenv("MINIO_BUCKET", "qa-documents"),
		MinIOUseSSL:          parseBool(getenv("MINIO_USE_SSL", "false")),
		VectorSearchEnabled:  parseBool(getenv("VECTOR_SEARCH_ENABLED", "false")),
		QdrantEndpoint:       getenv("QDRANT_ENDPOINT", "http://127.0.0.1:6333"),
		QdrantAPIKey:         getenv("QDRANT_API_KEY", ""),
		QdrantCollection:     getenv("QDRANT_COLLECTION", "qa_chunks"),
		QdrantTimeoutSeconds: parseInt(getenv("QDRANT_TIMEOUT_SECONDS", "10"), 10),
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

func parseInt(raw string, def int) int {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return def
	}
	var out int
	_, err := fmt.Sscanf(raw, "%d", &out)
	if err != nil || out <= 0 {
		return def
	}
	return out
}

func dialLLMRPC(addr string, timeout time.Duration) (*grpc.ClientConn, error) {
	dialCtx, dialCancel := context.WithTimeout(context.Background(), timeout)
	defer dialCancel()
	return grpc.DialContext(
		dialCtx,
		addr,
		grpc.WithTransportCredentials(insecure.NewCredentials()),
		grpc.WithBlock(),
	)
}
