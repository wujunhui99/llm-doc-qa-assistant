package store

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"llm-doc-qa-assistant/backend/apps/core-go-rpc/internal/types"
)

func TestStoreDefaultProviderIncludesSiliconFlow(t *testing.T) {
	tmp := t.TempDir()
	s, err := New(filepath.Join(tmp, "state.json"), filepath.Join(tmp, "audit.log"))
	if err != nil {
		t.Fatalf("init store failed: %v", err)
	}

	provider := s.GetProvider()
	if provider.ActiveProvider != "siliconflow" {
		t.Fatalf("expected default active provider siliconflow, got %q", provider.ActiveProvider)
	}
	if !containsProvider(provider.Available, "siliconflow") {
		t.Fatalf("expected available providers to include siliconflow, got %+v", provider.Available)
	}
}

func TestStoreMergesSiliconFlowIntoLegacyProviders(t *testing.T) {
	tmp := t.TempDir()
	statePath := filepath.Join(tmp, "state.json")
	auditPath := filepath.Join(tmp, "audit.log")

	legacy := types.State{
		Documents: map[string]types.Document{},
		Chunks:    map[string][]types.Chunk{},
		Threads:   map[string]types.Thread{},
		Turns:     map[string]types.Turn{},
		TurnItems: map[string][]types.TurnItem{},
		Provider: types.ProviderConfig{
			ActiveProvider: "mock",
			Available:      []string{"mock", "openai", "claude", "local"},
		},
		Initialized: true,
	}
	raw, err := json.Marshal(legacy)
	if err != nil {
		t.Fatalf("marshal legacy state failed: %v", err)
	}
	if err := os.WriteFile(statePath, raw, 0o644); err != nil {
		t.Fatalf("write legacy state failed: %v", err)
	}

	s, err := New(statePath, auditPath)
	if err != nil {
		t.Fatalf("init store failed: %v", err)
	}
	provider := s.GetProvider()
	if !containsProvider(provider.Available, "siliconflow") {
		t.Fatalf("expected migrated providers to include siliconflow, got %+v", provider.Available)
	}
}

func TestSetProviderAcceptsCaseInsensitiveSiliconFlow(t *testing.T) {
	tmp := t.TempDir()
	s, err := New(filepath.Join(tmp, "state.json"), filepath.Join(tmp, "audit.log"))
	if err != nil {
		t.Fatalf("init store failed: %v", err)
	}

	if err := s.SetProvider("SiliconFlow", "usr_1"); err != nil {
		t.Fatalf("set provider failed: %v", err)
	}
	provider := s.GetProvider()
	if provider.ActiveProvider != "siliconflow" {
		t.Fatalf("expected active provider siliconflow, got %q", provider.ActiveProvider)
	}
}
