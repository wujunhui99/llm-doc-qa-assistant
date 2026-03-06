package main

import (
	"io"
	"log"
	"os"
	"path/filepath"
	"testing"
)

func TestLoadDotEnvSetsMissingVars(t *testing.T) {
	t.Setenv("DOTENV_EXISTING", "")

	tmp := t.TempDir()
	envPath := filepath.Join(tmp, ".env")
	content := "SILICONFLOW_API_KEY=test_key\nSILICONFLOW_CHAT_MODEL=Pro/MiniMaxAI/MiniMax-M2.5\n"
	if err := os.WriteFile(envPath, []byte(content), 0o644); err != nil {
		t.Fatalf("write env file failed: %v", err)
	}

	loadDotEnv(envPath, log.New(io.Discard, "", 0))

	if got := os.Getenv("SILICONFLOW_API_KEY"); got != "test_key" {
		t.Fatalf("expected SILICONFLOW_API_KEY to be loaded, got %q", got)
	}
	if got := os.Getenv("SILICONFLOW_CHAT_MODEL"); got != "Pro/MiniMaxAI/MiniMax-M2.5" {
		t.Fatalf("expected SILICONFLOW_CHAT_MODEL to be loaded, got %q", got)
	}
}

func TestLoadDotEnvDoesNotOverrideExistingEnv(t *testing.T) {
	t.Setenv("SILICONFLOW_API_KEY", "from_env")

	tmp := t.TempDir()
	envPath := filepath.Join(tmp, ".env")
	content := "SILICONFLOW_API_KEY=from_file\n"
	if err := os.WriteFile(envPath, []byte(content), 0o644); err != nil {
		t.Fatalf("write env file failed: %v", err)
	}

	loadDotEnv(envPath, log.New(io.Discard, "", 0))

	if got := os.Getenv("SILICONFLOW_API_KEY"); got != "from_env" {
		t.Fatalf("expected env var to keep original value, got %q", got)
	}
}
