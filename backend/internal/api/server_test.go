package api

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"log"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"sync"
	"testing"
	"time"

	authapp "llm-doc-qa-assistant/backend/internal/application/auth"
	"llm-doc-qa-assistant/backend/internal/infrastructure/memory"
	"llm-doc-qa-assistant/backend/internal/infrastructure/security"
	"llm-doc-qa-assistant/backend/internal/store"
	"llm-doc-qa-assistant/backend/internal/types"
)

func TestDocumentIsolationByOwner(t *testing.T) {
	s, authSvc, h := newTestServer(t)
	ctx := context.Background()

	u1, err := authSvc.Register(ctx, "a@example.com", "password-123")
	if err != nil {
		t.Fatalf("register user1: %v", err)
	}
	u2, err := authSvc.Register(ctx, "b@example.com", "password-123")
	if err != nil {
		t.Fatalf("register user2: %v", err)
	}
	_, session1, err := authSvc.Login(ctx, "a@example.com", "password-123")
	if err != nil {
		t.Fatalf("login user1: %v", err)
	}
	_, session2, err := authSvc.Login(ctx, "b@example.com", "password-123")
	if err != nil {
		t.Fatalf("login user2: %v", err)
	}

	doc1 := types.Document{ID: "doc_1", OwnerUserID: u1.ID, Name: "u1.md", Status: "ready", CreatedAt: time.Now(), LastUpdatedAt: time.Now()}
	doc2 := types.Document{ID: "doc_2", OwnerUserID: u2.ID, Name: "u2.md", Status: "ready", CreatedAt: time.Now(), LastUpdatedAt: time.Now()}
	if err := s.UpsertDocument(doc1, []types.Chunk{{ID: "c1", DocID: doc1.ID, Content: "hello"}}); err != nil {
		t.Fatalf("upsert doc1: %v", err)
	}
	if err := s.UpsertDocument(doc2, []types.Chunk{{ID: "c2", DocID: doc2.ID, Content: "world"}}); err != nil {
		t.Fatalf("upsert doc2: %v", err)
	}

	listReq := httptest.NewRequest(http.MethodGet, "/api/documents", nil)
	listReq.Header.Set("Authorization", "Bearer "+session1.Token)
	listResp := httptest.NewRecorder()
	h.ServeHTTP(listResp, listReq)
	if listResp.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", listResp.Code)
	}
	var listPayload map[string]interface{}
	if err := json.Unmarshal(listResp.Body.Bytes(), &listPayload); err != nil {
		t.Fatalf("invalid list payload: %v", err)
	}
	docs, ok := listPayload["documents"].([]interface{})
	if !ok || len(docs) != 1 {
		t.Fatalf("expected exactly one document for user1, got %+v", listPayload["documents"])
	}

	forbiddenReq := httptest.NewRequest(http.MethodGet, "/api/documents/doc_1", nil)
	forbiddenReq.Header.Set("Authorization", "Bearer "+session2.Token)
	forbiddenResp := httptest.NewRecorder()
	h.ServeHTTP(forbiddenResp, forbiddenReq)
	if forbiddenResp.Code != http.StatusForbidden {
		t.Fatalf("expected 403 for cross-user access, got %d", forbiddenResp.Code)
	}
}

func TestRegisterAndLoginFlow(t *testing.T) {
	_, _, h := newTestServer(t)

	registerBody := []byte(`{"email":"new@example.com","password":"password-123"}`)
	registerReq := httptest.NewRequest(http.MethodPost, "/api/auth/register", bytes.NewReader(registerBody))
	registerResp := httptest.NewRecorder()
	h.ServeHTTP(registerResp, registerReq)
	if registerResp.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d, body=%s", registerResp.Code, registerResp.Body.String())
	}

	loginBody := []byte(`{"email":"new@example.com","password":"password-123"}`)
	loginReq := httptest.NewRequest(http.MethodPost, "/api/auth/login", bytes.NewReader(loginBody))
	loginResp := httptest.NewRecorder()
	h.ServeHTTP(loginResp, loginReq)
	if loginResp.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d, body=%s", loginResp.Code, loginResp.Body.String())
	}

	var payload map[string]interface{}
	if err := json.Unmarshal(loginResp.Body.Bytes(), &payload); err != nil {
		t.Fatalf("bad json: %v", err)
	}
	if payload["token"] == "" {
		t.Fatalf("expected non-empty token")
	}
}

