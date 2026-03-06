package handler

import (
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"

	"llm-doc-qa-assistant/backend/apps/api-go/internal/middleware"
)

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
	token := middleware.TokenFromContext(r.Context())
	page := parseIntDefault(r.URL.Query().Get("page"), 1)
	pageSize := parseIntDefault(r.URL.Query().Get("page_size"), 50)

	resp, err := s.docLogic.ListDocuments(r.Context(), token, int32(page), int32(pageSize))
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

	token := middleware.TokenFromContext(r.Context())
	switch r.Method {
	case http.MethodGet:
		resp, err := s.docLogic.GetDocument(r.Context(), token, docID)
		if err != nil {
			writeGRPCError(w, err)
			return
		}
		writeJSON(w, http.StatusOK, map[string]interface{}{"document": protoDocumentMap(resp.GetDocument())})
	case http.MethodDelete:
		confirm := strings.EqualFold(r.URL.Query().Get("confirm"), "true") || r.URL.Query().Get("confirm") == "1"
		if err := s.docLogic.DeleteDocument(r.Context(), token, docID, confirm); err != nil {
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
	token := middleware.TokenFromContext(r.Context())
	resp, err := s.docLogic.DownloadDocument(r.Context(), token, docID)
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
	token := middleware.TokenFromContext(r.Context())

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

	resp, err := s.docLogic.UploadDocument(r.Context(), token, header.Filename, header.Header.Get("Content-Type"), data)
	if err != nil {
		writeGRPCError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, map[string]interface{}{"document": protoDocumentMap(resp.GetDocument())})
}
