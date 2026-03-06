package qa

import (
	"testing"
	"time"

	"llm-doc-qa-assistant/backend/internal/types"
)

func TestResolveScopeDefaultAll(t *testing.T) {
	docs := []types.Document{{ID: "doc_1", Name: "a.md", CreatedAt: time.Now()}}
	scope, err := ResolveScope("请总结需求", "", nil, docs)
	if err != nil {
		t.Fatalf("ResolveScope error: %v", err)
	}
	if scope.Type != "all" {
		t.Fatalf("expected all scope, got %s", scope.Type)
	}
}

func TestResolveScopeDocByName(t *testing.T) {
	docs := []types.Document{{ID: "doc_1", Name: "prd-v2.md", CreatedAt: time.Now()}}
	scope, err := ResolveScope("@doc(prd-v2.md) 上线范围是什么", "", nil, docs)
	if err != nil {
		t.Fatalf("ResolveScope error: %v", err)
	}
	if scope.Type != "doc" || len(scope.DocIDs) != 1 || scope.DocIDs[0] != "doc_1" {
		t.Fatalf("unexpected scope result: %+v", scope)
	}
}

func TestResolveScopeRejectsUnknownDoc(t *testing.T) {
	_, err := ResolveScope("@doc(unknown.md) 这是什么", "", nil, nil)
	if err == nil {
		t.Fatalf("expected unknown doc error")
	}
}
