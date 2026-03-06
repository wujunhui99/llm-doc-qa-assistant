package auth

import (
	"context"
	"errors"
	"strings"
	"time"
)

var (
	ErrNotFound          = errors.New("not found")
	ErrEmailAlreadyExist = errors.New("email already exists")
)

type User struct {
	ID           string
	Email        string
	PasswordHash string
	CreatedAt    time.Time
}

type Session struct {
	Token     string
	UserID    string
	CreatedAt time.Time
	ExpiresAt time.Time
}

type UserRepository interface {
	Create(ctx context.Context, user User) error
	GetByEmail(ctx context.Context, email string) (User, error)
	GetByID(ctx context.Context, id string) (User, error)
}

type SessionRepository interface {
	Create(ctx context.Context, session Session) error
	GetByToken(ctx context.Context, token string) (Session, error)
	DeleteByToken(ctx context.Context, token string) error
}

func NormalizeEmail(email string) string {
	return strings.ToLower(strings.TrimSpace(email))
}
