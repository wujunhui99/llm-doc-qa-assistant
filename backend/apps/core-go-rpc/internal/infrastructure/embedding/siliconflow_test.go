package embedding

import (
	"bytes"
	"context"
	"io"
	"net/http"
	"strings"
	"testing"
	"time"
)

type roundTripFunc func(req *http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(req *http.Request) (*http.Response, error) {
	return f(req)
}

func TestSiliconFlowEmbedderEmbedSuccess(t *testing.T) {
	embedder := NewSiliconFlowEmbedder("https://api.siliconflow.cn/v1", "test-key", "Qwen/Qwen3-Embedding-4B", 2*time.Second)
	embedder.client = &http.Client{
		Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
			if r.Method != http.MethodPost {
				t.Fatalf("unexpected method: %s", r.Method)
			}
			if r.URL.Path != "/v1/embeddings" {
				t.Fatalf("unexpected path: %s", r.URL.Path)
			}
			if got := r.Header.Get("Authorization"); got != "Bearer test-key" {
				t.Fatalf("unexpected authorization header: %q", got)
			}
			body, _ := io.ReadAll(r.Body)
			if !strings.Contains(string(body), "\"model\":\"Qwen/Qwen3-Embedding-4B\"") {
				t.Fatalf("request should include embedding model, body=%s", string(body))
			}
			return &http.Response{
				StatusCode: http.StatusOK,
				Body:       io.NopCloser(bytes.NewBufferString(`{"data":[{"embedding":[0.1,0.2,0.3]},{"embedding":[0.9,0.8,0.7]}]}`)),
				Header:     make(http.Header),
			}, nil
		}),
	}

	vectors, err := embedder.Embed(context.Background(), []string{"hello", "world"})
	if err != nil {
		t.Fatalf("embed failed: %v", err)
	}
	if len(vectors) != 2 {
		t.Fatalf("expected 2 vectors, got %d", len(vectors))
	}
	if len(vectors[0]) != 3 || len(vectors[1]) != 3 {
		t.Fatalf("unexpected vector dimensions: %d, %d", len(vectors[0]), len(vectors[1]))
	}
}

func TestSiliconFlowEmbedderEmbedFailsWhenAPIKeyMissing(t *testing.T) {
	embedder := NewSiliconFlowEmbedder("https://api.siliconflow.cn/v1", "", "Qwen/Qwen3-Embedding-4B", time.Second)
	_, err := embedder.Embed(context.Background(), []string{"hello"})
	if err == nil {
		t.Fatalf("expected missing api key error")
	}
}

func TestSiliconFlowEmbedderEmbedFailsOnCountMismatch(t *testing.T) {
	embedder := NewSiliconFlowEmbedder("https://api.siliconflow.cn/v1", "test-key", "Qwen/Qwen3-Embedding-4B", time.Second)
	embedder.client = &http.Client{
		Transport: roundTripFunc(func(_ *http.Request) (*http.Response, error) {
			return &http.Response{
				StatusCode: http.StatusOK,
				Body:       io.NopCloser(bytes.NewBufferString(`{"data":[{"embedding":[0.1,0.2]}]}`)),
				Header:     make(http.Header),
			}, nil
		}),
	}

	_, err := embedder.Embed(context.Background(), []string{"a", "b"})
	if err == nil {
		t.Fatalf("expected mismatch error")
	}
}
