package qdrant

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"hash/fnv"
	"io"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"

	"llm-doc-qa-assistant/backend/apps/core-go-rpc/internal/types"
)

type Client struct {
	endpoint   string
	apiKey     string
	collection string
	client     *http.Client

	mu            sync.Mutex
	ensured       bool
	ensuredVector int
}

func NewClient(endpoint, apiKey, collection string, timeout time.Duration) *Client {
	endpoint = strings.TrimRight(strings.TrimSpace(endpoint), "/")
	collection = strings.TrimSpace(collection)
	if timeout <= 0 {
		timeout = 10 * time.Second
	}
	return &Client{
		endpoint:   endpoint,
		apiKey:     strings.TrimSpace(apiKey),
		collection: collection,
		client:     &http.Client{Timeout: timeout},
	}
}

func (c *Client) Enabled() bool {
	return c.endpoint != "" && c.collection != ""
}

func (c *Client) UpsertDocumentChunks(ctx context.Context, ownerID string, doc types.Document, chunks []types.Chunk, vectors [][]float32) error {
	if !c.Enabled() {
		return errors.New("qdrant is disabled")
	}
	if len(chunks) == 0 {
		return nil
	}
	if len(chunks) != len(vectors) {
		return fmt.Errorf("chunks/vectors size mismatch: chunks=%d vectors=%d", len(chunks), len(vectors))
	}
	dim := len(vectors[0])
	if dim <= 0 {
		return errors.New("embedding vector is empty")
	}
	if err := c.ensureCollection(ctx, dim); err != nil {
		return err
	}

	type point struct {
		ID      uint64         `json:"id"`
		Vector  []float32      `json:"vector"`
		Payload map[string]any `json:"payload"`
	}
	points := make([]point, 0, len(chunks))
	for i := range chunks {
		chunk := chunks[i]
		vector := vectors[i]
		if len(vector) != dim {
			return fmt.Errorf("inconsistent vector size at index=%d: want=%d got=%d", i, dim, len(vector))
		}
		points = append(points, point{
			ID:     pointID(ownerID, doc.ID, chunk.ID),
			Vector: vector,
			Payload: map[string]any{
				"owner_user_id": ownerID,
				"doc_id":        doc.ID,
				"doc_name":      doc.Name,
				"chunk_id":      chunk.ID,
				"chunk_index":   chunk.Index,
				"content":       chunk.Content,
			},
		})
	}

	reqBody := map[string]any{"points": points}
	return c.doJSON(ctx, http.MethodPut, "/collections/"+url.PathEscape(c.collection)+"/points?wait=true", reqBody, nil, http.StatusOK)
}

func (c *Client) SearchChunks(ctx context.Context, ownerID string, queryVector []float32, limit int) ([]types.VectorHit, error) {
	if !c.Enabled() {
		return nil, errors.New("qdrant is disabled")
	}
	if len(queryVector) == 0 {
		return nil, errors.New("query vector is empty")
	}
	if limit <= 0 {
		limit = 5
	}
	if err := c.ensureCollection(ctx, len(queryVector)); err != nil {
		return nil, err
	}

	reqBody := map[string]any{
		"vector":       queryVector,
		"limit":        limit,
		"with_payload": true,
		"filter": map[string]any{
			"must": []map[string]any{
				{
					"key": "owner_user_id",
					"match": map[string]any{
						"value": ownerID,
					},
				},
			},
		},
	}

	var resp struct {
		Result []struct {
			Score   float64        `json:"score"`
			Payload map[string]any `json:"payload"`
		} `json:"result"`
	}
	if err := c.doJSON(ctx, http.MethodPost, "/collections/"+url.PathEscape(c.collection)+"/points/search", reqBody, &resp, http.StatusOK); err != nil {
		return nil, err
	}

	hits := make([]types.VectorHit, 0, len(resp.Result))
	for _, item := range resp.Result {
		docID, _ := item.Payload["doc_id"].(string)
		chunkID, _ := item.Payload["chunk_id"].(string)
		if docID == "" || chunkID == "" {
			continue
		}
		docName, _ := item.Payload["doc_name"].(string)
		content, _ := item.Payload["content"].(string)
		hits = append(hits, types.VectorHit{
			DocID:      docID,
			DocName:    docName,
			ChunkID:    chunkID,
			ChunkIndex: payloadInt(item.Payload["chunk_index"]),
			Content:    content,
			Score:      item.Score,
		})
	}
	return hits, nil
}

