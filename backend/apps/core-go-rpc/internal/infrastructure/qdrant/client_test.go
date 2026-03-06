package qdrant

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"testing"
	"time"

	"llm-doc-qa-assistant/backend/apps/core-go-rpc/internal/types"
)

type testRoundTrip func(req *http.Request) (*http.Response, error)

func (f testRoundTrip) RoundTrip(req *http.Request) (*http.Response, error) {
	return f(req)
}

func TestPointIDDeterministic(t *testing.T) {
	a := pointID("u1", "d1", "c1")
	b := pointID("u1", "d1", "c1")
	c := pointID("u1", "d1", "c2")
	if a != b {
		t.Fatalf("expected deterministic id, got %d and %d", a, b)
	}
	if a == c {
		t.Fatalf("expected different point id for different chunk ids")
	}
}

func TestUpsertDocumentChunks(t *testing.T) {
	var gotCreate bool
	var gotUpsert bool

	client := NewClient("http://qdrant.local", "", "qa_chunks", 2*time.Second)
	client.client = &http.Client{
		Transport: testRoundTrip(func(r *http.Request) (*http.Response, error) {
			switch {
			case r.Method == http.MethodGet && r.URL.Path == "/collections/qa_chunks":
				return jsonResp(http.StatusNotFound, `{"status":"ok","result":null}`), nil
			case r.Method == http.MethodPut && r.URL.Path == "/collections/qa_chunks":
				gotCreate = true
				return jsonResp(http.StatusOK, `{"status":"ok","result":true}`), nil
			case r.Method == http.MethodPut && r.URL.Path == "/collections/qa_chunks/points":
				gotUpsert = true
				body, _ := io.ReadAll(r.Body)
				var payload map[string]any
				if err := json.Unmarshal(body, &payload); err != nil {
					t.Fatalf("decode upsert body failed: %v", err)
				}
				points, ok := payload["points"].([]any)
				if !ok || len(points) != 1 {
					t.Fatalf("expected one point, got %#v", payload["points"])
				}
				return jsonResp(http.StatusOK, `{"status":"ok","result":{"operation_id":1}}`), nil
			default:
				t.Fatalf("unexpected request: %s %s", r.Method, r.URL.String())
				return nil, nil
			}
		}),
	}

	doc := types.Document{ID: "doc_1", Name: "file.md"}
	chunks := []types.Chunk{{ID: "chk_1", DocID: "doc_1", Index: 0, Content: "hello"}}
	vectors := [][]float32{{0.1, 0.2, 0.3}}

	if err := client.UpsertDocumentChunks(context.Background(), "usr_1", doc, chunks, vectors); err != nil {
		t.Fatalf("upsert failed: %v", err)
	}
	if !gotCreate {
		t.Fatalf("expected collection create call")
	}
	if !gotUpsert {
		t.Fatalf("expected points upsert call")
	}
}

func TestSearchChunksParsesResponse(t *testing.T) {
	client := NewClient("http://qdrant.local", "", "qa_chunks", 2*time.Second)
	client.client = &http.Client{
		Transport: testRoundTrip(func(r *http.Request) (*http.Response, error) {
			switch {
			case r.Method == http.MethodGet && r.URL.Path == "/collections/qa_chunks":
				return jsonResp(http.StatusOK, `{"status":"ok","result":{"status":"green"}}`), nil
			case r.Method == http.MethodPut && r.URL.Path == "/collections/qa_chunks":
				return jsonResp(http.StatusConflict, `{"status":"error","result":null}`), nil
			case r.Method == http.MethodPost && r.URL.Path == "/collections/qa_chunks/points/search":
				bodyBytes, _ := io.ReadAll(r.Body)
				if !strings.Contains(string(bodyBytes), "owner_user_id") {
					t.Fatalf("expected owner filter in search request, body=%s", string(bodyBytes))
				}
				return jsonResp(http.StatusOK, `{"status":"ok","result":[{"score":0.99,"payload":{"doc_id":"doc_1","doc_name":"n.md","chunk_id":"chk_1","chunk_index":3,"content":"ctx"}}]}`), nil
			default:
				t.Fatalf("unexpected request: %s %s", r.Method, r.URL.String())
				return nil, nil
			}
		}),
	}

	hits, err := client.SearchChunks(context.Background(), "usr_1", []float32{0.1, 0.2}, 3)
	if err != nil {
		t.Fatalf("search failed: %v", err)
	}
	if len(hits) != 1 {
		t.Fatalf("expected 1 hit, got %d", len(hits))
	}
	if hits[0].DocID != "doc_1" || hits[0].ChunkID != "chk_1" || hits[0].ChunkIndex != 3 {
		t.Fatalf("unexpected hit: %+v", hits[0])
	}
}

func jsonResp(status int, body string) *http.Response {
	return &http.Response{
		StatusCode: status,
		Body:       io.NopCloser(bytes.NewBufferString(body)),
		Header:     http.Header{"Content-Type": []string{"application/json"}},
	}
}
