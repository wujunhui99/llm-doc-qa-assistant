package api

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"mime/multipart"
	"net/http"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"

	authapp "llm-doc-qa-assistant/backend/internal/application/auth"
	domainauth "llm-doc-qa-assistant/backend/internal/domain/auth"
	"llm-doc-qa-assistant/backend/internal/ingest"
	"llm-doc-qa-assistant/backend/internal/qa"
	"llm-doc-qa-assistant/backend/internal/store"
	"llm-doc-qa-assistant/backend/internal/types"
)

const (
	maxUploadSize = 10 << 20 // 10MB
)

type contextKey string

const userContextKey contextKey = "user"

type AuthUseCase interface {
	Register(ctx context.Context, email, password string) (domainauth.User, error)
	Login(ctx context.Context, email, password string) (domainauth.User, domainauth.Session, error)
	Logout(ctx context.Context, token, actorID string) error
	Authenticate(ctx context.Context, token string) (domainauth.User, error)
}

type DocumentObjectStore interface {
	PutObject(ctx context.Context, key string, data []byte, contentType string) error
	GetObject(ctx context.Context, key string) ([]byte, string, error)
	DeleteObject(ctx context.Context, key string) error
}

type Server struct {
	store       *store.Store
	authService AuthUseCase
	objectStore DocumentObjectStore
	logger      *log.Logger
}

func NewServer(s *store.Store, authService AuthUseCase, objectStore DocumentObjectStore, logger *log.Logger) *Server {
	if objectStore == nil {
		panic("objectStore cannot be nil")
	}
	return &Server{store: s, authService: authService, objectStore: objectStore, logger: logger}
}

func (s *Server) Routes() http.Handler {
	mux := http.NewServeMux()

	mux.HandleFunc("/", s.handleRoot)
	mux.HandleFunc("/api/health", s.handleHealth)

	mux.HandleFunc("/api/auth/register", s.withMethod(http.MethodPost, s.handleRegister))
	mux.HandleFunc("/api/auth/login", s.withMethod(http.MethodPost, s.handleLogin))
	mux.HandleFunc("/api/auth/logout", s.withAuth(s.withMethod(http.MethodPost, s.handleLogout)))
	mux.HandleFunc("/api/auth/me", s.withAuth(s.withMethod(http.MethodGet, s.handleMe)))

	mux.HandleFunc("/api/documents/upload", s.withAuth(s.withMethod(http.MethodPost, s.handleUploadDocument)))
	mux.HandleFunc("/api/documents", s.withAuth(s.handleDocumentsRoot))
	mux.HandleFunc("/api/documents/", s.withAuth(s.handleDocumentRoutes))

	mux.HandleFunc("/api/threads", s.withAuth(s.handleThreadsRoot))
	mux.HandleFunc("/api/threads/", s.withAuth(s.handleThreadsSubRoutes))

	mux.HandleFunc("/api/config", s.withAuth(s.handleConfig))
	mux.HandleFunc("/api/config/health", s.withMethod(http.MethodGet, s.handleConfigHealth))

	return s.withCORS(mux)
}

func (s *Server) handleRoot(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, map[string]interface{}{
		"name":    "Smart Document QA Assistant API",
		"version": "0.1.0",
	})
}

func (s *Server) handleHealth(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, map[string]interface{}{"status": "ok", "time": time.Now().UTC()})
}

func (s *Server) withMethod(method string, next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		if r.Method != method {
			writeError(w, http.StatusMethodNotAllowed, "method_not_allowed", "method not allowed")
			return
		}
		next(w, r)
	}
}

func (s *Server) withCORS(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		next.ServeHTTP(w, r)
	})
}

