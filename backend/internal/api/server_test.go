package api

import (
	"bytes"
	"encoding/json"
	"io"
	"log"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"
	"time"

	"llm-doc-qa-assistant/backend/internal/auth"
	"llm-doc-qa-assistant/backend/internal/store"
	"llm-doc-qa-assistant/backend/internal/types"
)

func TestDocumentIsolationByOwner(t *testing.T) {
	tmp := t.TempDir()
	s, err := store.New(filepath.Join(tmp, "state.json"), filepath.Join(tmp, "audit.log"))
	if err != nil {
		t.Fatalf("store init error: %v", err)
	}
	server := NewServer(s, filepath.Join(tmp, "files"), log.New(io.Discard, "", 0))
	h := server.Routes()

	u1 := mustCreateUser(t, s, "a@example.com", "password-123")
	u2 := mustCreateUser(t, s, "b@example.com", "password-123")
	token1 := mustCreateSession(t, s, u1.ID)
	token2 := mustCreateSession(t, s, u2.ID)

	doc1 := types.Document{ID: "doc_1", OwnerUserID: u1.ID, Name: "u1.md", Status: "ready", CreatedAt: time.Now(), LastUpdatedAt: time.Now()}
	doc2 := types.Document{ID: "doc_2", OwnerUserID: u2.ID, Name: "u2.md", Status: "ready", CreatedAt: time.Now(), LastUpdatedAt: time.Now()}
	if err := s.UpsertDocument(doc1, []types.Chunk{{ID: "c1", DocID: doc1.ID, Content: "hello"}}); err != nil {
		t.Fatalf("upsert doc1: %v", err)
	}
	if err := s.UpsertDocument(doc2, []types.Chunk{{ID: "c2", DocID: doc2.ID, Content: "world"}}); err != nil {
		t.Fatalf("upsert doc2: %v", err)
	}

	listReq := httptest.NewRequest(http.MethodGet, "/api/documents", nil)
	listReq.Header.Set("Authorization", "Bearer "+token1)
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
	forbiddenReq.Header.Set("Authorization", "Bearer "+token2)
	forbiddenResp := httptest.NewRecorder()
	h.ServeHTTP(forbiddenResp, forbiddenReq)
	if forbiddenResp.Code != http.StatusForbidden {
		t.Fatalf("expected 403 for cross-user access, got %d", forbiddenResp.Code)
	}
}

func TestRegisterAndLoginFlow(t *testing.T) {
	tmp := t.TempDir()
	s, err := store.New(filepath.Join(tmp, "state.json"), filepath.Join(tmp, "audit.log"))
	if err != nil {
		t.Fatalf("store init error: %v", err)
	}
	server := NewServer(s, filepath.Join(tmp, "files"), log.New(io.Discard, "", 0))
	h := server.Routes()

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

func mustCreateUser(t *testing.T, s *store.Store, email, password string) types.User {
	t.Helper()
	hash, err := auth.HashPassword(password)
	if err != nil {
		t.Fatalf("hash password: %v", err)
	}
	user := types.User{ID: types.NewID("usr"), Email: email, PasswordHash: hash, CreatedAt: time.Now()}
	if err := s.CreateUser(user); err != nil {
		t.Fatalf("create user: %v", err)
	}
	return user
}

func mustCreateSession(t *testing.T, s *store.Store, userID string) string {
	t.Helper()
	token, err := auth.NewSessionToken()
	if err != nil {
		t.Fatalf("new token: %v", err)
	}
	session := types.Session{Token: token, UserID: userID, CreatedAt: time.Now(), ExpiresAt: time.Now().Add(time.Hour)}
	if err := s.CreateSession(session); err != nil {
		t.Fatalf("create session: %v", err)
	}
	return token
}
