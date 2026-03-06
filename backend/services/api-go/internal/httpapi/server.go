package httpapi

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"mime/multipart"
	"net/http"
	"sort"
	"strconv"
	"strings"
	"time"

	qav1 "llm-doc-qa-assistant/backend/proto/gen/go/qa/v1"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

const maxUploadSize = 10 << 20

type contextKey string

const tokenContextKey contextKey = "token"

type Server struct {
	core   qav1.CoreServiceClient
	logger *log.Logger
}

func NewServer(core qav1.CoreServiceClient, logger *log.Logger) *Server {
	if core == nil {
		panic("core client cannot be nil")
	}
	if logger == nil {
		logger = log.New(log.Writer(), "[api-go] ", log.LstdFlags|log.LUTC)
	}
	return &Server{core: core, logger: logger}
}

func (s *Server) Routes() http.Handler {
	mux := http.NewServeMux()

	mux.HandleFunc("/", s.handleRoot)
	mux.HandleFunc("/api/health", s.withMethod(http.MethodGet, s.handleHealth))

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
		"version": "0.2.0",
	})
}

func (s *Server) handleHealth(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), 3*time.Second)
	defer cancel()

	resp, err := s.core.Health(ctx, &qav1.Empty{})
	if err != nil {
		writeGRPCError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]interface{}{"status": resp.GetStatus(), "time": resp.GetTime()})
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
			writeError(w, http.StatusUnauthorized, "unauthorized", "missing or invalid auth token")
			return
		}
		ctx := context.WithValue(r.Context(), tokenContextKey, token)
		next(w, r.WithContext(ctx))
	}
}

func tokenFromContext(ctx context.Context) string {
	token, _ := ctx.Value(tokenContextKey).(string)
	return token
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

	ctx, cancel := context.WithTimeout(r.Context(), 8*time.Second)
	defer cancel()

	resp, err := s.core.Register(ctx, &qav1.RegisterRequest{Email: req.Email, Password: req.Password})
	if err != nil {
		writeGRPCError(w, err)
		return
	}

	writeJSON(w, http.StatusCreated, map[string]interface{}{"user": protoUserMap(resp.GetUser())})
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

	ctx, cancel := context.WithTimeout(r.Context(), 8*time.Second)
	defer cancel()

	resp, err := s.core.Login(ctx, &qav1.LoginRequest{Email: req.Email, Password: req.Password})
	if err != nil {
		writeGRPCError(w, err)
		return
	}

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"token":      resp.GetToken(),
		"expires_at": resp.GetExpiresAt(),
		"user":       protoUserMap(resp.GetUser()),
	})
}

func (s *Server) handleLogout(w http.ResponseWriter, r *http.Request) {
	token := tokenFromContext(r.Context())
	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()

	_, err := s.core.Logout(ctx, &qav1.LogoutRequest{Token: token})
	if err != nil {
		writeGRPCError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]interface{}{"ok": true})
}

func (s *Server) handleMe(w http.ResponseWriter, r *http.Request) {
	token := tokenFromContext(r.Context())
	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()

	resp, err := s.core.Me(ctx, &qav1.MeRequest{Token: token})
	if err != nil {
		writeGRPCError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]interface{}{"user": protoUserMap(resp.GetUser())})
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
	token := tokenFromContext(r.Context())
	page := parseIntDefault(r.URL.Query().Get("page"), 1)
	pageSize := parseIntDefault(r.URL.Query().Get("page_size"), 50)

	ctx, cancel := context.WithTimeout(r.Context(), 6*time.Second)
	defer cancel()

	resp, err := s.core.ListDocuments(ctx, &qav1.ListDocumentsRequest{Token: token, Page: int32(page), PageSize: int32(pageSize)})
	if err != nil {
		writeGRPCError(w, err)
		return
	}

	documents := make([]map[string]interface{}, 0, len(resp.GetDocuments()))
	for _, doc := range resp.GetDocuments() {
		documents = append(documents, protoDocumentMap(doc))
	}

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"documents": documents,
		"total":     resp.GetTotal(),
		"page":      resp.GetPage(),
		"page_size": resp.GetPageSize(),
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
	docID = strings.TrimSpace(docID)
	if docID == "" {
		writeError(w, http.StatusBadRequest, "invalid_document", "document id is required")
		return
	}

	token := tokenFromContext(r.Context())
	ctx, cancel := context.WithTimeout(r.Context(), 6*time.Second)
	defer cancel()

	switch r.Method {
	case http.MethodGet:
		resp, err := s.core.GetDocument(ctx, &qav1.DocumentRequest{Token: token, DocumentId: docID})
		if err != nil {
			writeGRPCError(w, err)
			return
		}
		writeJSON(w, http.StatusOK, map[string]interface{}{"document": protoDocumentMap(resp.GetDocument())})
	case http.MethodDelete:
		confirm := strings.EqualFold(r.URL.Query().Get("confirm"), "true") || r.URL.Query().Get("confirm") == "1"
		_, err := s.core.DeleteDocument(ctx, &qav1.DeleteDocumentRequest{Token: token, DocumentId: docID, Confirm: confirm})
		if err != nil {
			writeGRPCError(w, err)
			return
		}
		writeJSON(w, http.StatusOK, map[string]interface{}{"ok": true})
	case http.MethodOptions:
		w.WriteHeader(http.StatusNoContent)
	default:
		writeError(w, http.StatusMethodNotAllowed, "method_not_allowed", "method not allowed")
	}
}

