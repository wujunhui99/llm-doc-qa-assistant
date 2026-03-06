package rpc

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"path/filepath"
	"sort"
	"strings"
	"time"

	authapp "llm-doc-qa-assistant/backend/apps/core-go-rpc/internal/application/auth"
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

const (
	maxUploadSize = 10 << 20
)

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

type TextEmbedder interface {
	Embed(ctx context.Context, texts []string) ([][]float32, error)
}

type VectorIndexer interface {
	UpsertDocumentChunks(ctx context.Context, ownerID string, doc types.Document, chunks []types.Chunk, vectors [][]float32) error
	SearchChunks(ctx context.Context, ownerID string, queryVector []float32, limit int) ([]types.VectorHit, error)
	DeleteDocument(ctx context.Context, ownerID, docID string) error
}

type Server struct {
	qav1.UnimplementedCoreServiceServer

	store           *store.Store
	authService     AuthUseCase
	objectStore     DocumentObjectStore
	answerGenerator llm.Generator
	embedder        TextEmbedder
	vectorIndexer   VectorIndexer
	logger          *log.Logger
}

func NewServer(
	store *store.Store,
	authService AuthUseCase,
	objectStore DocumentObjectStore,
	logger *log.Logger,
) *Server {
	if store == nil {
		panic("store cannot be nil")
	}
	if authService == nil {
		panic("authService cannot be nil")
	}
	if objectStore == nil {
		panic("objectStore cannot be nil")
	}
	if logger == nil {
		logger = log.New(log.Writer(), "[core-go-rpc] ", log.LstdFlags|log.LUTC)
	}

	return &Server{
		store:           store,
		authService:     authService,
		objectStore:     objectStore,
		answerGenerator: nil,
		logger:          logger,
	}
}

func (s *Server) WithVectorSearch(embedder TextEmbedder, vectorIndexer VectorIndexer) *Server {
	s.embedder = embedder
	s.vectorIndexer = vectorIndexer
	return s
}

func (s *Server) WithAnswerGenerator(generator llm.Generator) *Server {
	s.answerGenerator = generator
	return s
}

func (s *Server) Health(_ context.Context, _ *qav1.Empty) (*qav1.HealthReply, error) {
	return &qav1.HealthReply{Status: "ok", Time: time.Now().UTC().Format(time.RFC3339Nano)}, nil
}

func (s *Server) Register(ctx context.Context, req *qav1.RegisterRequest) (*qav1.AuthReply, error) {
	if req == nil {
		return nil, status.Error(codes.InvalidArgument, "request is required")
	}

	user, err := s.authService.Register(ctx, req.GetEmail(), req.GetPassword())
	if err != nil {
		return nil, s.authError(err)
	}

	return &qav1.AuthReply{User: toProtoUser(user)}, nil
}

func (s *Server) Login(ctx context.Context, req *qav1.LoginRequest) (*qav1.AuthReply, error) {
	if req == nil {
		return nil, status.Error(codes.InvalidArgument, "request is required")
	}

	user, session, err := s.authService.Login(ctx, req.GetEmail(), req.GetPassword())
	if err != nil {
		return nil, s.authError(err)
	}

	return &qav1.AuthReply{
		User:      toProtoUser(user),
		Token:     session.Token,
		ExpiresAt: session.ExpiresAt.UTC().Format(time.RFC3339Nano),
	}, nil
}

func (s *Server) Logout(ctx context.Context, req *qav1.LogoutRequest) (*qav1.Empty, error) {
	if req == nil {
		return nil, status.Error(codes.InvalidArgument, "request is required")
	}

	user, err := s.authService.Authenticate(ctx, req.GetToken())
	if err != nil {
		return nil, s.authError(err)
	}

	if err := s.authService.Logout(ctx, req.GetToken(), user.ID); err != nil {
		return nil, s.authError(err)
	}
	return &qav1.Empty{}, nil
}