func (s *Server) withAuth(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		token := extractBearerToken(r.Header.Get("Authorization"))
		if token == "" {
			s.store.RecordAudit("auth.unauthorized", "", r.URL.Path, map[string]interface{}{"reason": "missing_token"})
			writeError(w, http.StatusUnauthorized, "unauthorized", "missing or invalid auth token")
			return
		}

		user, err := s.authService.Authenticate(r.Context(), token)
		if err != nil {
			if errors.Is(err, authapp.ErrUnauthorized) {
				s.store.RecordAudit("auth.unauthorized", "", r.URL.Path, map[string]interface{}{"reason": "expired_or_unknown_token"})
				writeError(w, http.StatusUnauthorized, "unauthorized", "session expired or invalid")
				return
			}
			writeError(w, http.StatusInternalServerError, "auth_error", "authentication failed")
			return
		}

		ctx := context.WithValue(r.Context(), userContextKey, user)
		next(w, r.WithContext(ctx))
	}
}

func getUserFromContext(ctx context.Context) (domainauth.User, bool) {
	user, ok := ctx.Value(userContextKey).(domainauth.User)
	return user, ok
}

type registerRequest struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

func (s *Server) handleRegister(w http.ResponseWriter, r *http.Request) {
	var req registerRequest
	if err := decodeJSONBody(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_json", err.Error())
		return
	}
	user, err := s.authService.Register(r.Context(), req.Email, req.Password)
	if err != nil {
		s.writeAuthServiceError(w, err)
		return
	}

	writeJSON(w, http.StatusCreated, map[string]interface{}{
		"user": sanitizeUser(user),
	})
}

type loginRequest struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

func (s *Server) handleLogin(w http.ResponseWriter, r *http.Request) {
	var req loginRequest
	if err := decodeJSONBody(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_json", err.Error())
		return
	}
	user, session, err := s.authService.Login(r.Context(), req.Email, req.Password)
	if err != nil {
		s.writeAuthServiceError(w, err)
		return
	}

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"token":      session.Token,
		"expires_at": session.ExpiresAt,
		"user":       sanitizeUser(user),
	})
}

func (s *Server) handleLogout(w http.ResponseWriter, r *http.Request) {
	user, _ := getUserFromContext(r.Context())
	token := extractBearerToken(r.Header.Get("Authorization"))
	if err := s.authService.Logout(r.Context(), token, user.ID); err != nil {
		if errors.Is(err, authapp.ErrUnauthorized) {
			writeError(w, http.StatusUnauthorized, "unauthorized", "missing or invalid token")
			return
		}
		writeError(w, http.StatusInternalServerError, "session_error", "failed to logout")
		return
	}
	writeJSON(w, http.StatusOK, map[string]interface{}{"ok": true})
}

func (s *Server) handleMe(w http.ResponseWriter, r *http.Request) {
	user, ok := getUserFromContext(r.Context())
	if !ok {
		writeError(w, http.StatusUnauthorized, "unauthorized", "missing user context")
		return
	}
	writeJSON(w, http.StatusOK, map[string]interface{}{"user": sanitizeUser(user)})
}

func (s *Server) handleDocumentsRoot(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		s.handleListDocuments(w, r)
	case http.MethodOptions:
		w.WriteHeader(http.StatusNoContent)
	default:
		writeError(w, http.StatusMethodNotAllowed, "method_not_allowed", "method not allowed")
	}
}

func (s *Server) handleListDocuments(w http.ResponseWriter, r *http.Request) {
	user, _ := getUserFromContext(r.Context())
	docs := s.store.ListDocumentsByOwner(user.ID)
	sort.Slice(docs, func(i, j int) bool {
		return docs[i].CreatedAt.After(docs[j].CreatedAt)
	})

	page := parseIntDefault(r.URL.Query().Get("page"), 1)
	pageSize := parseIntDefault(r.URL.Query().Get("page_size"), 50)
	if page < 1 {
		page = 1
	}
	if pageSize < 1 || pageSize > 100 {
		pageSize = 50
	}
	start := (page - 1) * pageSize
	if start > len(docs) {
		start = len(docs)
	}
	end := start + pageSize
	if end > len(docs) {
		end = len(docs)
	}

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"documents": docs[start:end],
		"total":     len(docs),
		"page":      page,
		"page_size": pageSize,
	})
}

