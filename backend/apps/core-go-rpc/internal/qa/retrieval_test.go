package qa

import (
	"strings"
	"testing"
	"time"

	"llm-doc-qa-assistant/backend/apps/core-go-rpc/internal/types"
)

func TestRetrieveTopChunksPrefersKeywordMatches(t *testing.T) {
	doc := types.Document{ID: "doc_1", Name: "prd.md", CreatedAt: time.Now()}
	chunks := map[string][]types.Chunk{
		"doc_1": {
			{ID: "c1", DocID: "doc_1", Index: 0, Content: "系统支持登录、注册和会话管理"},
			{ID: "c2", DocID: "doc_1", Index: 1, Content: "上传模块支持 PDF TXT Markdown"},
		},
	}

	res := RetrieveTopChunks("如何注册登录", []types.Document{doc}, chunks, 2)
	if len(res) == 0 {
		t.Fatalf("expected retrieval results")
	}
	if res[0].Chunk.ID != "c1" {
		t.Fatalf("expected chunk c1 ranked first, got %s", res[0].Chunk.ID)
	}
}

func TestRetrieveTopChunksSortsByScoreThenDocIDThenChunkIndex(t *testing.T) {
	longTail := strings.Repeat("x", 140)
	docs := []types.Document{
		{ID: "doc_b", Name: "b.md", CreatedAt: time.Now()},
		{ID: "doc_a", Name: "a.md", CreatedAt: time.Now()},
	}
	chunks := map[string][]types.Chunk{
		"doc_a": {
			{ID: "a2", DocID: "doc_a", Index: 2, Content: "register process " + longTail},
			{ID: "a1", DocID: "doc_a", Index: 1, Content: "register entry " + longTail},
		},
		"doc_b": {
			{ID: "b0", DocID: "doc_b", Index: 0, Content: "register usage " + longTail},
		},
	}

	res := RetrieveTopChunks("register", docs, chunks, 3)
	if len(res) != 3 {
		t.Fatalf("expected 3 results, got %d", len(res))
	}

	// Same score expected; tie-breaker should be doc ID then chunk index.
	gotOrder := []string{res[0].Chunk.ID, res[1].Chunk.ID, res[2].Chunk.ID}
	wantOrder := []string{"a1", "a2", "b0"}
	for i := range wantOrder {
		if gotOrder[i] != wantOrder[i] {
			t.Fatalf("unexpected order: want %v, got %v", wantOrder, gotOrder)
		}
	}
}

func TestRetrieveTopChunksDefaultsTopKToFour(t *testing.T) {
	longTail := strings.Repeat("y", 140)
	doc := types.Document{ID: "doc_1", Name: "n.md", CreatedAt: time.Now()}
	chunks := map[string][]types.Chunk{
		"doc_1": {
			{ID: "c1", DocID: "doc_1", Index: 1, Content: "register " + longTail},
			{ID: "c2", DocID: "doc_1", Index: 2, Content: "register " + longTail},
			{ID: "c3", DocID: "doc_1", Index: 3, Content: "register " + longTail},
			{ID: "c4", DocID: "doc_1", Index: 4, Content: "register " + longTail},
			{ID: "c5", DocID: "doc_1", Index: 5, Content: "register " + longTail},
		},
	}

	res := RetrieveTopChunks("register", []types.Document{doc}, chunks, 0)
	if len(res) != 4 {
		t.Fatalf("expected default topK=4 results, got %d", len(res))
	}
}

func TestRetrieveTopChunksReturnsEmptyWhenNoMatch(t *testing.T) {
	doc := types.Document{ID: "doc_1", Name: "n.md", CreatedAt: time.Now()}
	chunks := map[string][]types.Chunk{
		"doc_1": {
			{ID: "c1", DocID: "doc_1", Index: 0, Content: "alpha beta gamma"},
		},
	}

	res := RetrieveTopChunks("register", []types.Document{doc}, chunks, 2)
	if len(res) != 0 {
		t.Fatalf("expected empty result, got %d", len(res))
	}
}
