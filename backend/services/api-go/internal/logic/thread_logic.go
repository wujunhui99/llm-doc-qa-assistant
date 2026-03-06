package logic

import (
	"context"
	"time"

	qav1 "llm-doc-qa-assistant/backend/proto/gen/go/qa/v1"
	"llm-doc-qa-assistant/backend/services/api-go/internal/svc"
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

func (l *ThreadLogic) CreateTurn(ctx context.Context, token, threadID, message, scopeType string, scopeDocIDs []string) (*qav1.CreateTurnReply, error) {
	ctx, cancel := context.WithTimeout(ctx, 40*time.Second)
	defer cancel()
	return l.svcCtx.Core.CreateTurn(ctx, &qav1.CreateTurnRequest{Token: token, ThreadId: threadID, Message: message, ScopeType: scopeType, ScopeDocIds: scopeDocIDs})
}

func (l *ThreadLogic) GetTurn(ctx context.Context, token, threadID, turnID string) (*qav1.GetTurnReply, error) {
	ctx, cancel := context.WithTimeout(ctx, 20*time.Second)
	defer cancel()
	return l.svcCtx.Core.GetTurn(ctx, &qav1.GetTurnRequest{Token: token, ThreadId: threadID, TurnId: turnID})
}