func (s *Server) handleDocumentRoutes(w http.ResponseWriter, r *http.Request) {
	path := strings.Trim(strings.TrimPrefix(r.URL.Path, "/api/documents/"), "/")
	if path == "" {
		writeError(w, http.StatusBadRequest, "invalid_document", "document id is required")
		return
	}
	parts := strings.Split(path, "/")
	if len(parts) == 1 {
		s.handleDocumentByID(w, r, parts[0])
		return
	}
	if len(parts) == 2 && parts[1] == "download" && r.Method == http.MethodGet {
		s.handleDownloadDocument(w, r, parts[0])
		return
	}
	writeError(w, http.StatusNotFound, "not_found", "route not found")
}

func (s *Server) handleDocumentByID(w http.ResponseWriter, r *http.Request, docID string) {
	if docID == "" {
		writeError(w, http.StatusBadRequest, "invalid_document", "document id is required")
		return
	}
	user, _ := getUserFromContext(r.Context())
	doc, ok := s.store.GetDocument(docID)
	if !ok {
		writeError(w, http.StatusNotFound, "not_found", "document not found")
		return
	}
	if doc.OwnerUserID != user.ID {
		s.store.RecordAudit("auth.unauthorized", user.ID, docID, map[string]interface{}{"resource": "document", "operation": r.Method})
		writeError(w, http.StatusForbidden, "forbidden", "document does not belong to current user")
		return
	}

	switch r.Method {
	case http.MethodGet:
		writeJSON(w, http.StatusOK, map[string]interface{}{"document": doc})
	case http.MethodDelete:
		confirm := strings.EqualFold(r.URL.Query().Get("confirm"), "true") || r.URL.Query().Get("confirm") == "1"
		if !confirm {
			writeError(w, http.StatusBadRequest, "confirmation_required", "delete requires ?confirm=true")
			return
		}
		if err := s.objectStore.DeleteObject(r.Context(), doc.StoragePath); err != nil {
			writeError(w, http.StatusInternalServerError, "delete_failed", "failed to delete document file")
			return
		}
		if err := s.store.DeleteDocument(docID); err != nil {
			writeError(w, http.StatusInternalServerError, "delete_failed", "failed to delete document")
			return
		}
		s.store.RecordAudit("document.delete", user.ID, docID, map[string]interface{}{"name": doc.Name})
		writeJSON(w, http.StatusOK, map[string]interface{}{"ok": true})
	case http.MethodOptions:
		w.WriteHeader(http.StatusNoContent)
	default:
		writeError(w, http.StatusMethodNotAllowed, "method_not_allowed", "method not allowed")
	}
}

func (s *Server) handleDownloadDocument(w http.ResponseWriter, r *http.Request, docID string) {
	user, _ := getUserFromContext(r.Context())
	doc, ok := s.store.GetDocument(docID)
	if !ok {
		writeError(w, http.StatusNotFound, "not_found", "document not found")
		return
	}
	if doc.OwnerUserID != user.ID {
		s.store.RecordAudit("auth.unauthorized", user.ID, docID, map[string]interface{}{"resource": "document", "operation": "download"})
		writeError(w, http.StatusForbidden, "forbidden", "document does not belong to current user")
		return
	}

	data, contentType, err := s.objectStore.GetObject(r.Context(), doc.StoragePath)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "download_failed", "failed to download document")
		return
	}
	if contentType == "" {
		contentType = doc.MimeType
	}
	if contentType == "" {
		contentType = "application/octet-stream"
	}

	w.Header().Set("Content-Type", contentType)
	w.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=\"%s\"", sanitizeFileName(doc.Name)))
	w.Header().Set("Content-Length", strconv.Itoa(len(data)))
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(data)
}

