package handler

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"log"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	qav1 "llm-doc-qa-assistant/backend/proto/gen/go/qa/v1"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type mockCoreClient struct {
	healthFn           func(ctx context.Context, in *qav1.Empty, opts ...grpc.CallOption) (*qav1.HealthReply, error)
	registerFn         func(ctx context.Context, in *qav1.RegisterRequest, opts ...grpc.CallOption) (*qav1.AuthReply, error)
	loginFn            func(ctx context.Context, in *qav1.LoginRequest, opts ...grpc.CallOption) (*qav1.AuthReply, error)
	logoutFn           func(ctx context.Context, in *qav1.LogoutRequest, opts ...grpc.CallOption) (*qav1.Empty, error)
	meFn               func(ctx context.Context, in *qav1.MeRequest, opts ...grpc.CallOption) (*qav1.AuthReply, error)
	uploadDocumentFn   func(ctx context.Context, in *qav1.UploadDocumentRequest, opts ...grpc.CallOption) (*qav1.DocumentReply, error)
	listDocumentsFn    func(ctx context.Context, in *qav1.ListDocumentsRequest, opts ...grpc.CallOption) (*qav1.ListDocumentsReply, error)
	getDocumentFn      func(ctx context.Context, in *qav1.DocumentRequest, opts ...grpc.CallOption) (*qav1.DocumentReply, error)
	downloadDocumentFn func(ctx context.Context, in *qav1.DocumentRequest, opts ...grpc.CallOption) (*qav1.DownloadDocumentReply, error)
	deleteDocumentFn   func(ctx context.Context, in *qav1.DeleteDocumentRequest, opts ...grpc.CallOption) (*qav1.Empty, error)
	listThreadsFn      func(ctx context.Context, in *qav1.ListThreadsRequest, opts ...grpc.CallOption) (*qav1.ListThreadsReply, error)
	createThreadFn     func(ctx context.Context, in *qav1.CreateThreadRequest, opts ...grpc.CallOption) (*qav1.ThreadReply, error)
	createTurnFn       func(ctx context.Context, in *qav1.CreateTurnRequest, opts ...grpc.CallOption) (*qav1.CreateTurnReply, error)
	createTurnStreamFn func(ctx context.Context, in *qav1.CreateTurnRequest, opts ...grpc.CallOption) (qav1.CoreService_CreateTurnStreamClient, error)
	getTurnFn          func(ctx context.Context, in *qav1.GetTurnRequest, opts ...grpc.CallOption) (*qav1.GetTurnReply, error)
	getConfigFn        func(ctx context.Context, in *qav1.MeRequest, opts ...grpc.CallOption) (*qav1.ConfigReply, error)
	setConfigFn        func(ctx context.Context, in *qav1.SetConfigRequest, opts ...grpc.CallOption) (*qav1.ConfigReply, error)
}

func (m *mockCoreClient) Health(ctx context.Context, in *qav1.Empty, opts ...grpc.CallOption) (*qav1.HealthReply, error) {
	if m.healthFn != nil {
		return m.healthFn(ctx, in, opts...)
	}
	return &qav1.HealthReply{Status: "ok"}, nil
}

func (m *mockCoreClient) Register(ctx context.Context, in *qav1.RegisterRequest, opts ...grpc.CallOption) (*qav1.AuthReply, error) {
	if m.registerFn != nil {
		return m.registerFn(ctx, in, opts...)
	}
	return nil, status.Error(codes.Unimplemented, "register not mocked")
}

func (m *mockCoreClient) Login(ctx context.Context, in *qav1.LoginRequest, opts ...grpc.CallOption) (*qav1.AuthReply, error) {
	if m.loginFn != nil {
		return m.loginFn(ctx, in, opts...)
	}
	return nil, status.Error(codes.Unimplemented, "login not mocked")
}

func (m *mockCoreClient) Logout(ctx context.Context, in *qav1.LogoutRequest, opts ...grpc.CallOption) (*qav1.Empty, error) {
	if m.logoutFn != nil {
		return m.logoutFn(ctx, in, opts...)
	}
	return nil, status.Error(codes.Unimplemented, "logout not mocked")
}

