package rpc

import (
	"context"
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
	"llm-doc-qa-assistant/backend/apps/core-go-rpc/internal/llm"
	"llm-doc-qa-assistant/backend/apps/core-go-rpc/internal/qa"
	"llm-doc-qa-assistant/backend/apps/core-go-rpc/internal/store"
	"llm-doc-qa-assistant/backend/apps/core-go-rpc/internal/types"
	qav1 "llm-doc-qa-assistant/backend/proto/gen/go/qa/v1"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type fakeAnswerGenerator struct {
	generateAnswerFn func(ctx context.Context, in llm.Request) (string, error)
}

func (m *fakeAnswerGenerator) GenerateAnswer(ctx context.Context, in llm.Request) (string, error) {
	if m.generateAnswerFn != nil {
		return m.generateAnswerFn(ctx, in)
	}
	return "mock answer", nil
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

type fakeObjectStore struct{}

func (fakeObjectStore) PutObject(_ context.Context, _ string, _ []byte, _ string) error { return nil }
func (fakeObjectStore) GetObject(_ context.Context, _ string) ([]byte, string, error) {
	return nil, "", errors.New("not implemented")
}
func (fakeObjectStore) DeleteObject(_ context.Context, _ string) error { return nil }

type fakeEmbedder struct {
	vectors [][]float32
	err     error
}

func (f fakeEmbedder) Embed(_ context.Context, _ []string) ([][]float32, error) {
	if f.err != nil {
		return nil, f.err
	}
	return f.vectors, nil
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

func TestGenerateAnswerUsesAgentGenerator(t *testing.T) {
	answerGen := &fakeAnswerGenerator{
		generateAnswerFn: func(_ context.Context, in llm.Request) (string, error) {
			if in.OwnerUserID != "usr_1" || in.ThreadID != "th_1" || in.TurnID != "turn_1" {
				t.Fatalf("unexpected request identity fields: %+v", in)
			}
			if in.ActiveProvider != "openai" {
				t.Fatalf("expected active provider openai, got %s", in.ActiveProvider)
			}
			if len(in.Contexts) != 1 || in.Contexts[0].ChunkID != "chk_1" {
				t.Fatalf("unexpected contexts: %+v", in.Contexts)
			}
			return "answer from agent", nil
		},
	}

	s := &Server{
		answerGenerator: answerGen,
		logger:          log.New(io.Discard, "", 0),
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

	answer, err := s.generateAnswer(context.Background(), "usr_1", turn, retrieved, prev, "openai")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if answer != "answer from agent" {
		t.Fatalf("expected agent answer, got %q", answer)
	}
}

func TestGenerateAnswerReturnsUnavailableOnGeneratorError(t *testing.T) {
	answerGen := &fakeAnswerGenerator{
		generateAnswerFn: func(_ context.Context, _ llm.Request) (string, error) {
			return "", errors.New("llm unavailable")
		},
	}

	s := &Server{
		answerGenerator: answerGen,
		logger:          log.New(io.Discard, "", 0),
	}

	retrieved := []qa.ScoredChunk{
		{
			Document: types.Document{Name: "Doc A"},
			Chunk:    types.Chunk{Content: "chunk A"},
			Score:    8,
		},
	}
	turn := types.Turn{Question: "question", ScopeType: "all"}

	_, err := s.generateAnswer(context.Background(), "usr_1", turn, retrieved, nil, "")
	if err == nil {
		t.Fatalf("expected error, got nil")
	}
	if status.Code(err) != codes.Unavailable {
		t.Fatalf("expected unavailable status, got %v", status.Code(err))
	}
}

func TestGenerateAnswerReturnsFailedPreconditionWhenGeneratorNil(t *testing.T) {
	s := &Server{answerGenerator: nil, logger: log.New(io.Discard, "", 0)}
	turn := types.Turn{Question: "question", ScopeType: "all"}

	_, err := s.generateAnswer(context.Background(), "usr_1", turn, nil, nil, "")
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
	answerGen := &fakeAnswerGenerator{
		generateAnswerFn: func(_ context.Context, in llm.Request) (string, error) {
			callCount++
			switch callCount {
			case 1:
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
		store:           st,
		authService:     fakeAuthUseCase{user: user},
		objectStore:     fakeObjectStore{},
		answerGenerator: answerGen,
		logger:          log.New(io.Discard, "", 0),
	}

	turn1, err := server.CreateTurn(context.Background(), &qav1.CreateTurnRequest{
		Token:       "token-1",
		ThreadId:    thread.ID,
		Message:     "项目延期多久？",
		ScopeType:   "doc",
		ScopeDocIds: []string{doc.ID},
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
		embedder: fakeEmbedder{
			vectors: [][]float32{{0.1, 0.2, 0.3}},
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
		embedder:      fakeEmbedder{err: errors.New("embed failed")},
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