func (s *Server) handleUploadDocument(w http.ResponseWriter, r *http.Request) {
	user, _ := getUserFromContext(r.Context())

	if err := r.ParseMultipartForm(maxUploadSize); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_multipart", "unable to parse upload")
		return
	}
	file, header, err := r.FormFile("file")
	if err != nil {
		writeError(w, http.StatusBadRequest, "file_required", "multipart field 'file' is required")
		return
	}
	defer file.Close()

	if err := validateUploadHeader(header); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_file", err.Error())
		return
	}

	data, err := io.ReadAll(io.LimitReader(file, maxUploadSize+1))
	if err != nil {
		writeError(w, http.StatusBadRequest, "read_failed", "failed to read file")
		return
	}
	if int64(len(data)) > maxUploadSize {
		writeError(w, http.StatusBadRequest, "file_too_large", "file exceeds 10MB limit")
		return
	}

	docID := types.NewID("doc")
	ext := strings.ToLower(filepath.Ext(header.Filename))
	if ext == "" {
		ext = ".txt"
	}
	objectKey := user.ID + "/" + docID + ext
	if err := s.objectStore.PutObject(r.Context(), objectKey, data, inferContentType(header.Filename, header.Header.Get("Content-Type"))); err != nil {
		writeError(w, http.StatusInternalServerError, "storage_error", "failed to save file")
		return
	}

	now := time.Now().UTC()
	doc := types.Document{
		ID:            docID,
		OwnerUserID:   user.ID,
		Name:          header.Filename,
		SizeBytes:     int64(len(data)),
		MimeType:      header.Header.Get("Content-Type"),
		StoragePath:   objectKey,
		Status:        "indexing",
		ChunkCount:    0,
		CreatedAt:     now,
		LastUpdatedAt: now,
	}
	if err := s.store.UpsertDocument(doc, nil); err != nil {
		writeError(w, http.StatusInternalServerError, "persist_error", "failed to persist document")
		return
	}

	text, err := ingest.ParseDocumentText(header.Filename, doc.MimeType, data)
	if err != nil {
		doc.Status = "failed"
		doc.LastUpdatedAt = time.Now().UTC()
		_ = s.store.UpsertDocument(doc, nil)
		writeError(w, http.StatusBadRequest, "parse_failed", err.Error())
		return
	}
	pieces := ingest.ChunkText(text, 700, 120)
	chunks := make([]types.Chunk, 0, len(pieces))
	for i, content := range pieces {
		chunks = append(chunks, types.Chunk{
			ID:      types.NewID("chk"),
			DocID:   doc.ID,
			Index:   i,
			Content: content,
		})
	}
	doc.Status = "ready"
	doc.ChunkCount = len(chunks)
	doc.LastUpdatedAt = time.Now().UTC()
	if err := s.store.UpsertDocument(doc, chunks); err != nil {
		writeError(w, http.StatusInternalServerError, "persist_error", "failed to save index")
		return
	}

	writeJSON(w, http.StatusCreated, map[string]interface{}{"document": doc})
}

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
	user, _ := getUserFromContext(r.Context())
	threads := s.store.ListThreadsByOwner(user.ID)
	sort.Slice(threads, func(i, j int) bool {
		return threads[i].CreatedAt.After(threads[j].CreatedAt)
	})
	writeJSON(w, http.StatusOK, map[string]interface{}{"threads": threads})
}

type createThreadRequest struct {
	Title string `json:"title"`
}

func (s *Server) handleCreateThread(w http.ResponseWriter, r *http.Request) {
	user, _ := getUserFromContext(r.Context())
	var req createThreadRequest
	_ = decodeJSONBody(r, &req)
	title := strings.TrimSpace(req.Title)
	if title == "" {
		title = "Untitled Thread"
	}
	thread := types.Thread{
		ID:          types.NewID("th"),
		OwnerUserID: user.ID,
		Title:       title,
		CreatedAt:   time.Now().UTC(),
	}
	if err := s.store.CreateThread(thread); err != nil {
		writeError(w, http.StatusInternalServerError, "thread_error", "failed to create thread")
		return
	}
	writeJSON(w, http.StatusCreated, map[string]interface{}{"thread": thread})
}

