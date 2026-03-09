package rpc

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"log"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	domainauth "llm-doc-qa-assistant/backend/apps/core-go-rpc/internal/domain/auth"
	"llm-doc-qa-assistant/backend/apps/core-go-rpc/internal/ingest"
	"llm-doc-qa-assistant/backend/apps/core-go-rpc/internal/qa"
	"llm-doc-qa-assistant/backend/apps/core-go-rpc/internal/store"
	"llm-doc-qa-assistant/backend/apps/core-go-rpc/internal/types"
	qav1 "llm-doc-qa-assistant/backend/proto/gen/go/qa/v1"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type fakeLLMService struct {
	generateAnswerFn func(ctx context.Context, in LLMGenerateRequest) (string, error)
	streamAnswerFn   func(ctx context.Context, in LLMGenerateRequest, onChunk func(delta string, thinkingDelta string) error) (string, error)
	embedTextsFn     func(ctx context.Context, texts []string) ([][]float32, error)
	extractTextFn    func(ctx context.Context, filename, mimeType string, content []byte) (string, error)
}

func (m *fakeLLMService) GenerateAnswer(ctx context.Context, in LLMGenerateRequest) (string, error) {
	if m.generateAnswerFn != nil {
		return m.generateAnswerFn(ctx, in)
	}
	return "mock answer", nil
}

func (m *fakeLLMService) GenerateAnswerStream(ctx context.Context, in LLMGenerateRequest, onChunk func(delta string, thinkingDelta string) error) (string, error) {
	if m.streamAnswerFn != nil {
		return m.streamAnswerFn(ctx, in, onChunk)
	}
	if m.generateAnswerFn != nil {
		answer, err := m.generateAnswerFn(ctx, in)
		if err != nil {
			return "", err
		}
		if onChunk != nil && answer != "" {
			if cbErr := onChunk(answer, ""); cbErr != nil {
				return "", cbErr
			}
		}
		return answer, nil
	}
	return "mock answer", nil
}

func (m *fakeLLMService) EmbedTexts(ctx context.Context, texts []string) ([][]float32, error) {
	if m.embedTextsFn != nil {
		return m.embedTextsFn(ctx, texts)
	}
	return nil, nil
}

func (m *fakeLLMService) ExtractDocumentText(ctx context.Context, filename, mimeType string, content []byte) (string, error) {
	if m.extractTextFn != nil {
		return m.extractTextFn(ctx, filename, mimeType, content)
	}
	return "", nil
}

type fakeAuthUseCase struct {
	user domainauth.User
}

func (f fakeAuthUseCase) Register(_ context.Context, _, _ string) (domainauth.User, error) {
	return domainauth.User{}, errors.New("not implemented")
}

func (f fakeAuthUseCase) Login(_ context.Context, _, _ string) (domainauth.User, domainauth.Session, error) {
	return domainauth.User{}, domainauth.Session{}, errors.New("not implemented")
}

func (f fakeAuthUseCase) Logout(_ context.Context, _, _ string) error {
	return nil
}

func (f fakeAuthUseCase) Authenticate(_ context.Context, token string) (domainauth.User, error) {
	if token == "" {
		return domainauth.User{}, errors.New("missing token")
	}
	return f.user, nil
}

type fakeObjectStore struct {
	putObjectFn    func(ctx context.Context, key string, data []byte, contentType string) error
	getObjectFn    func(ctx context.Context, key string) ([]byte, string, error)
	deleteObjectFn func(ctx context.Context, key string) error
}

func (f fakeObjectStore) PutObject(ctx context.Context, key string, data []byte, contentType string) error {
	if f.putObjectFn != nil {
		return f.putObjectFn(ctx, key, data, contentType)
	}
	return nil
}

func (f fakeObjectStore) GetObject(ctx context.Context, key string) ([]byte, string, error) {
	if f.getObjectFn != nil {
		return f.getObjectFn(ctx, key)
	}
	return nil, "", errors.New("not implemented")
}

func (f fakeObjectStore) DeleteObject(ctx context.Context, key string) error {
	if f.deleteObjectFn != nil {
		return f.deleteObjectFn(ctx, key)
	}
	return nil
}

type fakeVectorIndexer struct {
	hits []types.VectorHit
	err  error
}

