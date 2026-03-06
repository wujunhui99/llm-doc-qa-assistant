package logic

import (
	"context"
	"time"

	qav1 "llm-doc-qa-assistant/backend/proto/gen/go/qa/v1"
	"llm-doc-qa-assistant/backend/apps/api-go/internal/svc"
)

type AuthLogic struct {
	svcCtx *svc.ServiceContext
}

func NewAuthLogic(svcCtx *svc.ServiceContext) *AuthLogic {
	return &AuthLogic{svcCtx: svcCtx}
}

func (l *AuthLogic) Register(ctx context.Context, email, password string) (*qav1.AuthReply, error) {
	ctx, cancel := context.WithTimeout(ctx, 8*time.Second)
	defer cancel()
	return l.svcCtx.Core.Register(ctx, &qav1.RegisterRequest{Email: email, Password: password})
}

func (l *AuthLogic) Login(ctx context.Context, email, password string) (*qav1.AuthReply, error) {
	ctx, cancel := context.WithTimeout(ctx, 8*time.Second)
	defer cancel()
	return l.svcCtx.Core.Login(ctx, &qav1.LoginRequest{Email: email, Password: password})
}

func (l *AuthLogic) Logout(ctx context.Context, token string) error {
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	_, err := l.svcCtx.Core.Logout(ctx, &qav1.LogoutRequest{Token: token})
	return err
}

func (l *AuthLogic) Me(ctx context.Context, token string) (*qav1.AuthReply, error) {
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	return l.svcCtx.Core.Me(ctx, &qav1.MeRequest{Token: token})
}
