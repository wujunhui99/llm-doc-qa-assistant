package svc

import (
	"log"

	qav1 "llm-doc-qa-assistant/backend/proto/gen/go/qa/v1"
)

type ServiceContext struct {
	Config Config
	Core   qav1.CoreServiceClient
	Logger *log.Logger
}

func NewServiceContext(cfg Config, core qav1.CoreServiceClient, logger *log.Logger) *ServiceContext {
	if core == nil {
		panic("core client cannot be nil")
	}
	if logger == nil {
		logger = log.New(log.Writer(), "[api-go] ", log.LstdFlags|log.LUTC)
	}
	return &ServiceContext{
		Config: cfg,
		Core:   core,
		Logger: logger,
	}
}
