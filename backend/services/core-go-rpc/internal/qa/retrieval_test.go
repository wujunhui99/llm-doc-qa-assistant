package qa

import (
	"testing"
	"time"

	"llm-doc-qa-assistant/backend/services/core-go-rpc/internal/types"
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
