package handler

import (
	"encoding/json"
	"fmt"
	"net/http"
	"sort"
	"strings"

	"llm-doc-qa-assistant/backend/apps/api-go/internal/middleware"
	apitypes "llm-doc-qa-assistant/backend/apps/api-go/internal/types"
)

func (s *Server) handleThreadsRoot(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		s.handleListThreads(w, r)
	case http.MethodPost:
		s.handleCreateThread(w, r)
	case http.MethodOptions:
		w.WriteHeader(http.StatusNoContent)
	default:
		writeError(w, http.StatusMethodNotAllowed, "method_not_allowed", "method not allowed")
	}
}

func (s *Server) handleListThreads(w http.ResponseWriter, r *http.Request) {
	token := middleware.TokenFromContext(r.Context())
	resp, err := s.threadLogic.ListThreads(r.Context(), token)
	if err != nil {
		writeGRPCError(w, err)
		return
	}

	threads := make([]map[string]interface{}, 0, len(resp.GetThreads()))
	for _, thread := range resp.GetThreads() {
		threads = append(threads, protoThreadMap(thread))
	}
	sort.Slice(threads, func(i, j int) bool {
		return fmt.Sprint(threads[i]["created_at"]) > fmt.Sprint(threads[j]["created_at"])
	})
	writeJSON(w, http.StatusOK, map[string]interface{}{"threads": threads})
}

func (s *Server) handleCreateThread(w http.ResponseWriter, r *http.Request) {
	token := middleware.TokenFromContext(r.Context())
	var req apitypes.CreateThreadRequest
	_ = decodeJSONBody(r, &req)

	resp, err := s.threadLogic.CreateThread(r.Context(), token, req.Title)
	if err != nil {
		writeGRPCError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, map[string]interface{}{"thread": protoThreadMap(resp.GetThread())})
}

func (s *Server) handleThreadsSubRoutes(w http.ResponseWriter, r *http.Request) {
	path := strings.TrimPrefix(r.URL.Path, "/api/threads/")
	parts := strings.Split(path, "/")
	if len(parts) < 2 {
		writeError(w, http.StatusNotFound, "not_found", "route not found")
		return
	}
	threadID := parts[0]

	if len(parts) == 2 && parts[1] == "turns" && r.Method == http.MethodPost {
		s.handleCreateTurn(w, r, threadID)
		return
	}
	if len(parts) == 4 && parts[1] == "turns" && parts[3] == "stream" && r.Method == http.MethodGet {
		s.handleTurnStream(w, r, threadID, parts[2])
		return
	}
	writeError(w, http.StatusNotFound, "not_found", "route not found")
}

func (s *Server) handleCreateTurn(w http.ResponseWriter, r *http.Request, threadID string) {
	token := middleware.TokenFromContext(r.Context())

	var req apitypes.CreateTurnRequest
	if err := decodeJSONBody(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_json", err.Error())
		return
	}

	resp, err := s.threadLogic.CreateTurn(r.Context(), token, threadID, req.Message, req.ScopeType, req.ScopeDocIDs)
	if err != nil {
		writeGRPCError(w, err)
		return
	}

	citations := make([]map[string]interface{}, 0, len(resp.GetCitations()))
	for _, citation := range resp.GetCitations() {
		citations = append(citations, map[string]interface{}{
			"doc_id":      citation.GetDocId(),
			"doc_name":    citation.GetDocName(),
			"chunk_id":    citation.GetChunkId(),
			"excerpt":     citation.GetExcerpt(),
			"score":       citation.GetScore(),
			"chunk_index": citation.GetChunkIndex(),
		})
	}

	items := make([]map[string]interface{}, 0, len(resp.GetItems()))
	for _, item := range resp.GetItems() {
		items = append(items, protoTurnItemMap(item))
	}

	writeJSON(w, http.StatusCreated, map[string]interface{}{
		"turn":      protoTurnMap(resp.GetTurn()),
		"citations": citations,
		"items":     items,
	})
}

func (s *Server) handleTurnStream(w http.ResponseWriter, r *http.Request, threadID string, turnID string) {
	token := middleware.TokenFromContext(r.Context())
	resp, err := s.threadLogic.GetTurn(r.Context(), token, threadID, turnID)
	if err != nil {
		writeGRPCError(w, err)
		return
	}

	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")

	flusher, ok := w.(http.Flusher)
	if !ok {
		writeError(w, http.StatusInternalServerError, "stream_unsupported", "streaming unsupported")
		return
	}

	for _, item := range resp.GetItems() {
		event := map[string]interface{}{
			"id":         item.GetId(),
			"turn_id":    item.GetTurnId(),
			"item_type":  item.GetItemType(),
			"payload":    decodePayloadJSON(item.GetPayloadJson()),
			"created_at": item.GetCreatedAt(),
		}
		payload, _ := json.Marshal(event)
		_, _ = fmt.Fprintf(w, "event: %s\n", item.GetItemType())
		_, _ = fmt.Fprintf(w, "data: %s\n\n", payload)
		flusher.Flush()
	}
	_, _ = fmt.Fprint(w, "event: done\ndata: {}\n\n")
	flusher.Flush()
}