func (m *mockCoreClient) Me(ctx context.Context, in *qav1.MeRequest, opts ...grpc.CallOption) (*qav1.AuthReply, error) {
	if m.meFn != nil {
		return m.meFn(ctx, in, opts...)
	}
	return nil, status.Error(codes.Unimplemented, "me not mocked")
}

func (m *mockCoreClient) UploadDocument(ctx context.Context, in *qav1.UploadDocumentRequest, opts ...grpc.CallOption) (*qav1.DocumentReply, error) {
	if m.uploadDocumentFn != nil {
		return m.uploadDocumentFn(ctx, in, opts...)
	}
	return nil, status.Error(codes.Unimplemented, "upload not mocked")
}

func (m *mockCoreClient) ListDocuments(ctx context.Context, in *qav1.ListDocumentsRequest, opts ...grpc.CallOption) (*qav1.ListDocumentsReply, error) {
	if m.listDocumentsFn != nil {
		return m.listDocumentsFn(ctx, in, opts...)
	}
	return nil, status.Error(codes.Unimplemented, "list documents not mocked")
}

func (m *mockCoreClient) GetDocument(ctx context.Context, in *qav1.DocumentRequest, opts ...grpc.CallOption) (*qav1.DocumentReply, error) {
	if m.getDocumentFn != nil {
		return m.getDocumentFn(ctx, in, opts...)
	}
	return nil, status.Error(codes.Unimplemented, "get document not mocked")
}

func (m *mockCoreClient) DownloadDocument(ctx context.Context, in *qav1.DocumentRequest, opts ...grpc.CallOption) (*qav1.DownloadDocumentReply, error) {
	if m.downloadDocumentFn != nil {
		return m.downloadDocumentFn(ctx, in, opts...)
	}
	return nil, status.Error(codes.Unimplemented, "download document not mocked")
}

func (m *mockCoreClient) DeleteDocument(ctx context.Context, in *qav1.DeleteDocumentRequest, opts ...grpc.CallOption) (*qav1.Empty, error) {
	if m.deleteDocumentFn != nil {
		return m.deleteDocumentFn(ctx, in, opts...)
	}
	return nil, status.Error(codes.Unimplemented, "delete document not mocked")
}

func (m *mockCoreClient) ListThreads(ctx context.Context, in *qav1.ListThreadsRequest, opts ...grpc.CallOption) (*qav1.ListThreadsReply, error) {
	if m.listThreadsFn != nil {
		return m.listThreadsFn(ctx, in, opts...)
	}
	return nil, status.Error(codes.Unimplemented, "list threads not mocked")
}

func (m *mockCoreClient) CreateThread(ctx context.Context, in *qav1.CreateThreadRequest, opts ...grpc.CallOption) (*qav1.ThreadReply, error) {
	if m.createThreadFn != nil {
		return m.createThreadFn(ctx, in, opts...)
	}
	return nil, status.Error(codes.Unimplemented, "create thread not mocked")
}

func (m *mockCoreClient) CreateTurn(ctx context.Context, in *qav1.CreateTurnRequest, opts ...grpc.CallOption) (*qav1.CreateTurnReply, error) {
	if m.createTurnFn != nil {
		return m.createTurnFn(ctx, in, opts...)
	}
	return nil, status.Error(codes.Unimplemented, "create turn not mocked")
}

func (m *mockCoreClient) CreateTurnStream(ctx context.Context, in *qav1.CreateTurnRequest, opts ...grpc.CallOption) (qav1.CoreService_CreateTurnStreamClient, error) {
	if m.createTurnStreamFn != nil {
		return m.createTurnStreamFn(ctx, in, opts...)
	}
	return nil, status.Error(codes.Unimplemented, "create turn stream not mocked")
}

func (m *mockCoreClient) GetTurn(ctx context.Context, in *qav1.GetTurnRequest, opts ...grpc.CallOption) (*qav1.GetTurnReply, error) {
	if m.getTurnFn != nil {
		return m.getTurnFn(ctx, in, opts...)
	}
	return nil, status.Error(codes.Unimplemented, "get turn not mocked")
}

