package logic

import (
	"context"
	"time"

	qav1 "llm-doc-qa-assistant/backend/proto/gen/go/qa/v1"
	"llm-doc-qa-assistant/backend/apps/api-go/internal/svc"
)

type ConfigLogic struct {
	svcCtx *svc.ServiceContext
}

func NewConfigLogic(svcCtx *svc.ServiceContext) *ConfigLogic {
	return &ConfigLogic{svcCtx: svcCtx}
}

func (l *ConfigLogic) GetConfig(ctx context.Context, token string) (*qav1.ConfigReply, error) {
	ctx, cancel := context.WithTimeout(ctx, 8*time.Second)
	defer cancel()
	return l.svcCtx.Core.GetConfig(ctx, &qav1.MeRequest{Token: token})
}

func (l *ConfigLogic) SetConfig(ctx context.Context, token, activeProvider string) (*qav1.ConfigReply, error) {
	ctx, cancel := context.WithTimeout(ctx, 8*time.Second)
	defer cancel()
	return l.svcCtx.Core.SetConfig(ctx, &qav1.SetConfigRequest{Token: token, ActiveProvider: activeProvider})
}

func (l *ConfigLogic) Health(ctx context.Context) (*qav1.HealthReply, error) {
	ctx, cancel := context.WithTimeout(ctx, 3*time.Second)
	defer cancel()
	return l.svcCtx.Core.Health(ctx, &qav1.Empty{})
}
