package handler

import (
	"encoding/json"
	"errors"
	"io"
	"mime/multipart"
	"net/http"
	"strconv"
	"strings"

	qav1 "llm-doc-qa-assistant/backend/proto/gen/go/qa/v1"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

const maxUploadSize = 10 << 20

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
	case codes.FailedPrecondition:
		httpCode = http.StatusPreconditionFailed
		errorCode = "failed_precondition"
	case codes.DeadlineExceeded:
		httpCode = http.StatusGatewayTimeout
		errorCode = "timeout"
	case codes.Unavailable:
		httpCode = http.StatusServiceUnavailable
		errorCode = "service_unavailable"
	}

	writeError(w, httpCode, errorCode, st.Message())
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