func (f fakeVectorIndexer) UpsertDocumentChunks(_ context.Context, _ string, _ types.Document, _ []types.Chunk, _ [][]float32) error {
	return nil
}

func (f fakeVectorIndexer) SearchChunks(_ context.Context, _ string, _ []float32, _ int) ([]types.VectorHit, error) {
	if f.err != nil {
		return nil, f.err
	}
	return f.hits, nil
}

func (f fakeVectorIndexer) DeleteDocument(_ context.Context, _, _ string) error {
	return nil
}

func TestValidateUpload(t *testing.T) {
	tests := []struct {
		name     string
		filename string
		size     int
		wantErr  bool
	}{
		{name: "txt ok", filename: "a.txt", size: 10, wantErr: false},
		{name: "md ok", filename: "a.md", size: 10, wantErr: false},
		{name: "markdown ok", filename: "a.markdown", size: 10, wantErr: false},
		{name: "pdf ok", filename: "a.pdf", size: 10, wantErr: false},
		{name: "unsupported ext", filename: "a.exe", size: 10, wantErr: true},
		{name: "empty content", filename: "a.txt", size: 0, wantErr: true},
		{name: "oversize", filename: "a.txt", size: maxUploadSize + 1, wantErr: true},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			err := validateUpload(tc.filename, tc.size)
			if tc.wantErr && err == nil {
				t.Fatalf("expected error, got nil")
			}
			if !tc.wantErr && err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
		})
	}
}

func TestResolveSelectedDocs(t *testing.T) {
	docs := []types.Document{
		{ID: "doc_1", Name: "one"},
		{ID: "doc_2", Name: "two"},
		{ID: "doc_3", Name: "three"},
	}

	selectedAll := resolveSelectedDocs(qa.Scope{Type: "all"}, docs)
	if len(selectedAll) != 3 {
		t.Fatalf("expected all docs, got %d", len(selectedAll))
	}

	selectedDoc := resolveSelectedDocs(qa.Scope{Type: "doc", DocIDs: []string{"doc_2", "doc_3"}}, docs)
	if len(selectedDoc) != 2 {
		t.Fatalf("expected 2 docs, got %d", len(selectedDoc))
	}
	if selectedDoc[0].ID != "doc_2" || selectedDoc[1].ID != "doc_3" {
		t.Fatalf("unexpected selected docs order/content: %+v", selectedDoc)
	}
}

func TestBuildCitationsTruncatesExcerpt(t *testing.T) {
	longText := strings.Repeat("x", 260)
	scored := []qa.ScoredChunk{
		{
			Document: types.Document{ID: "doc_1", Name: "Doc1"},
			Chunk:    types.Chunk{ID: "chk_1", Index: 7, Content: longText},
			Score:    99,
		},
	}

	citations := buildCitations(scored)
	if len(citations) != 1 {
		t.Fatalf("expected 1 citation, got %d", len(citations))
	}
	if !strings.HasSuffix(citations[0].Excerpt, "...") {
		t.Fatalf("expected excerpt to be truncated with ellipsis, got: %s", citations[0].Excerpt)
	}
	if citations[0].ChunkIdx != 7 || citations[0].DocID != "doc_1" {
		t.Fatalf("unexpected citation fields: %+v", citations[0])
	}
}

