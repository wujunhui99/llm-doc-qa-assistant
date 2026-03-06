package svc

import (
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"gopkg.in/yaml.v3"
)

type Config struct {
	Port                     string `yaml:"port"`
	CoreRPCAddr              string `yaml:"core_rpc_addr"`
	ReadHeaderTimeoutSeconds int    `yaml:"read_header_timeout_seconds"`
	ReadTimeoutSeconds       int    `yaml:"read_timeout_seconds"`
	WriteTimeoutSeconds      int    `yaml:"write_timeout_seconds"`
	IdleTimeoutSeconds       int    `yaml:"idle_timeout_seconds"`
}

func LoadConfig(configPath string) Config {
	cfg := defaultConfig()

	if strings.TrimSpace(configPath) != "" {
		if data, err := os.ReadFile(filepath.Clean(configPath)); err == nil && len(data) > 0 {
			_ = yaml.Unmarshal(data, &cfg)
		}
	}

	cfg.Port = getenv("PORT", cfg.Port)
	cfg.CoreRPCAddr = getenv("CORE_RPC_ADDR", cfg.CoreRPCAddr)
	cfg.ReadHeaderTimeoutSeconds = getenvInt("READ_HEADER_TIMEOUT_SECONDS", cfg.ReadHeaderTimeoutSeconds)
	cfg.ReadTimeoutSeconds = getenvInt("READ_TIMEOUT_SECONDS", cfg.ReadTimeoutSeconds)
	cfg.WriteTimeoutSeconds = getenvInt("WRITE_TIMEOUT_SECONDS", cfg.WriteTimeoutSeconds)
	cfg.IdleTimeoutSeconds = getenvInt("IDLE_TIMEOUT_SECONDS", cfg.IdleTimeoutSeconds)

	if cfg.Port == "" {
		cfg.Port = "8080"
	}
	if cfg.CoreRPCAddr == "" {
		cfg.CoreRPCAddr = "127.0.0.1:19090"
	}
	if cfg.ReadHeaderTimeoutSeconds <= 0 {
		cfg.ReadHeaderTimeoutSeconds = 5
	}
	if cfg.ReadTimeoutSeconds <= 0 {
		cfg.ReadTimeoutSeconds = 20
	}
	if cfg.WriteTimeoutSeconds <= 0 {
		cfg.WriteTimeoutSeconds = 60
	}
	if cfg.IdleTimeoutSeconds <= 0 {
		cfg.IdleTimeoutSeconds = 60
	}

	return cfg
}

func defaultConfig() Config {
	return Config{
		Port:                     "8080",
		CoreRPCAddr:              "127.0.0.1:19090",
		ReadHeaderTimeoutSeconds: 5,
		ReadTimeoutSeconds:       20,
		WriteTimeoutSeconds:      60,
		IdleTimeoutSeconds:       60,
	}
}

func getenv(key, def string) string {
	if v, ok := os.LookupEnv(key); ok && strings.TrimSpace(v) != "" {
		return strings.TrimSpace(v)
	}
	return def
}

func getenvInt(key string, def int) int {
	v, ok := os.LookupEnv(key)
	if !ok || strings.TrimSpace(v) == "" {
		return def
	}
	n, err := strconv.Atoi(strings.TrimSpace(v))
	if err != nil {
		return def
	}
	return n
}
