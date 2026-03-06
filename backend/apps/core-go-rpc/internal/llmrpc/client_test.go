package llmrpc

import (
	"context"
	"errors"
	"testing"

	corerpc "llm-doc-qa-assistant/backend/apps/core-go-rpc/internal/rpc"
	qav1 "llm-doc-qa-assistant/backend/proto/gen/go/qa/v1"

	"google.golang.org/grpc"
)

type fakeLlmGRPCClient struct {
	embedFn    func(ctx context.Context, in *qav1.EmbedTextsRequest, opts ...grpc.CallOption) (*qav1.EmbedTextsReply, error)
	generateFn func(ctx context.Context, in *qav1.GenerateAnswerRequest, opts ...grpc.CallOption) (*qav1.GenerateAnswerReply, error)
}

func (f *fakeLlmGRPCClient) Health(ctx context.Context, in *qav1.Empty, opts ...grpc.CallOption) (*qav1.HealthReply, error) {
	return &qav1.HealthReply{Status: "ok"}, nil
}

func (f *fakeLlmGRPCClient) EmbedTexts(ctx context.Context, in *qav1.EmbedTextsRequest, opts ...grpc.CallOption) (*qav1.EmbedTextsReply, error) {
	if f.embedFn != nil {
		return f.embedFn(ctx, in, opts...)
	}
	return &qav1.EmbedTextsReply{}, nil
}

func (f *fakeLlmGRPCClient) GenerateAnswer(ctx context.Context, in *qav1.GenerateAnswerRequest, opts ...grpc.CallOption) (*qav1.GenerateAnswerReply, error) {
	if f.generateFn != nil {
		return f.generateFn(ctx, in, opts...)
	}
	return &qav1.GenerateAnswerReply{Answer: "ok"}, nil
}

func TestClientEmbedTexts(t *testing.T) {
	c := New(&fakeLlmGRPCClient{
		embedFn: func(_ context.Context, in *qav1.EmbedTextsRequest, _ ...grpc.CallOption) (*qav1.EmbedTextsReply, error) {
			if len(in.GetTexts()) != 2 {
				t.Fatalf("unexpected texts size: %d", len(in.GetTexts()))
			}
			return &qav1.EmbedTextsReply{
				Vectors: []*qav1.EmbeddingVector{
					{Values: []float32{0.1, 0.2}},
					{Values: []float32{0.3, 0.4}},
				},
			}, nil
		},
	})

	out, err := c.EmbedTexts(context.Background(), []string{"a", "b"})
	if err != nil {
		t.Fatalf("embed failed: %v", err)
	}
	if len(out) != 2 {
		t.Fatalf("expected 2 vectors, got %d", len(out))
	}
	if out[0][0] != float32(0.1) || out[1][1] != float32(0.4) {
		t.Fatalf("unexpected vectors: %+v", out)
	}
}

func TestClientGenerateAnswerMapsFields(t *testing.T) {
	c := New(&fakeLlmGRPCClient{
		generateFn: func(_ context.Context, in *qav1.GenerateAnswerRequest, _ ...grpc.CallOption) (*qav1.GenerateAnswerReply, error) {
			if in.GetActiveProvider() != "siliconflow" {
				t.Fatalf("unexpected provider: %s", in.GetActiveProvider())
			}
			if len(in.GetContexts()) != 1 || in.GetContexts()[0].GetChunkId() != "chk_1" {
				t.Fatalf("unexpected contexts: %+v", in.GetContexts())
			}
			return &qav1.GenerateAnswerReply{Answer: "answer ok"}, nil
		},
	})

	out, err := c.GenerateAnswer(context.Background(), corerpc.LLMGenerateRequest{
		OwnerUserID:    "usr_1",
		ThreadID:       "th_1",
		TurnID:         "turn_1",
		Question:       "项目概述",
		ScopeType:      "doc",
		ScopeDocIDs:    []string{"doc_1"},
		ActiveProvider: "siliconflow",
		Contexts: []corerpc.LLMContextChunk{
			{DocID: "doc_1", DocName: "doc.md", ChunkID: "chk_1", ChunkIndex: 0, Content: "项目概述段落"},
		},
	})
	if err != nil {
		t.Fatalf("generate answer failed: %v", err)
	}
	if out != "answer ok" {
		t.Fatalf("unexpected answer: %s", out)
	}
}

func TestClientGenerateAnswerFailsOnEmpty(t *testing.T) {
	c := New(&fakeLlmGRPCClient{
		generateFn: func(_ context.Context, _ *qav1.GenerateAnswerRequest, _ ...grpc.CallOption) (*qav1.GenerateAnswerReply, error) {
			return &qav1.GenerateAnswerReply{Answer: "   "}, nil
		},
	})
	if _, err := c.GenerateAnswer(context.Background(), corerpc.LLMGenerateRequest{}); err == nil {
		t.Fatalf("expected error for empty answer")
	}
}

func TestClientEmbedTextsCountMismatch(t *testing.T) {
	c := New(&fakeLlmGRPCClient{
		embedFn: func(_ context.Context, _ *qav1.EmbedTextsRequest, _ ...grpc.CallOption) (*qav1.EmbedTextsReply, error) {
			return &qav1.EmbedTextsReply{Vectors: []*qav1.EmbeddingVector{{Values: []float32{0.1}}}}, nil
		},
	})
	_, err := c.EmbedTexts(context.Background(), []string{"a", "b"})
	if err == nil {
		t.Fatalf("expected mismatch error")
	}
}

func TestClientEmbedTextsPassesThroughErrors(t *testing.T) {
	c := New(&fakeLlmGRPCClient{
		embedFn: func(_ context.Context, _ *qav1.EmbedTextsRequest, _ ...grpc.CallOption) (*qav1.EmbedTextsReply, error) {
			return nil, errors.New("rpc down")
		},
	})
	if _, err := c.EmbedTexts(context.Background(), []string{"a"}); err == nil {
		t.Fatalf("expected error")
	}
}
