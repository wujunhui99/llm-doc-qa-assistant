package llm

import (
	"context"
	"errors"
	"testing"
	"time"
)

type fakeEmbedder struct {
	vectors [][]float32
	err     error
}

func (f fakeEmbedder) Embed(_ context.Context, _ []string) ([][]float32, error) {
	if f.err != nil {
		return nil, f.err
	}
	return f.vectors, nil
}

type fakeChat struct {
	available bool
	answer    string
	err       error
	lastModel string
}

func (f *fakeChat) Available() bool { return f.available }

func (f *fakeChat) ChatCompletion(_ context.Context, _ []ChatMessage, model string, _ float64) (string, error) {
	f.lastModel = model
	if f.err != nil {
		return "", f.err
	}
	return f.answer, nil
}

func TestAgentUsesChatWhenNoContexts(t *testing.T) {
	a := NewAgent(&fakeChat{available: true, answer: "ok"}, nil, Config{})
	out, err := a.GenerateAnswer(context.Background(), Request{
		Question:           "q",
		PreviousTurnAnswer: "prev",
		Contexts:           nil,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if out != "ok" {
		t.Fatalf("expected chat output, got %q", out)
	}
}

func TestAgentNoContextsReturnsErrorWhenChatUnavailable(t *testing.T) {
	a := NewAgent(&fakeChat{available: false}, nil, Config{})
	_, err := a.GenerateAnswer(context.Background(), Request{
		Question: "你好",
		Contexts: nil,
	})
	if !errors.Is(err, ErrUnavailable) {
		t.Fatalf("expected ErrUnavailable, got %v", err)
	}
}

func TestAgentUsesProviderModelMapping(t *testing.T) {
	chat := &fakeChat{available: true, answer: "model ok"}
	a := NewAgent(chat, nil, Config{
		ChatModel:       "default-model",
		DefaultProvider: "siliconflow",
		ProviderChatModel: map[string]string{
			"openai": "mapped-model",
		},
	})
	_, err := a.GenerateAnswer(context.Background(), Request{
		Question:       "q",
		ActiveProvider: "openai",
		Contexts: []ContextChunk{
			{DocID: "d1", DocName: "doc", ChunkID: "c1", ChunkIndex: 0, Content: "ctx"},
		},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if chat.lastModel != "mapped-model" {
		t.Fatalf("expected mapped model, got %s", chat.lastModel)
	}
}

func TestAgentReranksByEmbeddingSimilarity(t *testing.T) {
	chat := &fakeChat{available: true, answer: "ok"}
	embedder := fakeEmbedder{
		// vectors: question, ctx1, ctx2
		vectors: [][]float32{
			{1, 0},
			{0.2, 0.8},
			{0.9, 0.1},
		},
	}
	a := NewAgent(chat, embedder, Config{
		MaxContextChunks: 1,
		RequestTimeout:   2 * time.Second,
	})

	req := Request{
		Question: "which context?",
		Contexts: []ContextChunk{
			{DocID: "d1", ChunkID: "c1", ChunkIndex: 0, Content: "first"},
			{DocID: "d1", ChunkID: "c2", ChunkIndex: 1, Content: "second"},
		},
	}
	out, err := a.GenerateAnswer(context.Background(), req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if out != "ok" {
		t.Fatalf("expected chat output, got %q", out)
	}
}

func TestAgentReturnsErrorWhenChatFails(t *testing.T) {
	chat := &fakeChat{available: true, err: errors.New("chat down")}
	a := NewAgent(chat, nil, Config{})

	_, err := a.GenerateAnswer(context.Background(), Request{
		Question: "q",
		Contexts: []ContextChunk{
			{DocID: "d1", DocName: "doc", ChunkID: "c1", ChunkIndex: 0, Content: "ctx"},
		},
	})
	if err == nil {
		t.Fatalf("expected error, got nil")
	}
}
