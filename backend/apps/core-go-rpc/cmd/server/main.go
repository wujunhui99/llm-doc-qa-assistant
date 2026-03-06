package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net"
	"os"
	"path/filepath"
	"strings"
	"time"

	qav1 "llm-doc-qa-assistant/backend/proto/gen/go/qa/v1"
	authapp "llm-doc-qa-assistant/backend/apps/core-go-rpc/internal/application/auth"
	"llm-doc-qa-assistant/backend/apps/core-go-rpc/internal/infrastructure/embedding"
	"llm-doc-qa-assistant/backend/apps/core-go-rpc/internal/infrastructure/minio"
	"llm-doc-qa-assistant/backend/apps/core-go-rpc/internal/infrastructure/mysql"
	"llm-doc-qa-assistant/backend/apps/core-go-rpc/internal/infrastructure/qdrant"
	"llm-doc-qa-assistant/backend/apps/core-go-rpc/internal/infrastructure/security"
	"llm-doc-qa-assistant/backend/apps/core-go-rpc/internal/llm"
	"llm-doc-qa-assistant/backend/apps/core-go-rpc/internal/rpc"
	"llm-doc-qa-assistant/backend/apps/core-go-rpc/internal/store"

	"google.golang.org/grpc"
)

type config struct {
	CoreRPCAddr          string
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
	SiliconFlowAPIBase   string
	SiliconFlowAPIKey    string
	SiliconFlowChatModel string
	SiliconFlowTemp      float64
	DefaultProvider      string
	ProviderModelJSON    string
	AgentMaxContexts     int
	EmbeddingModel       string
	EmbeddingTimeoutSec  int
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

	embedder := embedding.NewSiliconFlowEmbedder(
		cfg.SiliconFlowAPIBase,
		cfg.SiliconFlowAPIKey,
		cfg.EmbeddingModel,
		time.Duration(cfg.EmbeddingTimeoutSec)*time.Second,
	)
	chatClient := llm.NewSiliconFlowChatClient(
		cfg.SiliconFlowAPIBase,
		cfg.SiliconFlowAPIKey,
		time.Duration(cfg.EmbeddingTimeoutSec)*time.Second,
	)
	agent := llm.NewAgent(chatClient, embedder, llm.Config{
		DefaultProvider:   cfg.DefaultProvider,
		ChatModel:         cfg.SiliconFlowChatModel,
		Temperature:       cfg.SiliconFlowTemp,
		MaxContextChunks:  cfg.AgentMaxContexts,
		RequestTimeout:    time.Duration(cfg.EmbeddingTimeoutSec) * time.Second,
		ProviderChatModel: parseProviderModelMapping(cfg.ProviderModelJSON, cfg.SiliconFlowChatModel),
	})

	coreServer := rpc.NewServer(s, authService, objectStore, logger).WithAnswerGenerator(agent)
	if cfg.VectorSearchEnabled {
		vectorClient := qdrant.NewClient(
			cfg.QdrantEndpoint,
			cfg.QdrantAPIKey,
			cfg.QdrantCollection,
			time.Duration(cfg.QdrantTimeoutSeconds)*time.Second,
		)
		if !embedder.Enabled() {
			logger.Printf("vector search disabled: SILICONFLOW_API_KEY is empty")
		} else if !vectorClient.Enabled() {
			logger.Printf("vector search disabled: QDRANT_ENDPOINT or QDRANT_COLLECTION is empty")
		} else {
			coreServer = coreServer.WithVectorSearch(embedder, vectorClient)
			logger.Printf("vector search enabled: collection=%s endpoint=%s model=%s", cfg.QdrantCollection, cfg.QdrantEndpoint, cfg.EmbeddingModel)
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

func loadConfig() config {
	return config{
		CoreRPCAddr:          getenv("CORE_RPC_ADDR", ":19090"),
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
		SiliconFlowAPIBase:   getenv("SILICONFLOW_API_BASE", "https://api.siliconflow.cn/v1"),
		SiliconFlowAPIKey:    getenv("SILICONFLOW_API_KEY", ""),
		SiliconFlowChatModel: getenv("SILICONFLOW_CHAT_MODEL", "Pro/MiniMaxAI/MiniMax-M2.5"),
		SiliconFlowTemp:      parseFloat(getenv("SILICONFLOW_TEMPERATURE", "0.2"), 0.2),
		DefaultProvider:      getenv("LLM_PROVIDER", "siliconflow"),
		ProviderModelJSON:    getenv("SILICONFLOW_PROVIDER_CHAT_MODELS_JSON", ""),
		AgentMaxContexts:     parseInt(getenv("LLM_AGENT_MAX_CONTEXT_CHUNKS", "6"), 6),
		EmbeddingModel:       getenv("SILICONFLOW_EMBEDDING_MODEL", "Qwen/Qwen3-Embedding-4B"),
		EmbeddingTimeoutSec:  parseInt(getenv("SILICONFLOW_TIMEOUT_SECONDS", "30"), 30),
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

func parseFloat(raw string, def float64) float64 {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return def
	}
	var out float64
	_, err := fmt.Sscanf(raw, "%f", &out)
	if err != nil {
		return def
	}
	return out
}

func parseProviderModelMapping(rawJSON string, defaultModel string) map[string]string {
	out := map[string]string{
		"siliconflow": defaultModel,
		"mock":        defaultModel,
		"openai":      defaultModel,
		"claude":      defaultModel,
		"local":       defaultModel,
	}
	rawJSON = strings.TrimSpace(rawJSON)
	if rawJSON == "" {
		return out
	}
	var parsed map[string]string
	if err := json.Unmarshal([]byte(rawJSON), &parsed); err != nil {
		return out
	}
	for k, v := range parsed {
		key := strings.ToLower(strings.TrimSpace(k))
		value := strings.TrimSpace(v)
		if key != "" && value != "" {
			out[key] = value
		}
	}
	return out
}