func (s *Server) handleThreadsSubRoutes(w http.ResponseWriter, r *http.Request) {
	path := strings.TrimPrefix(r.URL.Path, "/api/threads/")
	parts := strings.Split(path, "/")
	if len(parts) < 2 {
		writeError(w, http.StatusNotFound, "not_found", "route not found")
		return
	}
	threadID := parts[0]

	thread, ok := s.store.GetThread(threadID)
	if !ok {
		writeError(w, http.StatusNotFound, "not_found", "thread not found")
		return
	}
	user, _ := getUserFromContext(r.Context())
	if thread.OwnerUserID != user.ID {
		s.store.RecordAudit("auth.unauthorized", user.ID, threadID, map[string]interface{}{"resource": "thread"})
		writeError(w, http.StatusForbidden, "forbidden", "thread does not belong to current user")
		return
	}

	if len(parts) == 2 && parts[1] == "turns" && r.Method == http.MethodPost {
		s.handleCreateTurn(w, r, thread)
		return
	}
	if len(parts) == 4 && parts[1] == "turns" && parts[3] == "stream" && r.Method == http.MethodGet {
		s.handleTurnStream(w, r, thread, parts[2])
		return
	}

	writeError(w, http.StatusNotFound, "not_found", "route not found")
}

type createTurnRequest struct {
	Message     string   `json:"message"`
	ScopeType   string   `json:"scope_type"`
	ScopeDocIDs []string `json:"scope_doc_ids"`
}

func (s *Server) handleCreateTurn(w http.ResponseWriter, r *http.Request, thread types.Thread) {
	user, _ := getUserFromContext(r.Context())

	var req createTurnRequest
	if err := decodeJSONBody(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_json", err.Error())
		return
	}
	if strings.TrimSpace(req.Message) == "" {
		writeError(w, http.StatusBadRequest, "invalid_message", "message is required")
		return
	}

	ownedDocs := s.store.ListDocumentsByOwner(user.ID)
	scope, err := qa.ResolveScope(req.Message, strings.ToLower(strings.TrimSpace(req.ScopeType)), req.ScopeDocIDs, ownedDocs)
	if err != nil {
		writeError(w, http.StatusBadRequest, "scope_error", err.Error())
		return
	}

	selectedDocs := resolveSelectedDocs(scope, ownedDocs)
	if scope.Type == "doc" && len(selectedDocs) == 0 {
		writeError(w, http.StatusBadRequest, "scope_error", "selected documents do not exist or are not owned by current user")
		return
	}

	chunkMap := make(map[string][]types.Chunk, len(selectedDocs))
	for _, doc := range selectedDocs {
		chunkMap[doc.ID] = s.store.GetChunksForDoc(doc.ID)
	}
	retrieved := qa.RetrieveTopChunks(scope.QuestionBody, selectedDocs, chunkMap, 5)
	citations := buildCitations(retrieved)

	previousTurns := s.store.ListTurnsByThread(thread.ID)
	sort.Slice(previousTurns, func(i, j int) bool {
		return previousTurns[i].CreatedAt.Before(previousTurns[j].CreatedAt)
	})

	answer := buildAnswer(scope.QuestionBody, retrieved, previousTurns)
	now := time.Now().UTC()
	turn := types.Turn{
		ID:          types.NewID("turn"),
		ThreadID:    thread.ID,
		OwnerUserID: user.ID,
		Question:    scope.QuestionBody,
		Answer:      answer,
		Status:      "completed",
		ScopeType:   scope.Type,
		ScopeDocIDs: scope.DocIDs,
		CreatedAt:   now,
		UpdatedAt:   now,
	}
	items := []types.TurnItem{
		{
			ID:       types.NewID("item"),
			TurnID:   turn.ID,
			ItemType: "message",
			Payload:  map[string]interface{}{"question": turn.Question, "scope_type": turn.ScopeType, "scope_doc_ids": turn.ScopeDocIDs},
		},
		{
			ID:       types.NewID("item"),
			TurnID:   turn.ID,
			ItemType: "retrieval",
			Payload:  map[string]interface{}{"count": len(citations), "citations": citations},
		},
		{
			ID:       types.NewID("item"),
			TurnID:   turn.ID,
			ItemType: "final",
			Payload:  map[string]interface{}{"answer": answer, "citations": citations},
		},
	}
	for i := range items {
		items[i].CreatedAt = now.Add(time.Duration(i) * time.Millisecond)
	}
	if err := s.store.CreateOrUpdateTurn(turn, items); err != nil {
		writeError(w, http.StatusInternalServerError, "turn_error", "failed to save turn")
		return
	}

	writeJSON(w, http.StatusCreated, map[string]interface{}{
		"turn":      turn,
		"citations": citations,
	})
}

