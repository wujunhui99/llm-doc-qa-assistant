package mysql

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	domainauth "llm-doc-qa-assistant/backend/apps/core-go-rpc/internal/domain/auth"
)

type SessionRepository struct {
	db *sql.DB
}

func NewSessionRepository(db *sql.DB) *SessionRepository {
	return &SessionRepository{db: db}
}

func (r *SessionRepository) Create(ctx context.Context, session domainauth.Session) error {
	_, err := r.db.ExecContext(ctx,
		`INSERT INTO user_sessions (token, user_id, created_at, expires_at) VALUES (?, ?, ?, ?)`,
		session.Token, session.UserID, session.CreatedAt, session.ExpiresAt,
	)
	if err != nil {
		return fmt.Errorf("insert session: %w", err)
	}
	return nil
}

func (r *SessionRepository) GetByToken(ctx context.Context, token string) (domainauth.Session, error) {
	var session domainauth.Session
	err := r.db.QueryRowContext(ctx,
		`SELECT token, user_id, created_at, expires_at FROM user_sessions WHERE token = ? LIMIT 1`, token,
	).Scan(&session.Token, &session.UserID, &session.CreatedAt, &session.ExpiresAt)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return domainauth.Session{}, domainauth.ErrNotFound
		}
		return domainauth.Session{}, fmt.Errorf("get session by token: %w", err)
	}
	return session, nil
}

func (r *SessionRepository) DeleteByToken(ctx context.Context, token string) error {
	if token == "" {
		return nil
	}
	_, err := r.db.ExecContext(ctx, `DELETE FROM user_sessions WHERE token = ?`, token)
	if err != nil {
		return fmt.Errorf("delete session: %w", err)
	}
	return nil
}