func (s *Server) handleDownloadDocument(w http.ResponseWriter, r *http.Request, docID string) {
	token := tokenFromContext(r.Context())
	ctx, cancel := context.WithTimeout(r.Context(), 20*time.Second)
	defer cancel()

	resp, err := s.core.DownloadDocument(ctx, &qav1.DocumentRequest{Token: token, DocumentId: docID})
	if err != nil {
		writeGRPCError(w, err)
		return
	}

	contentType := strings.TrimSpace(resp.GetContentType())
	if contentType == "" {
		contentType = "application/octet-stream"
	}
	filename := sanitizeFileName(resp.GetFilename())
	if filename == "" {
		filename = "document.bin"
	}

	w.Header().Set("Content-Type", contentType)
	w.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=\"%s\"", filename))
	w.Header().Set("Content-Length", strconv.Itoa(len(resp.GetContent())))
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(resp.GetContent())
}

func (s *Server) handleUploadDocument(w http.ResponseWriter, r *http.Request) {
	token := tokenFromContext(r.Context())

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
	if len(data) > maxUploadSize {
		writeError(w, http.StatusBadRequest, "file_too_large", "file exceeds 10MB limit")
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), 45*time.Second)
	defer cancel()

	resp, err := s.core.UploadDocument(ctx, &qav1.UploadDocumentRequest{
		Token:    token,
		Filename: header.Filename,
		MimeType: header.Header.Get("Content-Type"),
		Content:  data,
	})
	if err != nil {
		writeGRPCError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, map[string]interface{}{"document": protoDocumentMap(resp.GetDocument())})
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
	token := tokenFromContext(r.Context())
	ctx, cancel := context.WithTimeout(r.Context(), 6*time.Second)
	defer cancel()

	resp, err := s.core.ListThreads(ctx, &qav1.ListThreadsRequest{Token: token})
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

type createThreadRequest struct {
	Title string `json:"title"`
}

func (s *Server) handleCreateThread(w http.ResponseWriter, r *http.Request) {
	token := tokenFromContext(r.Context())
	var req createThreadRequest
	_ = decodeJSONBody(r, &req)

	ctx, cancel := context.WithTimeout(r.Context(), 8*time.Second)
	defer cancel()

	resp, err := s.core.CreateThread(ctx, &qav1.CreateThreadRequest{Token: token, Title: req.Title})
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

type createTurnRequest struct {
	Message     string   `json:"message"`
	ScopeType   string   `json:"scope_type"`
	ScopeDocIDs []string `json:"scope_doc_ids"`
}

func (s *Server) handleCreateTurn(w http.ResponseWriter, r *http.Request, threadID string) {
	token := tokenFromContext(r.Context())

	var req createTurnRequest
	if err := decodeJSONBody(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_json", err.Error())
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), 40*time.Second)
	defer cancel()

	resp, err := s.core.CreateTurn(ctx, &qav1.CreateTurnRequest{
		Token:       token,
		ThreadId:    threadID,
		Message:     req.Message,
		ScopeType:   req.ScopeType,
		ScopeDocIds: req.ScopeDocIDs,
	})
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
	token := tokenFromContext(r.Context())
	ctx, cancel := context.WithTimeout(r.Context(), 20*time.Second)
	defer cancel()

	resp, err := s.core.GetTurn(ctx, &qav1.GetTurnRequest{Token: token, ThreadId: threadID, TurnId: turnID})
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

func (s *Server) handleConfig(w http.ResponseWriter, r *http.Request) {
	token := tokenFromContext(r.Context())
	ctx, cancel := context.WithTimeout(r.Context(), 8*time.Second)
	defer cancel()

	switch r.Method {
	case http.MethodGet:
		resp, err := s.core.GetConfig(ctx, &qav1.MeRequest{Token: token})
		if err != nil {
			writeGRPCError(w, err)
			return
		}
		writeJSON(w, http.StatusOK, map[string]interface{}{
			"active_provider": resp.GetActiveProvider(),
			"available":       resp.GetAvailable(),
		})
	case http.MethodPut:
		var req struct {
			ActiveProvider string `json:"active_provider"`
		}
		if err := decodeJSONBody(r, &req); err != nil {
			writeError(w, http.StatusBadRequest, "invalid_json", err.Error())
			return
		}
		resp, err := s.core.SetConfig(ctx, &qav1.SetConfigRequest{Token: token, ActiveProvider: req.ActiveProvider})
		if err != nil {
			writeGRPCError(w, err)
			return
		}
		writeJSON(w, http.StatusOK, map[string]interface{}{
			"active_provider": resp.GetActiveProvider(),
			"available":       resp.GetAvailable(),
		})
	case http.MethodOptions:
		w.WriteHeader(http.StatusNoContent)
	default:
		writeError(w, http.StatusMethodNotAllowed, "method_not_allowed", "method not allowed")
	}
}

func (s *Server) handleConfigHealth(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), 3*time.Second)
	defer cancel()

	resp, err := s.core.Health(ctx, &qav1.Empty{})
	if err != nil {
		writeGRPCError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]interface{}{"status": resp.GetStatus(), "time": resp.GetTime()})
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

