package auth

import (
	"context"
	"errors"
	"testing"
	"time"

	"llm-doc-qa-assistant/backend/internal/infrastructure/memory"
	"llm-doc-qa-assistant/backend/internal/infrastructure/security"
)

type auditSink struct{}

func (auditSink) RecordAudit(eventType, actorID, targetID string, metadata map[string]interface{}) {
	_ = eventType
	_ = actorID
	_ = targetID
	_ = metadata
}

func TestRegisterAndLogin(t *testing.T) {
	userRepo, sessionRepo := memory.NewAuthRepositories()
	svc := NewService(userRepo, sessionRepo, security.PasswordHasher{}, security.TokenGenerator{}, auditSink{})

	user, err := svc.Register(context.Background(), "Test@Example.Com", "password-123")
	if err != nil {
		t.Fatalf("register error: %v", err)
	}
	if user.Email != "test@example.com" {
		t.Fatalf("expected normalized email, got %s", user.Email)
	}

	_, session, err := svc.Login(context.Background(), "test@example.com", "password-123")
	if err != nil {
		t.Fatalf("login error: %v", err)
	}
	if session.Token == "" {
		t.Fatalf("expected non-empty token")
	}
}

func TestRegisterDuplicateEmail(t *testing.T) {
	userRepo, sessionRepo := memory.NewAuthRepositories()
	svc := NewService(userRepo, sessionRepo, security.PasswordHasher{}, security.TokenGenerator{}, auditSink{})

	_, err := svc.Register(context.Background(), "dup@example.com", "password-123")
	if err != nil {
		t.Fatalf("first register error: %v", err)
	}
	_, err = svc.Register(context.Background(), "dup@example.com", "password-123")
	if !errors.Is(err, ErrEmailAlreadyExists) {
		t.Fatalf("expected ErrEmailAlreadyExists, got %v", err)
	}
}

func TestAuthenticateExpiredSession(t *testing.T) {
	userRepo, sessionRepo := memory.NewAuthRepositories()
	svc := NewService(userRepo, sessionRepo, security.PasswordHasher{}, security.TokenGenerator{}, auditSink{})
	svc.sessionTTL = time.Minute

	_, err := svc.Register(context.Background(), "exp@example.com", "password-123")
	if err != nil {
		t.Fatalf("register error: %v", err)
	}
	_, session, err := svc.Login(context.Background(), "exp@example.com", "password-123")
	if err != nil {
		t.Fatalf("login error: %v", err)
	}

	svc.now = func() time.Time { return time.Now().UTC().Add(2 * time.Minute) }
	_, err = svc.Authenticate(context.Background(), session.Token)
	if !errors.Is(err, ErrUnauthorized) {
		t.Fatalf("expected ErrUnauthorized for expired session, got %v", err)
	}
}
