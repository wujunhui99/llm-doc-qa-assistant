package llm

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

type ChatMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type SiliconFlowChatClient struct {
	apiBase string
	apiKey  string
	client  *http.Client
}

func NewSiliconFlowChatClient(apiBase, apiKey string, timeout time.Duration) *SiliconFlowChatClient {
	apiBase = strings.TrimRight(strings.TrimSpace(apiBase), "/")
	if apiBase == "" {
		apiBase = "https://api.siliconflow.cn/v1"
	}
	if timeout <= 0 {
		timeout = 30 * time.Second
	}
	return &SiliconFlowChatClient{
		apiBase: apiBase,
		apiKey:  strings.TrimSpace(apiKey),
		client:  &http.Client{Timeout: timeout},
	}
}

func (c *SiliconFlowChatClient) Available() bool {
	return strings.TrimSpace(c.apiKey) != ""
}

func (c *SiliconFlowChatClient) ChatCompletion(ctx context.Context, messages []ChatMessage, model string, temperature float64) (string, error) {
	if !c.Available() {
		return "", fmt.Errorf("SILICONFLOW_API_KEY is empty")
	}
	payload := map[string]any{
		"model":       strings.TrimSpace(model),
		"messages":    messages,
		"temperature": temperature,
	}
	rawBody, err := json.Marshal(payload)
	if err != nil {
		return "", fmt.Errorf("marshal request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.apiBase+"/chat/completions", bytes.NewReader(rawBody))
	if err != nil {
		return "", fmt.Errorf("create request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+c.apiKey)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")

	resp, err := c.client.Do(req)
	if err != nil {
		return "", fmt.Errorf("call chat api: %w", err)
	}
	defer resp.Body.Close()

	respBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("read response: %w", err)
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return "", fmt.Errorf("chat api status=%d body=%s", resp.StatusCode, string(respBytes))
	}

	var parsed struct {
		Choices []struct {
			Message struct {
				Content string `json:"content"`
			} `json:"message"`
		} `json:"choices"`
	}
	if err := json.Unmarshal(respBytes, &parsed); err != nil {
		return "", fmt.Errorf("decode response: %w", err)
	}
	if len(parsed.Choices) == 0 {
		return "", fmt.Errorf("invalid chat response: choices is empty")
	}
	return strings.TrimSpace(parsed.Choices[0].Message.Content), nil
}