func writeGRPCError(w http.ResponseWriter, err error) {
	st, ok := status.FromError(err)
	if !ok {
		writeError(w, http.StatusInternalServerError, "internal_error", "internal server error")
		return
	}

	httpCode := http.StatusInternalServerError
	errorCode := "internal_error"
	switch st.Code() {
	case codes.InvalidArgument:
		httpCode = http.StatusBadRequest
		errorCode = "invalid_request"
	case codes.Unauthenticated:
		httpCode = http.StatusUnauthorized
		errorCode = "unauthorized"
	case codes.PermissionDenied:
		httpCode = http.StatusForbidden
		errorCode = "forbidden"
	case codes.NotFound:
		httpCode = http.StatusNotFound
		errorCode = "not_found"
	case codes.AlreadyExists:
		httpCode = http.StatusConflict
		errorCode = "conflict"
	case codes.DeadlineExceeded:
		httpCode = http.StatusGatewayTimeout
		errorCode = "timeout"
	case codes.Unavailable:
		httpCode = http.StatusServiceUnavailable
		errorCode = "service_unavailable"
	}

	writeError(w, httpCode, errorCode, st.Message())
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

func validateUploadHeader(header *multipart.FileHeader) error {
	if header == nil {
		return errors.New("missing file header")
	}
	ext := strings.ToLower(filepathExt(header.Filename))
	switch ext {
	case ".txt", ".md", ".markdown", ".pdf":
		return nil
	default:
		return errors.New("only TXT, Markdown, and PDF are supported")
	}
}

func filepathExt(name string) string {
	idx := strings.LastIndex(name, ".")
	if idx <= 0 || idx == len(name)-1 {
		if idx == len(name)-1 {
			return "."
		}
		return ""
	}
	return name[idx:]
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

func decodePayloadJSON(payloadJSON string) interface{} {
	if strings.TrimSpace(payloadJSON) == "" {
		return map[string]interface{}{}
	}
	var out interface{}
	if err := json.Unmarshal([]byte(payloadJSON), &out); err != nil {
		return map[string]interface{}{"raw": payloadJSON}
	}
	return out
}

func protoUserMap(user *qav1.User) map[string]interface{} {
	if user == nil {
		return map[string]interface{}{}
	}
	return map[string]interface{}{
		"id":         user.GetId(),
		"email":      user.GetEmail(),
		"created_at": user.GetCreatedAt(),
	}
}

func protoDocumentMap(doc *qav1.Document) map[string]interface{} {
	if doc == nil {
		return map[string]interface{}{}
	}
	return map[string]interface{}{
		"id":              doc.GetId(),
		"owner_user_id":   doc.GetOwnerUserId(),
		"name":            doc.GetName(),
		"size_bytes":      doc.GetSizeBytes(),
		"mime_type":       doc.GetMimeType(),
		"storage_path":    doc.GetStoragePath(),
		"status":          doc.GetStatus(),
		"chunk_count":     doc.GetChunkCount(),
		"created_at":      doc.GetCreatedAt(),
		"last_updated_at": doc.GetLastUpdatedAt(),
	}
}

func protoThreadMap(thread *qav1.Thread) map[string]interface{} {
	if thread == nil {
		return map[string]interface{}{}
	}
	return map[string]interface{}{
		"id":            thread.GetId(),
		"owner_user_id": thread.GetOwnerUserId(),
		"title":         thread.GetTitle(),
		"created_at":    thread.GetCreatedAt(),
	}
}

func protoTurnMap(turn *qav1.Turn) map[string]interface{} {
	if turn == nil {
		return map[string]interface{}{}
	}
	return map[string]interface{}{
		"id":            turn.GetId(),
		"thread_id":     turn.GetThreadId(),
		"owner_user_id": turn.GetOwnerUserId(),
		"question":      turn.GetQuestion(),
		"answer":        turn.GetAnswer(),
		"status":        turn.GetStatus(),
		"scope_type":    turn.GetScopeType(),
		"scope_doc_ids": turn.GetScopeDocIds(),
		"created_at":    turn.GetCreatedAt(),
		"updated_at":    turn.GetUpdatedAt(),
	}
}

func protoTurnItemMap(item *qav1.TurnItem) map[string]interface{} {
	if item == nil {
		return map[string]interface{}{}
	}
	return map[string]interface{}{
		"id":         item.GetId(),
		"turn_id":    item.GetTurnId(),
		"item_type":  item.GetItemType(),
		"payload":    decodePayloadJSON(item.GetPayloadJson()),
		"created_at": item.GetCreatedAt(),
	}
}