func (s *Server) handleTurnStream(w http.ResponseWriter, r *http.Request, thread types.Thread, turnID string) {
	turn, ok := s.store.GetTurn(turnID)
	if !ok || turn.ThreadID != thread.ID {
		writeError(w, http.StatusNotFound, "not_found", "turn not found")
		return
	}
	items := s.store.GetTurnItems(turnID)

	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")

	flusher, ok := w.(http.Flusher)
	if !ok {
		writeError(w, http.StatusInternalServerError, "stream_unsupported", "streaming unsupported")
		return
	}
	for _, item := range items {
		payload, _ := json.Marshal(item)
		_, _ = fmt.Fprintf(w, "event: %s\n", item.ItemType)
		_, _ = fmt.Fprintf(w, "data: %s\n\n", payload)
		flusher.Flush()
	}
	_, _ = fmt.Fprint(w, "event: done\ndata: {}\n\n")
	flusher.Flush()
}

func (s *Server) handleConfig(w http.ResponseWriter, r *http.Request) {
	user, _ := getUserFromContext(r.Context())
	switch r.Method {
	case http.MethodGet:
		writeJSON(w, http.StatusOK, s.store.GetProvider())
	case http.MethodPut:
		var req struct {
			ActiveProvider string `json:"active_provider"`
		}
		if err := decodeJSONBody(r, &req); err != nil {
			writeError(w, http.StatusBadRequest, "invalid_json", err.Error())
			return
		}
		if err := s.store.SetProvider(strings.TrimSpace(req.ActiveProvider), user.ID); err != nil {
			writeError(w, http.StatusBadRequest, "provider_error", err.Error())
			return
		}
		writeJSON(w, http.StatusOK, s.store.GetProvider())
	case http.MethodOptions:
		w.WriteHeader(http.StatusNoContent)
	default:
		writeError(w, http.StatusMethodNotAllowed, "method_not_allowed", "method not allowed")
	}
}

func (s *Server) handleConfigHealth(w http.ResponseWriter, _ *http.Request) {
	provider := s.store.GetProvider()
	writeJSON(w, http.StatusOK, map[string]interface{}{
		"status":          "ok",
		"active_provider": provider.ActiveProvider,
	})
}

func sanitizeUser(user domainauth.User) map[string]interface{} {
	return map[string]interface{}{
		"id":         user.ID,
		"email":      user.Email,
		"created_at": user.CreatedAt,
	}
}

func (s *Server) writeAuthServiceError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, authapp.ErrInvalidEmail):
		writeError(w, http.StatusBadRequest, "invalid_email", "email is required")
	case errors.Is(err, authapp.ErrInvalidPassword):
		writeError(w, http.StatusBadRequest, "invalid_password", "password must satisfy minimum policy")
	case errors.Is(err, authapp.ErrEmailAlreadyExists):
		writeError(w, http.StatusConflict, "email_exists", "email already registered")
	case errors.Is(err, authapp.ErrInvalidCredentials):
		writeError(w, http.StatusUnauthorized, "invalid_credentials", "email or password is incorrect")
	case errors.Is(err, authapp.ErrUnauthorized):
		writeError(w, http.StatusUnauthorized, "unauthorized", "unauthorized")
	default:
		writeError(w, http.StatusInternalServerError, "internal_error", "internal server error")
	}
}

func extractBearerToken(authHeader string) string {
	authHeader = strings.TrimSpace(authHeader)
	if authHeader == "" {
		return ""
	}
	parts := strings.SplitN(authHeader, " ", 2)
	if len(parts) != 2 || !strings.EqualFold(parts[0], "Bearer") {
		return ""
	}
	return strings.TrimSpace(parts[1])
}

func writeJSON(w http.ResponseWriter, status int, payload interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(payload)
}

func writeError(w http.ResponseWriter, status int, code, message string) {
	writeJSON(w, status, map[string]interface{}{
		"error": map[string]interface{}{
			"code":    code,
			"message": message,
		},
	})
}