func (m *mockCoreClient) GetConfig(ctx context.Context, in *qav1.MeRequest, opts ...grpc.CallOption) (*qav1.ConfigReply, error) {
	if m.getConfigFn != nil {
		return m.getConfigFn(ctx, in, opts...)
	}
	return nil, status.Error(codes.Unimplemented, "get config not mocked")
}

func (m *mockCoreClient) SetConfig(ctx context.Context, in *qav1.SetConfigRequest, opts ...grpc.CallOption) (*qav1.ConfigReply, error) {
	if m.setConfigFn != nil {
		return m.setConfigFn(ctx, in, opts...)
	}
	return nil, status.Error(codes.Unimplemented, "set config not mocked")
}

func TestHandleMeRequiresAuth(t *testing.T) {
	srv := NewServer(&mockCoreClient{}, log.New(io.Discard, "", 0))
	h := srv.Routes()

	req := httptest.NewRequest(http.MethodGet, "/api/auth/me", nil)
	resp := httptest.NewRecorder()
	h.ServeHTTP(resp, req)

	if resp.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", resp.Code)
	}
	body := decodeBody(t, resp.Body.Bytes())
	errMap := body["error"].(map[string]interface{})
	if errMap["code"] != "unauthorized" {
		t.Fatalf("expected unauthorized error code, got %v", errMap["code"])
	}
}

func TestHandleMePassesTokenToCore(t *testing.T) {
	core := &mockCoreClient{
		meFn: func(_ context.Context, in *qav1.MeRequest, _ ...grpc.CallOption) (*qav1.AuthReply, error) {
			if in.GetToken() != "token-123" {
				t.Fatalf("expected token-123, got %q", in.GetToken())
			}
			return &qav1.AuthReply{User: &qav1.User{Id: "usr_1", Email: "a@example.com"}}, nil
		},
	}
	srv := NewServer(core, log.New(io.Discard, "", 0))
	h := srv.Routes()

	req := httptest.NewRequest(http.MethodGet, "/api/auth/me", nil)
	req.Header.Set("Authorization", "Bearer token-123")
	resp := httptest.NewRecorder()
	h.ServeHTTP(resp, req)

	if resp.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d body=%s", resp.Code, resp.Body.String())
	}
	body := decodeBody(t, resp.Body.Bytes())
	user := body["user"].(map[string]interface{})
	if user["id"] != "usr_1" {
		t.Fatalf("expected user id usr_1, got %v", user["id"])
	}
}

func TestGetDocumentMapsGRPCNotFoundToHTTP404(t *testing.T) {
	core := &mockCoreClient{
		getDocumentFn: func(_ context.Context, in *qav1.DocumentRequest, _ ...grpc.CallOption) (*qav1.DocumentReply, error) {
			if in.GetDocumentId() != "doc_1" {
				t.Fatalf("expected doc_1, got %s", in.GetDocumentId())
			}
			return nil, status.Error(codes.NotFound, "document not found")
		},
	}
	srv := NewServer(core, log.New(io.Discard, "", 0))
	h := srv.Routes()

	req := httptest.NewRequest(http.MethodGet, "/api/documents/doc_1", nil)
	req.Header.Set("Authorization", "Bearer token-123")
	resp := httptest.NewRecorder()
	h.ServeHTTP(resp, req)

	if resp.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d body=%s", resp.Code, resp.Body.String())
	}
	body := decodeBody(t, resp.Body.Bytes())
	errMap := body["error"].(map[string]interface{})
	if errMap["code"] != "not_found" {
		t.Fatalf("expected not_found code, got %v", errMap["code"])
	}
}