func TestUploadAndDownloadDocument(t *testing.T) {
	_, authSvc, h := newTestServer(t)
	ctx := context.Background()

	_, err := authSvc.Register(ctx, "doc@example.com", "password-123")
	if err != nil {
		t.Fatalf("register error: %v", err)
	}
	_, session, err := authSvc.Login(ctx, "doc@example.com", "password-123")
	if err != nil {
		t.Fatalf("login error: %v", err)
	}

	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)
	part, err := writer.CreateFormFile("file", "hello.txt")
	if err != nil {
		t.Fatalf("create form file: %v", err)
	}
	_, _ = part.Write([]byte("hello minio upload test"))
	_ = writer.Close()

	uploadReq := httptest.NewRequest(http.MethodPost, "/api/documents/upload", body)
	uploadReq.Header.Set("Content-Type", writer.FormDataContentType())
	uploadReq.Header.Set("Authorization", "Bearer "+session.Token)
	uploadResp := httptest.NewRecorder()
	h.ServeHTTP(uploadResp, uploadReq)
	if uploadResp.Code != http.StatusCreated {
		t.Fatalf("expected 201 upload, got %d body=%s", uploadResp.Code, uploadResp.Body.String())
	}

	var uploadPayload map[string]interface{}
	if err := json.Unmarshal(uploadResp.Body.Bytes(), &uploadPayload); err != nil {
		t.Fatalf("bad upload payload: %v", err)
	}
	docObj, ok := uploadPayload["document"].(map[string]interface{})
	if !ok {
		t.Fatalf("missing document in upload payload")
	}
	docID, ok := docObj["id"].(string)
	if !ok || docID == "" {
		t.Fatalf("missing document id in upload payload")
	}

	downloadReq := httptest.NewRequest(http.MethodGet, "/api/documents/"+docID+"/download", nil)
	downloadReq.Header.Set("Authorization", "Bearer "+session.Token)
	downloadResp := httptest.NewRecorder()
	h.ServeHTTP(downloadResp, downloadReq)
	if downloadResp.Code != http.StatusOK {
		t.Fatalf("expected 200 download, got %d body=%s", downloadResp.Code, downloadResp.Body.String())
	}
	if !bytes.Contains(downloadResp.Body.Bytes(), []byte("hello minio upload test")) {
		t.Fatalf("unexpected download body: %s", downloadResp.Body.String())
	}
}

func TestUploadRejectsUnsupportedExtension(t *testing.T) {
	_, authSvc, h := newTestServer(t)
	ctx := context.Background()

	_, err := authSvc.Register(ctx, "badtype@example.com", "password-123")
	if err != nil {
		t.Fatalf("register error: %v", err)
	}
	_, session, err := authSvc.Login(ctx, "badtype@example.com", "password-123")
	if err != nil {
		t.Fatalf("login error: %v", err)
	}

	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)
	part, err := writer.CreateFormFile("file", "evil.exe")
	if err != nil {
		t.Fatalf("create form file: %v", err)
	}
	_, _ = part.Write([]byte("fake exe"))
	_ = writer.Close()

	uploadReq := httptest.NewRequest(http.MethodPost, "/api/documents/upload", body)
	uploadReq.Header.Set("Content-Type", writer.FormDataContentType())
	uploadReq.Header.Set("Authorization", "Bearer "+session.Token)
	uploadResp := httptest.NewRecorder()
	h.ServeHTTP(uploadResp, uploadReq)
	if uploadResp.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 upload for unsupported extension, got %d body=%s", uploadResp.Code, uploadResp.Body.String())
	}
}

func newTestServer(t *testing.T) (*store.Store, *authapp.Service, http.Handler) {
	t.Helper()
	tmp := t.TempDir()
	s, err := store.New(filepath.Join(tmp, "state.json"), filepath.Join(tmp, "audit.log"))
	if err != nil {
		t.Fatalf("store init error: %v", err)
	}
	userRepo, sessionRepo := memory.NewAuthRepositories()
	authSvc := authapp.NewService(userRepo, sessionRepo, security.PasswordHasher{}, security.TokenGenerator{}, s)
	server := NewServer(s, authSvc, newFakeObjectStore(), log.New(io.Discard, "", 0))
	return s, authSvc, server.Routes()
}

type fakeObjectStore struct {
	mu    sync.RWMutex
	files map[string][]byte
}

func newFakeObjectStore() *fakeObjectStore {
	return &fakeObjectStore{
		files: map[string][]byte{},
	}
}

func (f *fakeObjectStore) PutObject(_ context.Context, key string, data []byte, _ string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	cp := make([]byte, len(data))
	copy(cp, data)
	f.files[key] = cp
	return nil
}

func (f *fakeObjectStore) GetObject(_ context.Context, key string) ([]byte, string, error) {
	f.mu.RLock()
	defer f.mu.RUnlock()
	data, ok := f.files[key]
	if !ok {
		return nil, "", errors.New("not found")
	}
	cp := make([]byte, len(data))
	copy(cp, data)
	return cp, "application/octet-stream", nil
}

func (f *fakeObjectStore) DeleteObject(_ context.Context, key string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	delete(f.files, key)
	return nil
}
