package config

import (
	"os"
	"strconv"
	"strings"
)

const (
	defaultPort      = "8080"
	defaultDataDir   = "./data"
	defaultMySQLDSN  = "app:app123456@tcp(127.0.0.1:3306)/llm_doc_qa?parseTime=true&charset=utf8mb4&loc=Local"
	defaultMinIOHost = "127.0.0.1:9000"
	defaultMinIOUser = "minioadmin"
	defaultMinIOPass = "minioadmin123"
	defaultMinIOBuck = "qa-documents"
)

type Config struct {
	Port string

	DataDir string

	MySQLDSN string

	MinIOEndpoint  string
	MinIOAccessKey string
	MinIOSecretKey string
	MinIOBucket    string
	MinIOUseSSL    bool
}

func Load() Config {
	return Config{
		Port:           getenv("PORT", defaultPort),
		DataDir:        getenv("DATA_DIR", defaultDataDir),
		MySQLDSN:       getenv("MYSQL_DSN", defaultMySQLDSN),
		MinIOEndpoint:  getenv("MINIO_ENDPOINT", defaultMinIOHost),
		MinIOAccessKey: getenv("MINIO_ACCESS_KEY", defaultMinIOUser),
		MinIOSecretKey: getenv("MINIO_SECRET_KEY", defaultMinIOPass),
		MinIOBucket:    getenv("MINIO_BUCKET", defaultMinIOBuck),
		MinIOUseSSL:    parseBool(getenv("MINIO_USE_SSL", "false")),
	}
}

func getenv(key, def string) string {
	if value, ok := os.LookupEnv(key); ok && strings.TrimSpace(value) != "" {
		return value
	}
	return def
}

func parseBool(raw string) bool {
	if raw == "" {
		return false
	}
	v, err := strconv.ParseBool(strings.ToLower(strings.TrimSpace(raw)))
	if err != nil {
		return false
	}
	return v
}