func TestUploadRejectsUnsupportedExtensionBeforeCoreCall(t *testing.T) {
	called := false
	core := &mockCoreClient{
		uploadDocumentFn: func(_ context.Context, _ *qav1.UploadDocumentRequest, _ ...grpc.CallOption) (*qav1.DocumentReply, error) {
			called = true
			return &qav1.DocumentReply{}, nil
		},
	}
	srv := NewServer(core, log.New(io.Discard, "", 0))
	h := srv.Routes()

	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)
	part, err := writer.CreateFormFile("file", "malware.exe")
	if err != nil {
		t.Fatalf("create form file: %v", err)
	}
	_, _ = part.Write([]byte("not allowed"))
	_ = writer.Close()

	req := httptest.NewRequest(http.MethodPost, "/api/documents/upload", body)
	req.Header.Set("Authorization", "Bearer token-123")
	req.Header.Set("Content-Type", writer.FormDataContentType())
	resp := httptest.NewRecorder()
	h.ServeHTTP(resp, req)

	if resp.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d body=%s", resp.Code, resp.Body.String())
	}
	if called {
		t.Fatalf("core UploadDocument should not be called for unsupported extension")
	}
}

func TestDownloadDocumentSetsHeadersAndBody(t *testing.T) {
	core := &mockCoreClient{
		downloadDocumentFn: func(_ context.Context, in *qav1.DocumentRequest, _ ...grpc.CallOption) (*qav1.DownloadDocumentReply, error) {
			if in.GetDocumentId() != "doc_1" {
				t.Fatalf("expected doc_1, got %s", in.GetDocumentId())
			}
			return &qav1.DownloadDocumentReply{
				Content:     []byte("hello-download"),
				ContentType: "text/plain; charset=utf-8",
				Filename:    "hello.txt",
			}, nil
		},
	}
	srv := NewServer(core, log.New(io.Discard, "", 0))
	h := srv.Routes()

	req := httptest.NewRequest(http.MethodGet, "/api/documents/doc_1/download", nil)
	req.Header.Set("Authorization", "Bearer token-123")
	resp := httptest.NewRecorder()
	h.ServeHTTP(resp, req)

	if resp.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d body=%s", resp.Code, resp.Body.String())
	}
	if ct := resp.Header().Get("Content-Type"); ct != "text/plain; charset=utf-8" {
		t.Fatalf("unexpected content type: %q", ct)
	}
	if disp := resp.Header().Get("Content-Disposition"); !strings.Contains(disp, "hello.txt") {
		t.Fatalf("unexpected content disposition: %q", disp)
	}
	if body := resp.Body.String(); body != "hello-download" {
		t.Fatalf("unexpected body: %q", body)
	}
}

func TestCreateTurnForwardsThinkMode(t *testing.T) {
	core := &mockCoreClient{
		createTurnFn: func(_ context.Context, in *qav1.CreateTurnRequest, _ ...grpc.CallOption) (*qav1.CreateTurnReply, error) {
			if in.GetThreadId() != "th_1" {
				t.Fatalf("expected thread th_1, got %s", in.GetThreadId())
			}
			if !in.GetThinkMode() {
				t.Fatalf("expected think_mode true")
			}
			return &qav1.CreateTurnReply{
				Turn: &qav1.Turn{
					Id:        "turn_1",
					ThreadId:  "th_1",
					Question:  in.GetMessage(),
					Answer:    "ok",
					ScopeType: in.GetScopeType(),
				},
			}, nil
		},
	}
	srv := NewServer(core, log.New(io.Discard, "", 0))
	h := srv.Routes()

	req := httptest.NewRequest(http.MethodPost, "/api/threads/th_1/turns", strings.NewReader(`{
		"message":"hi",
		"scope_type":"all",
		"scope_doc_ids":[],
		"think_mode":true
	}`))
	req.Header.Set("Authorization", "Bearer token-123")
	req.Header.Set("Content-Type", "application/json")
	resp := httptest.NewRecorder()
	h.ServeHTTP(resp, req)

	if resp.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d body=%s", resp.Code, resp.Body.String())
	}
}

func decodeBody(t *testing.T, payload []byte) map[string]interface{} {
	t.Helper()
	var out map[string]interface{}
	if err := json.Unmarshal(payload, &out); err != nil {
		t.Fatalf("decode json failed: %v, payload=%s", err, string(payload))
	}
	return out
}