func (c *Client) DeleteDocument(ctx context.Context, ownerID, docID string) error {
	if !c.Enabled() {
		return nil
	}
	if strings.TrimSpace(docID) == "" {
		return nil
	}

	reqBody := map[string]any{
		"filter": map[string]any{
			"must": []map[string]any{
				{
					"key": "owner_user_id",
					"match": map[string]any{
						"value": ownerID,
					},
				},
				{
					"key": "doc_id",
					"match": map[string]any{
						"value": docID,
					},
				},
			},
		},
	}

	err := c.doJSON(ctx, http.MethodPost, "/collections/"+url.PathEscape(c.collection)+"/points/delete?wait=true", reqBody, nil, http.StatusOK)
	if err == nil {
		return nil
	}
	if strings.Contains(err.Error(), "status=404") {
		return nil
	}
	return err
}

func (c *Client) ensureCollection(ctx context.Context, vectorSize int) error {
	if vectorSize <= 0 {
		return errors.New("vector size must be positive")
	}

	c.mu.Lock()
	defer c.mu.Unlock()

	if c.ensured {
		if c.ensuredVector != vectorSize {
			return fmt.Errorf("qdrant collection vector size mismatch: existing=%d incoming=%d", c.ensuredVector, vectorSize)
		}
		return nil
	}

	path := "/collections/" + url.PathEscape(c.collection)
	status, err := c.doJSONWithStatus(ctx, http.MethodGet, path, nil, nil, http.StatusOK, http.StatusNotFound)
	if err != nil {
		return err
	}
	if status == http.StatusNotFound {
		createReq := map[string]any{
			"vectors": map[string]any{
				"size":     vectorSize,
				"distance": "Cosine",
			},
		}
		if err := c.doJSON(ctx, http.MethodPut, path, createReq, nil, http.StatusOK); err != nil {
			return err
		}
	}

	c.ensured = true
	c.ensuredVector = vectorSize
	return nil
}

func (c *Client) doJSON(ctx context.Context, method, path string, reqBody any, respBody any, expected ...int) error {
	_, err := c.doJSONWithStatus(ctx, method, path, reqBody, respBody, expected...)
	return err
}

func (c *Client) doJSONWithStatus(ctx context.Context, method, path string, reqBody any, respBody any, expected ...int) (int, error) {
	var bodyReader io.Reader
	if reqBody != nil {
		data, err := json.Marshal(reqBody)
		if err != nil {
			return 0, fmt.Errorf("marshal request: %w", err)
		}
		bodyReader = bytes.NewReader(data)
	}

	req, err := http.NewRequestWithContext(ctx, method, c.endpoint+path, bodyReader)
	if err != nil {
		return 0, fmt.Errorf("create request: %w", err)
	}
	if reqBody != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	if c.apiKey != "" {
		req.Header.Set("api-key", c.apiKey)
	}

	resp, err := c.client.Do(req)
	if err != nil {
		return 0, fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	respBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return 0, fmt.Errorf("read response: %w", err)
	}

	if !containsStatus(expected, resp.StatusCode) {
		return resp.StatusCode, fmt.Errorf("qdrant request failed: status=%d body=%s", resp.StatusCode, string(respBytes))
	}

	if respBody != nil && len(respBytes) > 0 {
		if err := json.Unmarshal(respBytes, respBody); err != nil {
			return resp.StatusCode, fmt.Errorf("decode response: %w", err)
		}
	}
	return resp.StatusCode, nil
}

func containsStatus(expected []int, status int) bool {
	if len(expected) == 0 {
		return status >= 200 && status < 300
	}
	for _, s := range expected {
		if s == status {
			return true
		}
	}
	return false
}

func pointID(ownerID, docID, chunkID string) uint64 {
	h := fnv.New64a()
	_, _ = h.Write([]byte(ownerID))
	_, _ = h.Write([]byte{0})
	_, _ = h.Write([]byte(docID))
	_, _ = h.Write([]byte{0})
	_, _ = h.Write([]byte(chunkID))
	return h.Sum64()
}

func payloadInt(v any) int {
	switch x := v.(type) {
	case int:
		return x
	case int32:
		return int(x)
	case int64:
		return int(x)
	case float32:
		return int(x)
	case float64:
		return int(x)
	default:
		return 0
	}
}
