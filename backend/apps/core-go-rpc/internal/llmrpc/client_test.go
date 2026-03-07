package llmrpc

import (
	"context"
	"errors"
	"io"
	"testing"

	corerpc "llm-doc-qa-assistant/backend/apps/core-go-rpc/internal/rpc"
	qav1 "llm-doc-qa-assistant/backend/proto/gen/go/qa/v1"

	"google.golang.org/grpc"
	"google.golang.org/grpc/metadata"
)

type fakeLlmGRPCClient struct {
	embedFn    func(ctx context.Context, in *qav1.EmbedTextsRequest, opts ...grpc.CallOption) (*qav1.EmbedTextsReply, error)
	extractFn  func(ctx context.Context, in *qav1.ExtractDocumentTextRequest, opts ...grpc.CallOption) (*qav1.ExtractDocumentTextReply, error)
	generateFn func(ctx context.Context, in *qav1.GenerateAnswerRequest, opts ...grpc.CallOption) (*qav1.GenerateAnswerReply, error)
	streamFn   func(ctx context.Context, in *qav1.GenerateAnswerRequest, opts ...grpc.CallOption) (qav1.LlmService_StreamGenerateAnswerClient, error)
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

func (f *fakeLlmGRPCClient) StreamGenerateAnswer(ctx context.Context, in *qav1.GenerateAnswerRequest, opts ...grpc.CallOption) (qav1.LlmService_StreamGenerateAnswerClient, error) {
	if f.streamFn != nil {
		return f.streamFn(ctx, in, opts...)
	}
	return nil, errors.New("stream generate not mocked")
}

func (f *fakeLlmGRPCClient) ExtractDocumentText(ctx context.Context, in *qav1.ExtractDocumentTextRequest, opts ...grpc.CallOption) (*qav1.ExtractDocumentTextReply, error) {
	if f.extractFn != nil {
		return f.extractFn(ctx, in, opts...)
	}
	return &qav1.ExtractDocumentTextReply{Text: ""}, nil
}

type fakeStreamGenerateClient struct {
	chunks []*qav1.GenerateAnswerChunk
	idx    int
}

func (f *fakeStreamGenerateClient) Recv() (*qav1.GenerateAnswerChunk, error) {
	if f.idx >= len(f.chunks) {
		return nil, io.EOF
	}
	chunk := f.chunks[f.idx]
	f.idx++
	return chunk, nil
}

func (f *fakeStreamGenerateClient) Header() (metadata.MD, error) {
	return nil, nil
}

func (f *fakeStreamGenerateClient) Trailer() metadata.MD {
	return nil
}

func (f *fakeStreamGenerateClient) CloseSend() error {
	return nil
}

func (f *fakeStreamGenerateClient) Context() context.Context {
	return context.Background()
}

func (f *fakeStreamGenerateClient) SendMsg(_ interface{}) error {
	return nil
}

func (f *fakeStreamGenerateClient) RecvMsg(_ interface{}) error {
	return nil
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
			if !in.GetThinkMode() {
				t.Fatalf("expected think mode true")
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
		ThinkMode:      true,
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

func TestClientGenerateAnswerStream(t *testing.T) {
	c := New(&fakeLlmGRPCClient{
		streamFn: func(_ context.Context, in *qav1.GenerateAnswerRequest, _ ...grpc.CallOption) (qav1.LlmService_StreamGenerateAnswerClient, error) {
			if in.GetActiveProvider() != "ollama" {
				t.Fatalf("unexpected provider: %s", in.GetActiveProvider())
			}
			if !in.GetThinkMode() {
				t.Fatalf("expected think mode true")
			}
			return &fakeStreamGenerateClient{
				chunks: []*qav1.GenerateAnswerChunk{
					{ThinkingDelta: "思考", Done: false},
					{Delta: "你", Done: false},
					{Delta: "好", Done: false},
					{Answer: "你好", Done: true},
				},
			}, nil
		},
	})

	var got string
	var gotThinking string
	out, err := c.GenerateAnswerStream(context.Background(), corerpc.LLMGenerateRequest{
		ActiveProvider: "ollama",
		ThinkMode:      true,
	}, func(delta string, thinkingDelta string) error {
		got += delta
		gotThinking += thinkingDelta
		return nil
	})
	if err != nil {
		t.Fatalf("stream generate failed: %v", err)
	}
	if out != "你好" {
		t.Fatalf("unexpected final answer: %q", out)
	}
	if got != "你好" {
		t.Fatalf("unexpected streamed delta: %q", got)
	}
	if gotThinking != "思考" {
		t.Fatalf("unexpected streamed thinking: %q", gotThinking)
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

func TestClientExtractDocumentText(t *testing.T) {
	c := New(&fakeLlmGRPCClient{
		extractFn: func(_ context.Context, in *qav1.ExtractDocumentTextRequest, _ ...grpc.CallOption) (*qav1.ExtractDocumentTextReply, error) {
			if in.GetFilename() != "a.pdf" || in.GetMimeType() != "application/pdf" {
				t.Fatalf("unexpected extract request: %+v", in)
			}
			return &qav1.ExtractDocumentTextReply{Text: "项目概述 内容"}, nil
		},
	})

	out, err := c.ExtractDocumentText(context.Background(), "a.pdf", "application/pdf", []byte("raw"))
	if err != nil {
		t.Fatalf("extract failed: %v", err)
	}
	if out != "项目概述 内容" {
		t.Fatalf("unexpected extract output: %q", out)
	}
}
