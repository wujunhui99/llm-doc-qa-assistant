package llm

import (
	"bytes"
	"context"
	"io"
	"net/http"
	"strings"
	"testing"
	"time"
)

type rtFunc func(req *http.Request) (*http.Response, error)

func (f rtFunc) RoundTrip(req *http.Request) (*http.Response, error) {
	return f(req)
}

func TestSiliconFlowChatCompletionSuccess(t *testing.T) {
	client := NewSiliconFlowChatClient("https://api.siliconflow.cn/v1", "test-key", time.Second)
	client.client = &http.Client{
		Transport: rtFunc(func(req *http.Request) (*http.Response, error) {
			if req.URL.Path != "/v1/chat/completions" {
				t.Fatalf("unexpected path: %s", req.URL.Path)
			}
			if got := req.Header.Get("Authorization"); got != "Bearer test-key" {
				t.Fatalf("unexpected auth header: %s", got)
			}
			body, _ := io.ReadAll(req.Body)
			if !strings.Contains(string(body), "\"model\":\"Pro/MiniMaxAI/MiniMax-M2.5\"") {
				t.Fatalf("model missing in request body: %s", string(body))
			}
			return &http.Response{
				StatusCode: http.StatusOK,
				Body:       io.NopCloser(bytes.NewBufferString(`{"choices":[{"message":{"content":"answer"}}]}`)),
				Header:     make(http.Header),
			}, nil
		}),
	}

	out, err := client.ChatCompletion(context.Background(), []ChatMessage{{Role: "user", Content: "hello"}}, "Pro/MiniMaxAI/MiniMax-M2.5", 0.2)
	if err != nil {
		t.Fatalf("chat completion failed: %v", err)
	}
	if out != "answer" {
		t.Fatalf("unexpected answer: %q", out)
	}
}

func TestSiliconFlowChatCompletionUnavailable(t *testing.T) {
	client := NewSiliconFlowChatClient("https://api.siliconflow.cn/v1", "", time.Second)
	_, err := client.ChatCompletion(context.Background(), []ChatMessage{{Role: "user", Content: "hello"}}, "model", 0.2)
	if err == nil {
		t.Fatalf("expected error for missing api key")
	}
}
