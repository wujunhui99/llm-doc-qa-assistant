package embedding

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

type SiliconFlowEmbedder struct {
	apiBase string
	apiKey  string
	model   string
	client  *http.Client
}

func NewSiliconFlowEmbedder(apiBase, apiKey, model string, timeout time.Duration) *SiliconFlowEmbedder {
	apiBase = strings.TrimRight(strings.TrimSpace(apiBase), "/")
	if apiBase == "" {
		apiBase = "https://api.siliconflow.cn/v1"
	}
	if strings.TrimSpace(model) == "" {
		model = "Qwen/Qwen3-Embedding-4B"
	}
	if timeout <= 0 {
		timeout = 30 * time.Second
	}
	return &SiliconFlowEmbedder{
		apiBase: apiBase,
		apiKey:  strings.TrimSpace(apiKey),
		model:   strings.TrimSpace(model),
		client:  &http.Client{Timeout: timeout},
	}
}

func (e *SiliconFlowEmbedder) Enabled() bool {
	return strings.TrimSpace(e.apiKey) != ""
}

func (e *SiliconFlowEmbedder) Embed(ctx context.Context, texts []string) ([][]float32, error) {
	if len(texts) == 0 {
		return nil, nil
	}
	if !e.Enabled() {
		return nil, errors.New("siliconflow api key is empty")
	}

	reqBody := map[string]any{
		"model": e.model,
		"input": texts,
	}
	bodyBytes, err := json.Marshal(reqBody)
	if err != nil {
		return nil, fmt.Errorf("marshal embedding request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, e.apiBase+"/embeddings", bytes.NewReader(bodyBytes))
	if err != nil {
		return nil, fmt.Errorf("create embedding request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+e.apiKey)
	req.Header.Set("Content-Type", "application/json")

	resp, err := e.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("call embeddings api: %w", err)
	}
	defer resp.Body.Close()

	respBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read embeddings response: %w", err)
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("embeddings api status=%d body=%s", resp.StatusCode, string(respBytes))
	}

	var parsed struct {
		Data []struct {
			Embedding []float64 `json:"embedding"`
		} `json:"data"`
	}
	if err := json.Unmarshal(respBytes, &parsed); err != nil {
		return nil, fmt.Errorf("decode embeddings response: %w", err)
	}
	if len(parsed.Data) != len(texts) {
		return nil, fmt.Errorf("embeddings count mismatch: want=%d got=%d", len(texts), len(parsed.Data))
	}

	out := make([][]float32, 0, len(parsed.Data))
	for i, item := range parsed.Data {
		if len(item.Embedding) == 0 {
			return nil, fmt.Errorf("empty embedding vector at index=%d", i)
		}
		vec := make([]float32, len(item.Embedding))
		for j, v := range item.Embedding {
			vec[j] = float32(v)
		}
		out = append(out, vec)
	}
	return out, nil
}