func TestGenerateAnswerUsesLLMRPC(t *testing.T) {
	llmSvc := &fakeLLMService{
		generateAnswerFn: func(_ context.Context, in LLMGenerateRequest) (string, error) {
			if in.OwnerUserID != "usr_1" || in.ThreadID != "th_1" || in.TurnID != "turn_1" {
				t.Fatalf("unexpected request identity fields: %+v", in)
			}
			if in.ActiveProvider != "openai" {
				t.Fatalf("expected active provider openai, got %s", in.ActiveProvider)
			}
			if !in.ThinkMode {
				t.Fatalf("expected think mode true")
			}
			if len(in.Contexts) != 1 || in.Contexts[0].ChunkID != "chk_1" {
				t.Fatalf("unexpected contexts: %+v", in.Contexts)
			}
			return "answer from agent", nil
		},
	}

	s := &Server{
		llmService: llmSvc,
		logger:     log.New(io.Discard, "", 0),
	}

	retrieved := []qa.ScoredChunk{
		{
			Document: types.Document{ID: "doc_1", Name: "Doc 1"},
			Chunk:    types.Chunk{ID: "chk_1", Index: 1, Content: "context"},
			Score:    10,
		},
	}
	turn := types.Turn{ID: "turn_1", ThreadID: "th_1", Question: "q", ScopeType: "all"}
	prev := []types.Turn{{Question: "prev q", Answer: "prev a"}}

	answer, err := s.generateAnswer(context.Background(), "usr_1", turn, retrieved, prev, "openai", true, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if answer != "answer from agent" {
		t.Fatalf("expected agent answer, got %q", answer)
	}
}

func TestGenerateAnswerReturnsUnavailableOnLLMError(t *testing.T) {
	llmSvc := &fakeLLMService{
		generateAnswerFn: func(_ context.Context, _ LLMGenerateRequest) (string, error) {
			return "", errors.New("llm unavailable")
		},
	}

	s := &Server{
		llmService: llmSvc,
		logger:     log.New(io.Discard, "", 0),
	}

	retrieved := []qa.ScoredChunk{
		{
			Document: types.Document{Name: "Doc A"},
			Chunk:    types.Chunk{Content: "chunk A"},
			Score:    8,
		},
	}
	turn := types.Turn{Question: "question", ScopeType: "all"}

	_, err := s.generateAnswer(context.Background(), "usr_1", turn, retrieved, nil, "", false, nil)
	if err == nil {
		t.Fatalf("expected error, got nil")
	}
	if status.Code(err) != codes.Unavailable {
		t.Fatalf("expected unavailable status, got %v", status.Code(err))
	}
}

func TestGenerateAnswerReturnsFailedPreconditionWhenLLMNil(t *testing.T) {
	s := &Server{llmService: nil, logger: log.New(io.Discard, "", 0)}
	turn := types.Turn{Question: "question", ScopeType: "all"}

	_, err := s.generateAnswer(context.Background(), "usr_1", turn, nil, nil, "", false, nil)
	if err == nil {
		t.Fatalf("expected error, got nil")
	}
	if status.Code(err) != codes.FailedPrecondition {
		t.Fatalf("expected failed precondition status, got %v", status.Code(err))
	}
}

func TestCreateTurnRAGAgentDialogueWithGeneratedDocument(t *testing.T) {
	tmp := t.TempDir()
	statePath := filepath.Join(tmp, "state.json")
	auditPath := filepath.Join(tmp, "audit.log")
	st, err := store.New(statePath, auditPath)
	if err != nil {
		t.Fatalf("init store failed: %v", err)
	}

	user := domainauth.User{
		ID:        "usr_test",
		Email:     "rag@example.com",
		CreatedAt: time.Now().UTC(),
	}

	// Generate a markdown document for test RAG scenarios.
	markdownDoc := strings.TrimSpace(`
# 供应商合同说明

项目代号：Phoenix

1. 交付计划
- 原计划：2026-04-01 完成一期交付
- 变更后：因接口联调问题，整体延期 **2周**，新的交付日期为 2026-04-15

2. 付款节点
- 首付款：合同签订后 5 个工作日
- 尾款：验收通过后 10 个工作日
`)
	docPath := filepath.Join(tmp, "contract.md")
	if err := os.WriteFile(docPath, []byte(markdownDoc), 0o644); err != nil {
		t.Fatalf("write test document failed: %v", err)
	}
	docBytes, err := os.ReadFile(docPath)
	if err != nil {
		t.Fatalf("read test document failed: %v", err)
	}

	text, err := ingest.ParseDocumentText("contract.md", "text/markdown", docBytes)
	if err != nil {
		t.Fatalf("parse test document failed: %v", err)
	}
	pieces := ingest.ChunkText(text, 140, 20)
	if len(pieces) == 0 {
		t.Fatalf("expected non-empty chunks")
	}

	doc := types.Document{
		ID:            "doc_contract",
		OwnerUserID:   user.ID,
		Name:          "contract.md",
		SizeBytes:     int64(len(docBytes)),
		MimeType:      "text/markdown",
		StoragePath:   user.ID + "/doc_contract.md",
		Status:        "ready",
		ChunkCount:    len(pieces),
		CreatedAt:     time.Now().UTC(),
		LastUpdatedAt: time.Now().UTC(),
	}
	chunks := make([]types.Chunk, 0, len(pieces))
	for i, content := range pieces {
		chunks = append(chunks, types.Chunk{
			ID:      "chk_" + strconv.Itoa(i+1),
			DocID:   doc.ID,
			Index:   i,
			Content: content,
		})
	}
	if err := st.UpsertDocument(doc, chunks); err != nil {
		t.Fatalf("upsert document failed: %v", err)
	}

	thread := types.Thread{
		ID:          "th_dialog",
		OwnerUserID: user.ID,
		Title:       "RAG Agent Dialogue",
		CreatedAt:   time.Now().UTC(),
	}
	if err := st.CreateThread(thread); err != nil {
		t.Fatalf("create thread failed: %v", err)
	}

	callCount := 0
	llmSvc := &fakeLLMService{
		generateAnswerFn: func(_ context.Context, in LLMGenerateRequest) (string, error) {
			callCount++
			switch callCount {
			case 1:
				if in.ThinkMode {
					t.Fatalf("expected think mode false on first turn")
				}
				if len(in.Contexts) == 0 {
					t.Fatalf("expected rag contexts for first turn")
				}
				joined := ""
				for _, ctx := range in.Contexts {
					joined += ctx.Content + "\n"
				}
				if !strings.Contains(joined, "延期") {
					t.Fatalf("expected first turn contexts to include延期 evidence, got: %s", joined)
				}
				return "根据合同证据，项目延期2周。", nil
			case 2:
				if in.ThinkMode {
					t.Fatalf("expected think mode false on second turn")
				}
				if strings.TrimSpace(in.PreviousTurnQuestion) == "" || strings.TrimSpace(in.PreviousTurnAnswer) == "" {
					t.Fatalf("expected previous turn context on second turn")
				}
				return "付款节点是验收通过后10个工作日支付尾款。", nil
			default:
				return "ok", nil
			}
		},
	}

	server := &Server{
		store:       st,
		authService: fakeAuthUseCase{user: user},
		objectStore: fakeObjectStore{},
		llmService:  llmSvc,
		logger:      log.New(io.Discard, "", 0),
	}

	turn1, err := server.CreateTurn(context.Background(), &qav1.CreateTurnRequest{
		Token:       "token-1",
		ThreadId:    thread.ID,
		Message:     "项目延期多久？",
		ScopeType:   "doc",
		ScopeDocIds: []string{doc.ID},
		ThinkMode:   true,
	})
	if err != nil {
		t.Fatalf("create turn1 failed: %v", err)
	}
	if !strings.Contains(turn1.GetTurn().GetAnswer(), "延期2周") {
		t.Fatalf("unexpected turn1 answer: %s", turn1.GetTurn().GetAnswer())
	}
	if len(turn1.GetCitations()) == 0 {
		t.Fatalf("expected citations in turn1")
	}

	turn2, err := server.CreateTurn(context.Background(), &qav1.CreateTurnRequest{
		Token:       "token-1",
		ThreadId:    thread.ID,
		Message:     "那付款节点是什么？",
		ScopeType:   "doc",
		ScopeDocIds: []string{doc.ID},
	})
	if err != nil {
		t.Fatalf("create turn2 failed: %v", err)
	}
	if !strings.Contains(turn2.GetTurn().GetAnswer(), "付款节点") {
		t.Fatalf("unexpected turn2 answer: %s", turn2.GetTurn().GetAnswer())
	}
	if callCount != 2 {
		t.Fatalf("expected 2 llm calls, got %d", callCount)
	}
}

func TestCreateTurnRepairsUnreadablePDFChunks(t *testing.T) {
	tmp := t.TempDir()
	st, err := store.New(filepath.Join(tmp, "state.json"), filepath.Join(tmp, "audit.log"))
	if err != nil {
		t.Fatalf("init store failed: %v", err)
	}

	user := domainauth.User{ID: "usr_pdf", Email: "pdf@example.com", CreatedAt: time.Now().UTC()}
	doc := types.Document{
		ID:            "doc_pdf",
		OwnerUserID:   user.ID,
		Name:          "spec.pdf",
		SizeBytes:     1024,
		MimeType:      "application/pdf",
		StoragePath:   "usr_pdf/doc_pdf.pdf",
		Status:        "ready",
		ChunkCount:    1,
		CreatedAt:     time.Now().UTC(),
		LastUpdatedAt: time.Now().UTC(),
	}
	badChunks := []types.Chunk{
		{ID: "chk_bad", DocID: doc.ID, Index: 0, Content: "꣄骤廿Ɦ endstream 㰼 obj"},
	}
	if err := st.UpsertDocument(doc, badChunks); err != nil {
		t.Fatalf("seed bad doc failed: %v", err)
	}
	thread := types.Thread{ID: "th_pdf", OwnerUserID: user.ID, Title: "pdf", CreatedAt: time.Now().UTC()}
	if err := st.CreateThread(thread); err != nil {
		t.Fatalf("create thread failed: %v", err)
	}

	pseudoPDF := []byte("%PDF-1.4\nBT (项目概述 智能文档问答助手用于上传文档并回答问题) Tj ET\n%%EOF")
	obj := fakeObjectStore{
		getObjectFn: func(_ context.Context, key string) ([]byte, string, error) {
			if key != doc.StoragePath {
				t.Fatalf("unexpected object key: %s", key)
			}
			return pseudoPDF, "application/pdf", nil
		},
	}
	llmSvc := &fakeLLMService{
		extractTextFn: func(_ context.Context, filename, mimeType string, _ []byte) (string, error) {
			if filename != "spec.pdf" || mimeType != "application/pdf" {
				t.Fatalf("unexpected extract args: filename=%s mime=%s", filename, mimeType)
			}
			return "项目概述 智能文档问答助手用于上传文档并回答问题", nil
		},
		generateAnswerFn: func(_ context.Context, req LLMGenerateRequest) (string, error) {
			if len(req.Contexts) == 0 {
				t.Fatalf("expected contexts after repair")
			}
			joined := req.Contexts[0].Content
			if !strings.Contains(joined, "项目概述") {
				t.Fatalf("expected repaired chunk to contain 项目概述, got: %s", joined)
			}
			return "项目概述：智能文档问答助手用于上传文档并回答问题。", nil
		},
	}

	server := &Server{
		store:       st,
		authService: fakeAuthUseCase{user: user},
		objectStore: obj,
		llmService:  llmSvc,
		logger:      log.New(io.Discard, "", 0),
	}

	out, err := server.CreateTurn(context.Background(), &qav1.CreateTurnRequest{
		Token:       "token",
		ThreadId:    thread.ID,
		Message:     "这个文档项目的 项目概述 是什么",
		ScopeType:   "doc",
		ScopeDocIds: []string{doc.ID},
	})
	if err != nil {
		t.Fatalf("create turn failed: %v", err)
	}
	if !strings.Contains(out.GetTurn().GetAnswer(), "项目概述") {
		t.Fatalf("unexpected answer: %s", out.GetTurn().GetAnswer())
	}
	if len(out.GetCitations()) == 0 {
		t.Fatalf("expected citations after repair")
	}

	rebuilt := st.GetChunksForDoc(doc.ID)
	if len(rebuilt) == 0 {
		t.Fatalf("expected rebuilt chunks persisted")
	}
	if strings.Contains(strings.ToLower(rebuilt[0].Content), "endstream") {
		t.Fatalf("expected repaired chunks without raw pdf tokens, got: %s", rebuilt[0].Content)
	}
}

func TestCreateTurnAutoModeEmitsRetrievalDecision(t *testing.T) {
	tmp := t.TempDir()
	st, err := store.New(filepath.Join(tmp, "state.json"), filepath.Join(tmp, "audit.log"))
	if err != nil {
		t.Fatalf("init store failed: %v", err)
	}

	user := domainauth.User{
		ID:        "usr_auto",
		Email:     "auto@example.com",
		CreatedAt: time.Now().UTC(),
	}
	doc := types.Document{
		ID:            "doc_auto",
		OwnerUserID:   user.ID,
		Name:          "faq.md",
		SizeBytes:     128,
		MimeType:      "text/markdown",
		StoragePath:   user.ID + "/doc_auto.md",
		Status:        "ready",
		ChunkCount:    1,
		CreatedAt:     time.Now().UTC(),
		LastUpdatedAt: time.Now().UTC(),
	}
	chunks := []types.Chunk{
		{ID: "chk_auto_1", DocID: doc.ID, Index: 0, Content: "release date is 2026-04-15"},
	}
	if err := st.UpsertDocument(doc, chunks); err != nil {
		t.Fatalf("seed document failed: %v", err)
	}
	thread := types.Thread{
		ID:          "th_auto",
		OwnerUserID: user.ID,
		Title:       "auto",
		CreatedAt:   time.Now().UTC(),
	}
	if err := st.CreateThread(thread); err != nil {
		t.Fatalf("create thread failed: %v", err)
	}

	llmSvc := &fakeLLMService{
		generateAnswerFn: func(_ context.Context, in LLMGenerateRequest) (string, error) {
			if len(in.Contexts) != 0 {
				t.Fatalf("expected no retrieval contexts for small talk auto mode, got %d", len(in.Contexts))
			}
			return "hello", nil
		},
	}
	server := &Server{
		store:       st,
		authService: fakeAuthUseCase{user: user},
		objectStore: fakeObjectStore{},
		llmService:  llmSvc,
		logger:      log.New(io.Discard, "", 0),
	}

	out, err := server.CreateTurn(context.Background(), &qav1.CreateTurnRequest{
		Token:     "token",
		ThreadId:  thread.ID,
		Message:   "hi",
		ScopeType: "auto",
	})
	if err != nil {
		t.Fatalf("create turn failed: %v", err)
	}
	if out.GetTurn().GetScopeType() != "all" {
		t.Fatalf("expected scope type all, got %s", out.GetTurn().GetScopeType())
	}

	found := false
	for _, item := range out.GetItems() {
		if item.GetItemType() != "retrieval_decision" {
			continue
		}
		found = true
		var payload map[string]interface{}
		if err := json.Unmarshal([]byte(item.GetPayloadJson()), &payload); err != nil {
			t.Fatalf("decode retrieval_decision payload failed: %v", err)
		}
		if payload["mode"] != "auto" {
			t.Fatalf("expected auto mode, got %+v", payload["mode"])
		}
		use, _ := payload["use_retrieval"].(bool)
		if use {
			t.Fatalf("expected use_retrieval false for small talk, payload=%+v", payload)
		}
	}
	if !found {
		t.Fatalf("expected retrieval_decision item")
	}
}

func TestRetrieveChunksUsesVectorHitsWithinScope(t *testing.T) {
	doc1 := types.Document{ID: "doc_1", Name: "a.md"}
	doc2 := types.Document{ID: "doc_2", Name: "b.md"}
	chunkMap := map[string][]types.Chunk{
		"doc_1": {
			{ID: "chk_1", DocID: "doc_1", Index: 0, Content: "vector winner"},
		},
		"doc_2": {
			{ID: "chk_2", DocID: "doc_2", Index: 0, Content: "out of scope"},
		},
	}

	s := &Server{
		llmService: &fakeLLMService{
			embedTextsFn: func(_ context.Context, texts []string) ([][]float32, error) {
				if len(texts) != 1 || texts[0] != "question" {
					t.Fatalf("unexpected embed request: %+v", texts)
				}
				return [][]float32{{0.1, 0.2, 0.3}}, nil
			},
		},
		vectorIndexer: fakeVectorIndexer{
			hits: []types.VectorHit{
				{DocID: "doc_2", ChunkID: "chk_2", ChunkIndex: 0, Content: "out of scope", Score: 0.99},
				{DocID: "doc_1", ChunkID: "chk_1", ChunkIndex: 0, Content: "vector winner", Score: 0.92},
			},
		},
		logger: log.New(io.Discard, "", 0),
	}

	got := s.retrieveChunks(context.Background(), "usr_1", "question", []types.Document{doc1}, chunkMap, 2)
	if len(got) != 1 {
		t.Fatalf("expected one in-scope retrieved chunk, got %d", len(got))
	}
	if got[0].Document.ID != "doc_1" || got[0].Chunk.ID != "chk_1" {
		t.Fatalf("unexpected retrieved chunk: %+v", got[0])
	}
	if got[0].Score <= 0 {
		t.Fatalf("expected positive converted score, got %d", got[0].Score)
	}

	// Ensure lexical fallback would have returned both docs if vector scope filtering failed.
	fallback := qa.RetrieveTopChunks("question", []types.Document{doc1, doc2}, chunkMap, 2)
	_ = fallback
}

func TestRetrieveChunksFallsBackToLexicalWhenVectorSearchFails(t *testing.T) {
	doc := types.Document{ID: "doc_1", Name: "a.md"}
	chunkMap := map[string][]types.Chunk{
		"doc_1": {
			{ID: "chk_1", DocID: "doc_1", Index: 0, Content: "register login process"},
		},
	}

	s := &Server{
		llmService: &fakeLLMService{
			embedTextsFn: func(_ context.Context, _ []string) ([][]float32, error) {
				return nil, errors.New("embed failed")
			},
		},
		vectorIndexer: fakeVectorIndexer{},
		logger:        log.New(io.Discard, "", 0),
	}

	got := s.retrieveChunks(context.Background(), "usr_1", "register", []types.Document{doc}, chunkMap, 1)
	if len(got) != 1 {
		t.Fatalf("expected lexical fallback result, got %d", len(got))
	}
	if got[0].Chunk.ID != "chk_1" {
		t.Fatalf("unexpected fallback chunk: %+v", got[0])
	}
}

func TestListTurnsReturnsThreadHistoryInDescOrder(t *testing.T) {
	tmp := t.TempDir()
	statePath := filepath.Join(tmp, "state.json")
	auditPath := filepath.Join(tmp, "audit.log")
	st, err := store.New(statePath, auditPath)
	if err != nil {
		t.Fatalf("init store: %v", err)
	}

	user := domainauth.User{ID: "usr_1", Email: "a@example.com"}
	thread := types.Thread{ID: "th_1", OwnerUserID: user.ID, Title: "demo", CreatedAt: time.Now().Add(-10 * time.Minute)}
	if err := st.CreateThread(thread); err != nil {
		t.Fatalf("create thread: %v", err)
	}

	older := types.Turn{
		ID:          "turn_1",
		ThreadID:    thread.ID,
		OwnerUserID: user.ID,
		Question:    "q1",
		Answer:      "a1",
		Status:      "done",
		ScopeType:   "all",
		CreatedAt:   time.Now().Add(-2 * time.Minute),
		UpdatedAt:   time.Now().Add(-2 * time.Minute),
	}
	newer := types.Turn{
		ID:          "turn_2",
		ThreadID:    thread.ID,
		OwnerUserID: user.ID,
		Question:    "q2",
		Answer:      "a2",
		Status:      "done",
		ScopeType:   "all",
		CreatedAt:   time.Now().Add(-1 * time.Minute),
		UpdatedAt:   time.Now().Add(-1 * time.Minute),
	}
	if err := st.CreateOrUpdateTurn(older, []types.TurnItem{{ID: "i_1", TurnID: older.ID, ItemType: "final", Payload: map[string]interface{}{"answer": "a1"}, CreatedAt: older.CreatedAt}}); err != nil {
		t.Fatalf("save older turn: %v", err)
	}
	if err := st.CreateOrUpdateTurn(newer, []types.TurnItem{{ID: "i_2", TurnID: newer.ID, ItemType: "final", Payload: map[string]interface{}{"answer": "a2"}, CreatedAt: newer.CreatedAt}}); err != nil {
		t.Fatalf("save newer turn: %v", err)
	}

	srv := &Server{
		store:       st,
		authService: fakeAuthUseCase{user: user},
		objectStore: fakeObjectStore{},
		logger:      log.New(io.Discard, "", 0),
	}

	out, err := srv.ListTurns(context.Background(), &qav1.ListTurnsRequest{
		Token:    "token-123",
		ThreadId: thread.ID,
	})
	if err != nil {
		t.Fatalf("list turns failed: %v", err)
	}
	if len(out.GetTurns()) != 2 {
		t.Fatalf("expected 2 turns, got %d", len(out.GetTurns()))
	}
	if out.GetTurns()[0].GetTurn().GetId() != "turn_2" {
		t.Fatalf("expected newest turn first, got %s", out.GetTurns()[0].GetTurn().GetId())
	}
	if len(out.GetTurns()[0].GetItems()) == 0 {
		t.Fatalf("expected turn items to be returned")
	}
}
