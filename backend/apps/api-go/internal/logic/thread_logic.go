package logic

import (
	"context"
	"time"

	"llm-doc-qa-assistant/backend/apps/api-go/internal/svc"
	qav1 "llm-doc-qa-assistant/backend/proto/gen/go/qa/v1"
)

type ThreadLogic struct {
	svcCtx *svc.ServiceContext
}

func NewThreadLogic(svcCtx *svc.ServiceContext) *ThreadLogic {
	return &ThreadLogic{svcCtx: svcCtx}
}

func (l *ThreadLogic) ListThreads(ctx context.Context, token string) (*qav1.ListThreadsReply, error) {
	ctx, cancel := context.WithTimeout(ctx, 6*time.Second)
	defer cancel()
	return l.svcCtx.Core.ListThreads(ctx, &qav1.ListThreadsRequest{Token: token})
}

func (l *ThreadLogic) CreateThread(ctx context.Context, token, title string) (*qav1.ThreadReply, error) {
	ctx, cancel := context.WithTimeout(ctx, 8*time.Second)
	defer cancel()
	return l.svcCtx.Core.CreateThread(ctx, &qav1.CreateThreadRequest{Token: token, Title: title})
}

func (l *ThreadLogic) CreateTurn(ctx context.Context, token, threadID, message, scopeType string, scopeDocIDs []string, thinkMode bool) (*qav1.CreateTurnReply, error) {
	_ = thinkMode
	ctx, cancel := context.WithTimeout(ctx, 40*time.Second)
	defer cancel()
	return l.svcCtx.Core.CreateTurn(ctx, &qav1.CreateTurnRequest{
		Token:       token,
		ThreadId:    threadID,
		Message:     message,
		ScopeType:   scopeType,
		ScopeDocIds: scopeDocIDs,
		ThinkMode:   false,
	})
}

func (l *ThreadLogic) ListTurns(ctx context.Context, token, threadID string) (*qav1.ListTurnsReply, error) {
	ctx, cancel := context.WithTimeout(ctx, 20*time.Second)
	defer cancel()
	return l.svcCtx.Core.ListTurns(ctx, &qav1.ListTurnsRequest{Token: token, ThreadId: threadID})
}

func (l *ThreadLogic) CreateTurnStream(ctx context.Context, token, threadID, message, scopeType string, scopeDocIDs []string, thinkMode bool) (qav1.CoreService_CreateTurnStreamClient, error) {
	_ = thinkMode
	return l.svcCtx.Core.CreateTurnStream(ctx, &qav1.CreateTurnRequest{
		Token:       token,
		ThreadId:    threadID,
		Message:     message,
		ScopeType:   scopeType,
		ScopeDocIds: scopeDocIDs,
		ThinkMode:   false,
	})
}

func (l *ThreadLogic) GetTurn(ctx context.Context, token, threadID, turnID string) (*qav1.GetTurnReply, error) {
	ctx, cancel := context.WithTimeout(ctx, 20*time.Second)
	defer cancel()
	return l.svcCtx.Core.GetTurn(ctx, &qav1.GetTurnRequest{Token: token, ThreadId: threadID, TurnId: turnID})
}