func decodeJSONBody(r *http.Request, out interface{}) error {
	if r.Body == nil {
		return errors.New("request body is required")
	}
	dec := json.NewDecoder(io.LimitReader(r.Body, 1<<20))
	dec.DisallowUnknownFields()
	if err := dec.Decode(out); err != nil {
		return err
	}
	return nil
}

func validateUploadHeader(header *multipart.FileHeader) error {
	if header == nil {
		return errors.New("missing file header")
	}
	ext := strings.ToLower(filepath.Ext(header.Filename))
	switch ext {
	case ".txt", ".md", ".markdown", ".pdf":
		return nil
	default:
		return errors.New("only TXT, Markdown, and PDF are supported")
	}
}

func parseIntDefault(raw string, def int) int {
	if raw == "" {
		return def
	}
	v, err := strconv.Atoi(raw)
	if err != nil {
		return def
	}
	return v
}

func resolveSelectedDocs(scope qa.Scope, owned []types.Document) []types.Document {
	if scope.Type == "all" {
		return owned
	}
	if len(scope.DocIDs) == 0 {
		return nil
	}
	allowed := make(map[string]struct{}, len(scope.DocIDs))
	for _, id := range scope.DocIDs {
		allowed[id] = struct{}{}
	}
	selected := make([]types.Document, 0, len(scope.DocIDs))
	for _, doc := range owned {
		if _, ok := allowed[doc.ID]; ok {
			selected = append(selected, doc)
		}
	}
	return selected
}

func buildCitations(scored []qa.ScoredChunk) []types.Citation {
	out := make([]types.Citation, 0, len(scored))
	for _, item := range scored {
		excerpt := item.Chunk.Content
		if len([]rune(excerpt)) > 220 {
			excerpt = string([]rune(excerpt)[:220]) + "..."
		}
		out = append(out, types.Citation{
			DocID:    item.Document.ID,
			DocName:  item.Document.Name,
			ChunkID:  item.Chunk.ID,
			Excerpt:  excerpt,
			Score:    item.Score,
			ChunkIdx: item.Chunk.Index,
		})
	}
	return out
}

func buildAnswer(question string, retrieved []qa.ScoredChunk, previousTurns []types.Turn) string {
	if len(retrieved) == 0 {
		if len(previousTurns) > 0 {
			last := previousTurns[len(previousTurns)-1]
			return "未在当前作用域中检索到足够证据。请尝试切换到 @all 或补充更具体的问题。上一轮结论：" + truncate(last.Answer, 120)
		}
		return "未在当前作用域中检索到足够证据。请尝试切换到 @all 或指定 @doc(文档名)。"
	}

	var b strings.Builder
	if len(previousTurns) > 0 {
		last := previousTurns[len(previousTurns)-1]
		b.WriteString("延续上一轮上下文（")
		b.WriteString(truncate(last.Question, 40))
		b.WriteString("），")
	}
	b.WriteString("针对你的问题“")
	b.WriteString(question)
	b.WriteString("”，根据检索到的文档证据：")
	for i, item := range retrieved {
		if i >= 2 {
			break
		}
		b.WriteString("\n")
		b.WriteString(fmt.Sprintf("- [%s] %s", item.Document.Name, truncate(item.Chunk.Content, 180)))
	}
	b.WriteString("\n建议你结合以上引用继续追问细节。")
	return b.String()
}

func truncate(s string, n int) string {
	r := []rune(strings.TrimSpace(s))
	if len(r) <= n {
		return string(r)
	}
	return string(r[:n]) + "..."
}

func inferContentType(fileName, fallback string) string {
	if strings.TrimSpace(fallback) != "" {
		return fallback
	}
	switch strings.ToLower(filepath.Ext(fileName)) {
	case ".txt":
		return "text/plain; charset=utf-8"
	case ".md", ".markdown":
		return "text/markdown; charset=utf-8"
	case ".pdf":
		return "application/pdf"
	default:
		return "application/octet-stream"
	}
}

func sanitizeFileName(name string) string {
	name = strings.ReplaceAll(name, "\\", "_")
	name = strings.ReplaceAll(name, "/", "_")
	name = strings.ReplaceAll(name, "\"", "_")
	name = strings.TrimSpace(name)
	if name == "" {
		return "document.bin"
	}
	return name
}
