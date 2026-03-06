package auth

import (
	"context"
	"errors"
	"time"

	domainauth "llm-doc-qa-assistant/backend/internal/domain/auth"
	"llm-doc-qa-assistant/backend/internal/types"
)

var (
	ErrInvalidEmail       = errors.New("invalid email")
	ErrInvalidPassword    = errors.New("invalid password")
	ErrEmailAlreadyExists = errors.New("email already exists")
	ErrInvalidCredentials = errors.New("invalid credentials")
	ErrUnauthorized       = errors.New("unauthorized")
)

type PasswordHasher interface {
	HashPassword(password string) (string, error)
	VerifyPassword(password, encoded string) bool
}

type SessionTokenGenerator interface {
	NewSessionToken() (string, error)
}

type AuditLogger interface {
	RecordAudit(eventType, actorID, targetID string, metadata map[string]interface{})
}

type Service struct {
	users      domainauth.UserRepository
	sessions   domainauth.SessionRepository
	hasher     PasswordHasher
	tokenGen   SessionTokenGenerator
	audit      AuditLogger
	now        func() time.Time
	sessionTTL time.Duration
}

func NewService(
	users domainauth.UserRepository,
	sessions domainauth.SessionRepository,
	hasher PasswordHasher,
	tokenGen SessionTokenGenerator,
	audit AuditLogger,
) *Service {
	return &Service{
		users:      users,
		sessions:   sessions,
		hasher:     hasher,
		tokenGen:   tokenGen,
		audit:      audit,
		now:        func() time.Time { return time.Now().UTC() },
		sessionTTL: 24 * time.Hour,
	}
}

func (s *Service) Register(ctx context.Context, email, password string) (domainauth.User, error) {
	email = domainauth.NormalizeEmail(email)
	if email == "" {
		return domainauth.User{}, ErrInvalidEmail
	}

	hash, err := s.hasher.HashPassword(password)
	if err != nil {
		return domainauth.User{}, ErrInvalidPassword
	}

	user := domainauth.User{
		ID:           types.NewID("usr"),
		Email:        email,
		PasswordHash: hash,
		CreatedAt:    s.now(),
	}

	if err := s.users.Create(ctx, user); err != nil {
		if errors.Is(err, domainauth.ErrEmailAlreadyExist) {
			return domainauth.User{}, ErrEmailAlreadyExists
		}
		return domainauth.User{}, err
	}

	s.audit.RecordAudit("auth.register", user.ID, user.ID, nil)
	return user, nil
}

func (s *Service) Login(ctx context.Context, email, password string) (domainauth.User, domainauth.Session, error) {
	email = domainauth.NormalizeEmail(email)
	user, err := s.users.GetByEmail(ctx, email)
	if err != nil {
		s.audit.RecordAudit("auth.login_failed", "", email, nil)
		if errors.Is(err, domainauth.ErrNotFound) {
			return domainauth.User{}, domainauth.Session{}, ErrInvalidCredentials
		}
		return domainauth.User{}, domainauth.Session{}, err
	}

	if !s.hasher.VerifyPassword(password, user.PasswordHash) {
		s.audit.RecordAudit("auth.login_failed", "", email, nil)
		return domainauth.User{}, domainauth.Session{}, ErrInvalidCredentials
	}

	token, err := s.tokenGen.NewSessionToken()
	if err != nil {
		return domainauth.User{}, domainauth.Session{}, err
	}
	now := s.now()
	session := domainauth.Session{
		Token:     token,
		UserID:    user.ID,
		CreatedAt: now,
		ExpiresAt: now.Add(s.sessionTTL),
	}
	if err := s.sessions.Create(ctx, session); err != nil {
		return domainauth.User{}, domainauth.Session{}, err
	}

	s.audit.RecordAudit("auth.login_success", user.ID, user.ID, nil)
	return user, session, nil
}

func (s *Service) Logout(ctx context.Context, token, actorID string) error {
	if token == "" {
		return ErrUnauthorized
	}
	if err := s.sessions.DeleteByToken(ctx, token); err != nil {
		return err
	}
	s.audit.RecordAudit("auth.logout", actorID, actorID, nil)
	return nil
}

func (s *Service) Authenticate(ctx context.Context, token string) (domainauth.User, error) {
	if token == "" {
		return domainauth.User{}, ErrUnauthorized
	}

	session, err := s.sessions.GetByToken(ctx, token)
	if err != nil {
		if errors.Is(err, domainauth.ErrNotFound) {
			return domainauth.User{}, ErrUnauthorized
		}
		return domainauth.User{}, err
	}
	if session.ExpiresAt.Before(s.now()) {
		_ = s.sessions.DeleteByToken(ctx, token)
		return domainauth.User{}, ErrUnauthorized
	}

	user, err := s.users.GetByID(ctx, session.UserID)
	if err != nil {
		if errors.Is(err, domainauth.ErrNotFound) {
			return domainauth.User{}, ErrUnauthorized
		}
		return domainauth.User{}, err
	}
	return user, nil
}