func (s *Server) Me(ctx context.Context, req *qav1.MeRequest) (*qav1.AuthReply, error) {
	user, err := s.authenticate(ctx, tokenFromMeReq(req))
	if err != nil {
		return nil, err
	}
	return &qav1.AuthReply{User: toProtoUser(user)}, nil
}

func (s *Server) UploadDocument(ctx context.Context, req *qav1.UploadDocumentRequest) (*qav1.DocumentReply, error) {
	if req == nil {
		return nil, status.Error(codes.InvalidArgument, "request is required")
	}
	user, err := s.authenticate(ctx, req.GetToken())
	if err != nil {
		return nil, err
	}

	if err := validateUpload(req.GetFilename(), len(req.GetContent())); err != nil {
		return nil, status.Error(codes.InvalidArgument, err.Error())
	}

	docID := types.NewID("doc")
	ext := strings.ToLower(filepath.Ext(req.GetFilename()))
	if ext == "" {
		ext = ".txt"
	}
	objectKey := user.ID + "/" + docID + ext
	contentType := inferContentType(req.GetFilename(), req.GetMimeType())
	if err := s.objectStore.PutObject(ctx, objectKey, req.GetContent(), contentType); err != nil {
		s.logger.Printf("upload object error: %v", err)
		return nil, status.Error(codes.Internal, "failed to store object")
	}

	now := time.Now().UTC()
	doc := types.Document{
		ID:            docID,
		OwnerUserID:   user.ID,
		Name:          req.GetFilename(),
		SizeBytes:     int64(len(req.GetContent())),
		MimeType:      contentType,
		StoragePath:   objectKey,
		Status:        "indexing",
		ChunkCount:    0,
		CreatedAt:     now,
		LastUpdatedAt: now,
	}
	if err := s.store.UpsertDocument(doc, nil); err != nil {
		s.logger.Printf("persist document error: %v", err)
		return nil, status.Error(codes.Internal, "failed to persist document")
	}

	text, err := ingest.ParseDocumentText(req.GetFilename(), req.GetMimeType(), req.GetContent())
	if err != nil {
		doc.Status = "failed"
		doc.LastUpdatedAt = time.Now().UTC()
		_ = s.store.UpsertDocument(doc, nil)
		return nil, status.Error(codes.InvalidArgument, err.Error())
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
	if err := s.indexDocumentVectors(ctx, user.ID, doc, chunks); err != nil {
		s.logger.Printf("index vectors error: %v", err)
		doc.Status = "failed"
		doc.LastUpdatedAt = time.Now().UTC()
		_ = s.store.UpsertDocument(doc, nil)
		return nil, status.Error(codes.Internal, "failed to index document vectors")
	}

	doc.Status = "ready"
	doc.ChunkCount = len(chunks)
	doc.LastUpdatedAt = time.Now().UTC()
	if err := s.store.UpsertDocument(doc, chunks); err != nil {
		s.logger.Printf("persist chunks error: %v", err)
		return nil, status.Error(codes.Internal, "failed to persist chunks")
	}

	return &qav1.DocumentReply{Document: toProtoDocument(doc)}, nil
}

func (s *Server) ListDocuments(ctx context.Context, req *qav1.ListDocumentsRequest) (*qav1.ListDocumentsReply, error) {
	if req == nil {
		return nil, status.Error(codes.InvalidArgument, "request is required")
	}
	user, err := s.authenticate(ctx, req.GetToken())
	if err != nil {
		return nil, err
	}

	docs := s.store.ListDocumentsByOwner(user.ID)
	sort.Slice(docs, func(i, j int) bool {
		return docs[i].CreatedAt.After(docs[j].CreatedAt)
	})

	page := int(req.GetPage())
	if page <= 0 {
		page = 1
	}
	pageSize := int(req.GetPageSize())
	if pageSize <= 0 || pageSize > 100 {
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

	respDocs := make([]*qav1.Document, 0, end-start)
	for _, doc := range docs[start:end] {
		respDocs = append(respDocs, toProtoDocument(doc))
	}

	return &qav1.ListDocumentsReply{
		Documents: respDocs,
		Total:     int32(len(docs)),
		Page:      int32(page),
		PageSize:  int32(pageSize),
	}, nil
}

func (s *Server) GetDocument(ctx context.Context, req *qav1.DocumentRequest) (*qav1.DocumentReply, error) {
	doc, _, err := s.getOwnedDocument(ctx, req)
	if err != nil {
		return nil, err
	}
	return &qav1.DocumentReply{Document: toProtoDocument(doc)}, nil
}

func (s *Server) DownloadDocument(ctx context.Context, req *qav1.DocumentRequest) (*qav1.DownloadDocumentReply, error) {
	doc, _, err := s.getOwnedDocument(ctx, req)
	if err != nil {
		return nil, err
	}

	data, contentType, err := s.objectStore.GetObject(ctx, doc.StoragePath)
	if err != nil {
		s.logger.Printf("download object error: %v", err)
		return nil, status.Error(codes.Internal, "failed to read document")
	}
	if strings.TrimSpace(contentType) == "" {
		contentType = inferContentType(doc.Name, doc.MimeType)
	}

	return &qav1.DownloadDocumentReply{
		Content:     data,
		ContentType: contentType,
		Filename:    doc.Name,
	}, nil
}

func (s *Server) DeleteDocument(ctx context.Context, req *qav1.DeleteDocumentRequest) (*qav1.Empty, error) {
	if req == nil {
		return nil, status.Error(codes.InvalidArgument, "request is required")
	}
	if !req.GetConfirm() {
		return nil, status.Error(codes.InvalidArgument, "delete requires confirm=true")
	}

	doc, user, err := s.getOwnedDocument(ctx, &qav1.DocumentRequest{Token: req.GetToken(), DocumentId: req.GetDocumentId()})
	if err != nil {
		return nil, err
	}

	if err := s.objectStore.DeleteObject(ctx, doc.StoragePath); err != nil {
		s.logger.Printf("delete object error: %v", err)
		return nil, status.Error(codes.Internal, "failed to delete document file")
	}
	if s.vectorIndexer != nil {
		if err := s.vectorIndexer.DeleteDocument(ctx, user.ID, doc.ID); err != nil {
			s.logger.Printf("delete vectors warning: %v", err)
		}
	}
	if err := s.store.DeleteDocument(doc.ID); err != nil {
		s.logger.Printf("delete document error: %v", err)
		return nil, status.Error(codes.Internal, "failed to delete document")
	}
	s.store.RecordAudit("document.delete", user.ID, doc.ID, map[string]interface{}{"name": doc.Name})
	return &qav1.Empty{}, nil
}

func (s *Server) ListThreads(ctx context.Context, req *qav1.ListThreadsRequest) (*qav1.ListThreadsReply, error) {
	if req == nil {
		return nil, status.Error(codes.InvalidArgument, "request is required")
	}
	user, err := s.authenticate(ctx, req.GetToken())
	if err != nil {
		return nil, err
	}

	threads := s.store.ListThreadsByOwner(user.ID)
	sort.Slice(threads, func(i, j int) bool {
		return threads[i].CreatedAt.After(threads[j].CreatedAt)
	})

	out := make([]*qav1.Thread, 0, len(threads))
	for _, thread := range threads {
		out = append(out, toProtoThread(thread))
	}
	return &qav1.ListThreadsReply{Threads: out}, nil
}

func (s *Server) CreateThread(ctx context.Context, req *qav1.CreateThreadRequest) (*qav1.ThreadReply, error) {
	if req == nil {
		return nil, status.Error(codes.InvalidArgument, "request is required")
	}
	user, err := s.authenticate(ctx, req.GetToken())
	if err != nil {
		return nil, err
	}

	title := strings.TrimSpace(req.GetTitle())
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
		s.logger.Printf("create thread error: %v", err)
		return nil, status.Error(codes.Internal, "failed to create thread")
	}

	return &qav1.ThreadReply{Thread: toProtoThread(thread)}, nil
}

func (s *Server) CreateTurn(ctx context.Context, req *qav1.CreateTurnRequest) (*qav1.CreateTurnReply, error) {
	if req == nil {
		return nil, status.Error(codes.InvalidArgument, "request is required")
	}
	user, err := s.authenticate(ctx, req.GetToken())
	if err != nil {
		return nil, err
	}

	thread, err := s.getOwnedThread(req.GetThreadId(), user.ID)
	if err != nil {
		return nil, err
	}

	message := strings.TrimSpace(req.GetMessage())
	if message == "" {
		return nil, status.Error(codes.InvalidArgument, "message is required")
	}

	ownedDocs := s.store.ListDocumentsByOwner(user.ID)
	scope, err := qa.ResolveScope(message, strings.ToLower(strings.TrimSpace(req.GetScopeType())), req.GetScopeDocIds(), ownedDocs)
	if err != nil {
		return nil, status.Error(codes.InvalidArgument, err.Error())
	}

	selectedDocs := resolveSelectedDocs(scope, ownedDocs)
	if scope.Type == "doc" && len(selectedDocs) == 0 {
		return nil, status.Error(codes.InvalidArgument, "selected documents do not exist or are not owned by current user")
	}

	chunkMap := make(map[string][]types.Chunk, len(selectedDocs))
	for _, doc := range selectedDocs {
		chunkMap[doc.ID] = s.store.GetChunksForDoc(doc.ID)
	}
	retrieved := s.retrieveChunks(ctx, user.ID, scope.QuestionBody, selectedDocs, chunkMap, 5)
	citations := buildCitations(retrieved)

	previousTurns := s.store.ListTurnsByThread(thread.ID)
	sort.Slice(previousTurns, func(i, j int) bool {
		return previousTurns[i].CreatedAt.Before(previousTurns[j].CreatedAt)
	})

	now := time.Now().UTC()
	turn := types.Turn{
		ID:          types.NewID("turn"),
		ThreadID:    thread.ID,
		OwnerUserID: user.ID,
		Question:    scope.QuestionBody,
		Status:      "completed",
		ScopeType:   scope.Type,
		ScopeDocIDs: scope.DocIDs,
		CreatedAt:   now,
		UpdatedAt:   now,
	}

	activeProvider := s.store.GetProvider().ActiveProvider
	answer, err := s.generateAnswer(ctx, user.ID, turn, retrieved, previousTurns, activeProvider)
	if err != nil {
		return nil, err
	}
	turn.Answer = answer

	items := []types.TurnItem{
		{
			ID:       types.NewID("item"),
			TurnID:   turn.ID,
			ItemType: "message",
			Payload: map[string]interface{}{
				"question":      turn.Question,
				"scope_type":    turn.ScopeType,
				"scope_doc_ids": turn.ScopeDocIDs,
			},
			CreatedAt: now,
		},
		{
			ID:       types.NewID("item"),
			TurnID:   turn.ID,
			ItemType: "retrieval",
			Payload: map[string]interface{}{
				"count":     len(citations),
				"citations": citations,
			},
			CreatedAt: now.Add(1 * time.Millisecond),
		},
		{
			ID:       types.NewID("item"),
			TurnID:   turn.ID,
			ItemType: "final",
			Payload: map[string]interface{}{
				"answer":    answer,
				"citations": citations,
			},
			CreatedAt: now.Add(2 * time.Millisecond),
		},
	}
	if err := s.store.CreateOrUpdateTurn(turn, items); err != nil {
		s.logger.Printf("save turn error: %v", err)
		return nil, status.Error(codes.Internal, "failed to save turn")
	}

	protoCitations := make([]*qav1.Citation, 0, len(citations))
	for _, citation := range citations {
		protoCitations = append(protoCitations, &qav1.Citation{
			DocId:      citation.DocID,
			DocName:    citation.DocName,
			ChunkId:    citation.ChunkID,
			Excerpt:    citation.Excerpt,
			Score:      int32(citation.Score),
			ChunkIndex: int32(citation.ChunkIdx),
		})
	}

	protoItems := make([]*qav1.TurnItem, 0, len(items))
	for _, item := range items {
		protoItems = append(protoItems, toProtoTurnItem(item))
	}

	return &qav1.CreateTurnReply{
		Turn:      toProtoTurn(turn),
		Citations: protoCitations,
		Items:     protoItems,
	}, nil
}

func (s *Server) GetTurn(ctx context.Context, req *qav1.GetTurnRequest) (*qav1.GetTurnReply, error) {
	if req == nil {
		return nil, status.Error(codes.InvalidArgument, "request is required")
	}
	user, err := s.authenticate(ctx, req.GetToken())
	if err != nil {
		return nil, err
	}

	thread, err := s.getOwnedThread(req.GetThreadId(), user.ID)
	if err != nil {
		return nil, err
	}

	turn, ok := s.store.GetTurn(req.GetTurnId())
	if !ok || turn.ThreadID != thread.ID {
		return nil, status.Error(codes.NotFound, "turn not found")
	}
	if turn.OwnerUserID != user.ID {
		s.store.RecordAudit("auth.unauthorized", user.ID, turn.ID, map[string]interface{}{"resource": "turn"})
		return nil, status.Error(codes.PermissionDenied, "turn does not belong to current user")
	}

	items := s.store.GetTurnItems(turn.ID)
	protoItems := make([]*qav1.TurnItem, 0, len(items))
	for _, item := range items {
		protoItems = append(protoItems, toProtoTurnItem(item))
	}

	return &qav1.GetTurnReply{Turn: toProtoTurn(turn), Items: protoItems}, nil
}

func (s *Server) GetConfig(ctx context.Context, req *qav1.MeRequest) (*qav1.ConfigReply, error) {
	user, err := s.authenticate(ctx, tokenFromMeReq(req))
	if err != nil {
		return nil, err
	}

	_ = user
	provider := s.store.GetProvider()
	return &qav1.ConfigReply{ActiveProvider: provider.ActiveProvider, Available: provider.Available}, nil
}

func (s *Server) SetConfig(ctx context.Context, req *qav1.SetConfigRequest) (*qav1.ConfigReply, error) {
	if req == nil {
		return nil, status.Error(codes.InvalidArgument, "request is required")
	}
	user, err := s.authenticate(ctx, req.GetToken())
	if err != nil {
		return nil, err
	}

	active := strings.TrimSpace(req.GetActiveProvider())
	if active == "" {
		return nil, status.Error(codes.InvalidArgument, "active_provider is required")
	}
	if err := s.store.SetProvider(active, user.ID); err != nil {
		return nil, status.Error(codes.InvalidArgument, err.Error())
	}

	provider := s.store.GetProvider()
	return &qav1.ConfigReply{ActiveProvider: provider.ActiveProvider, Available: provider.Available}, nil
}

func (s *Server) getOwnedDocument(ctx context.Context, req *qav1.DocumentRequest) (types.Document, domainauth.User, error) {
	if req == nil {
		return types.Document{}, domainauth.User{}, status.Error(codes.InvalidArgument, "request is required")
	}
	user, err := s.authenticate(ctx, req.GetToken())
	if err != nil {
		return types.Document{}, domainauth.User{}, err
	}

	docID := strings.TrimSpace(req.GetDocumentId())
	if docID == "" {
		return types.Document{}, domainauth.User{}, status.Error(codes.InvalidArgument, "document_id is required")
	}
	doc, ok := s.store.GetDocument(docID)
	if !ok {
		return types.Document{}, domainauth.User{}, status.Error(codes.NotFound, "document not found")
	}
	if doc.OwnerUserID != user.ID {
		s.store.RecordAudit("auth.unauthorized", user.ID, docID, map[string]interface{}{"resource": "document"})
		return types.Document{}, domainauth.User{}, status.Error(codes.PermissionDenied, "document does not belong to current user")
	}
	return doc, user, nil
}

func (s *Server) getOwnedThread(threadID, ownerID string) (types.Thread, error) {
	threadID = strings.TrimSpace(threadID)
	if threadID == "" {
		return types.Thread{}, status.Error(codes.InvalidArgument, "thread_id is required")
	}

	thread, ok := s.store.GetThread(threadID)
	if !ok {
		return types.Thread{}, status.Error(codes.NotFound, "thread not found")
	}
	if thread.OwnerUserID != ownerID {
		s.store.RecordAudit("auth.unauthorized", ownerID, threadID, map[string]interface{}{"resource": "thread"})
		return types.Thread{}, status.Error(codes.PermissionDenied, "thread does not belong to current user")
	}
	return thread, nil
}

func (s *Server) generateAnswer(
	ctx context.Context,
	ownerID string,
	turn types.Turn,
	retrieved []qa.ScoredChunk,
	previousTurns []types.Turn,
	activeProvider string,
) (string, error) {
	if s.answerGenerator == nil {
		return "", status.Error(codes.FailedPrecondition, "LLM generator is not configured")
	}

	contexts := make([]llm.ContextChunk, 0, len(retrieved))
	for _, item := range retrieved {
		contexts = append(contexts, llm.ContextChunk{
			DocID:      item.Document.ID,
			DocName:    item.Document.Name,
			ChunkID:    item.Chunk.ID,
			ChunkIndex: item.Chunk.Index,
			Score:      item.Score,
			Content:    item.Chunk.Content,
		})
	}

	llmReq := llm.Request{
		OwnerUserID:    ownerID,
		ThreadID:       turn.ThreadID,
		TurnID:         turn.ID,
		Question:       turn.Question,
		ScopeType:      turn.ScopeType,
		ScopeDocIDs:    append([]string(nil), turn.ScopeDocIDs...),
		Contexts:       contexts,
		ActiveProvider: strings.TrimSpace(activeProvider),
	}
	if len(previousTurns) > 0 {
		last := previousTurns[len(previousTurns)-1]
		llmReq.PreviousTurnQuestion = last.Question
		llmReq.PreviousTurnAnswer = last.Answer
	}

	answer, err := s.answerGenerator.GenerateAnswer(ctx, llmReq)
	if err != nil {
		s.logger.Printf("llm generate failed: %v", err)
		if errors.Is(err, llm.ErrUnavailable) {
			return "", status.Error(codes.FailedPrecondition, "LLM provider is unavailable: check provider selection and API key")
		}
		msg := strings.ToLower(err.Error())
		if strings.Contains(msg, "api_key") || strings.Contains(msg, "api key") || strings.Contains(msg, "unauthorized") || strings.Contains(msg, "401") {
			return "", status.Error(codes.FailedPrecondition, "LLM provider authentication/config error: "+err.Error())
		}
		return "", status.Error(codes.Unavailable, "LLM provider call failed: "+err.Error())
	}
	answer = strings.TrimSpace(answer)
	if answer == "" {
		return "", status.Error(codes.Unavailable, "LLM provider returned empty answer")
	}
	return answer, nil
}

func (s *Server) authenticate(ctx context.Context, token string) (domainauth.User, error) {
	token = strings.TrimSpace(token)
	if token == "" {
		return domainauth.User{}, status.Error(codes.Unauthenticated, "missing token")
	}
	user, err := s.authService.Authenticate(ctx, token)
	if err != nil {
		return domainauth.User{}, s.authError(err)
	}
	return user, nil
}

func (s *Server) authError(err error) error {
	switch {
	case errors.Is(err, authapp.ErrInvalidEmail):
		return status.Error(codes.InvalidArgument, "email is required")
	case errors.Is(err, authapp.ErrInvalidPassword):
		return status.Error(codes.InvalidArgument, "password must satisfy minimum policy")
	case errors.Is(err, authapp.ErrEmailAlreadyExists):
		return status.Error(codes.AlreadyExists, "email already registered")
	case errors.Is(err, authapp.ErrInvalidCredentials):
		return status.Error(codes.Unauthenticated, "email or password is incorrect")
	case errors.Is(err, authapp.ErrUnauthorized):
		return status.Error(codes.Unauthenticated, "unauthorized")
	default:
		s.logger.Printf("auth service error: %v", err)
		return status.Error(codes.Internal, "internal server error")
	}
}

func validateUpload(filename string, size int) error {
	filename = strings.TrimSpace(filename)
	if filename == "" {
		return errors.New("filename is required")
	}
	ext := strings.ToLower(filepath.Ext(filename))
	switch ext {
	case ".txt", ".md", ".markdown", ".pdf":
	default:
		return errors.New("only TXT, Markdown, and PDF are supported")
	}
	if size <= 0 {
		return errors.New("file content is empty")
	}
	if size > maxUploadSize {
		return errors.New("file exceeds 10MB limit")
	}
	return nil
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

func fallbackAnswer(question string, retrieved []qa.ScoredChunk, previousTurns []types.Turn) string {
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

func (s *Server) indexDocumentVectors(ctx context.Context, ownerID string, doc types.Document, chunks []types.Chunk) error {
	if s.embedder == nil || s.vectorIndexer == nil || len(chunks) == 0 {
		return nil
	}

	texts := make([]string, 0, len(chunks))
	for _, chunk := range chunks {
		texts = append(texts, chunk.Content)
	}
	vectors, err := s.embedder.Embed(ctx, texts)
	if err != nil {
		return err
	}
	if len(vectors) != len(chunks) {
		return fmt.Errorf("vectorization count mismatch: chunks=%d vectors=%d", len(chunks), len(vectors))
	}
	return s.vectorIndexer.UpsertDocumentChunks(ctx, ownerID, doc, chunks, vectors)
}

func (s *Server) retrieveChunks(
	ctx context.Context,
	ownerID string,
	question string,
	selectedDocs []types.Document,
	chunkMap map[string][]types.Chunk,
	topK int,
) []qa.ScoredChunk {
	if topK <= 0 {
		topK = 5
	}

	// Default lexical retrieval path.
	fallback := func() []qa.ScoredChunk {
		return qa.RetrieveTopChunks(question, selectedDocs, chunkMap, topK)
	}
	if len(selectedDocs) == 0 {
		return nil
	}
	if s.embedder == nil || s.vectorIndexer == nil {
		return fallback()
	}

	queryVecs, err := s.embedder.Embed(ctx, []string{question})
	if err != nil || len(queryVecs) != 1 {
		if err != nil {
			s.logger.Printf("embed query warning: %v", err)
		}
		return fallback()
	}

	// Ask Qdrant for larger candidate pool then filter to scope docs.
	hits, err := s.vectorIndexer.SearchChunks(ctx, ownerID, queryVecs[0], topK*8)
	if err != nil {
		s.logger.Printf("vector search warning: %v", err)
		return fallback()
	}
	if len(hits) == 0 {
		return fallback()
	}

	docByID := make(map[string]types.Document, len(selectedDocs))
	allowedDocID := make(map[string]struct{}, len(selectedDocs))
	for _, doc := range selectedDocs {
		docByID[doc.ID] = doc
		allowedDocID[doc.ID] = struct{}{}
	}
	chunkByID := make(map[string]types.Chunk, 32)
	for _, chunks := range chunkMap {
		for _, chunk := range chunks {
			chunkByID[chunk.ID] = chunk
		}
	}

	retrieved := make([]qa.ScoredChunk, 0, topK)
	seenChunk := make(map[string]struct{}, topK*2)
	for _, hit := range hits {
		if _, ok := allowedDocID[hit.DocID]; !ok {
			continue
		}
		if _, exists := seenChunk[hit.ChunkID]; exists {
			continue
		}
		doc, ok := docByID[hit.DocID]
		if !ok {
			continue
		}
		chunk, ok := chunkByID[hit.ChunkID]
		if !ok {
			chunk = types.Chunk{
				ID:      hit.ChunkID,
				DocID:   hit.DocID,
				Index:   hit.ChunkIndex,
				Content: hit.Content,
			}
		}
		retrieved = append(retrieved, qa.ScoredChunk{
			Chunk:    chunk,
			Document: doc,
			Score:    int(hit.Score * 1000),
		})
		seenChunk[hit.ChunkID] = struct{}{}
		if len(retrieved) >= topK {
			break
		}
	}
	if len(retrieved) == 0 {
		return fallback()
	}
	return retrieved
}

func truncate(s string, n int) string {
	r := []rune(strings.TrimSpace(s))
	if len(r) <= n {
		return string(r)
	}
	return string(r[:n]) + "..."
}

func toProtoUser(user domainauth.User) *qav1.User {
	return &qav1.User{Id: user.ID, Email: user.Email, CreatedAt: user.CreatedAt.UTC().Format(time.RFC3339Nano)}
}

func toProtoDocument(doc types.Document) *qav1.Document {
	return &qav1.Document{
		Id:            doc.ID,
		OwnerUserId:   doc.OwnerUserID,
		Name:          doc.Name,
		SizeBytes:     doc.SizeBytes,
		MimeType:      doc.MimeType,
		StoragePath:   doc.StoragePath,
		Status:        doc.Status,
		ChunkCount:    int32(doc.ChunkCount),
		CreatedAt:     doc.CreatedAt.UTC().Format(time.RFC3339Nano),
		LastUpdatedAt: doc.LastUpdatedAt.UTC().Format(time.RFC3339Nano),
	}
}

func toProtoThread(thread types.Thread) *qav1.Thread {
	return &qav1.Thread{
		Id:          thread.ID,
		OwnerUserId: thread.OwnerUserID,
		Title:       thread.Title,
		CreatedAt:   thread.CreatedAt.UTC().Format(time.RFC3339Nano),
	}
}

func toProtoTurn(turn types.Turn) *qav1.Turn {
	return &qav1.Turn{
		Id:          turn.ID,
		ThreadId:    turn.ThreadID,
		OwnerUserId: turn.OwnerUserID,
		Question:    turn.Question,
		Answer:      turn.Answer,
		Status:      turn.Status,
		ScopeType:   turn.ScopeType,
		ScopeDocIds: append([]string(nil), turn.ScopeDocIDs...),
		CreatedAt:   turn.CreatedAt.UTC().Format(time.RFC3339Nano),
		UpdatedAt:   turn.UpdatedAt.UTC().Format(time.RFC3339Nano),
	}
}

func toProtoTurnItem(item types.TurnItem) *qav1.TurnItem {
	payload, _ := json.Marshal(item.Payload)
	return &qav1.TurnItem{
		Id:          item.ID,
		TurnId:      item.TurnID,
		ItemType:    item.ItemType,
		PayloadJson: string(payload),
		CreatedAt:   item.CreatedAt.UTC().Format(time.RFC3339Nano),
	}
}

func tokenFromMeReq(req *qav1.MeRequest) string {
	if req == nil {
		return ""
	}
	return req.GetToken()
}
