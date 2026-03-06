package memory

import (
	"context"
	"sync"

	domainauth "llm-doc-qa-assistant/backend/apps/core-go-rpc/internal/domain/auth"
)

type authStore struct {
	mu          sync.RWMutex
	users       map[string]domainauth.User
	emailToUser map[string]string
	sessions    map[string]domainauth.Session
}

func NewAuthRepositories() (*UserRepository, *SessionRepository) {
	store := &authStore{
		users:       map[string]domainauth.User{},
		emailToUser: map[string]string{},
		sessions:    map[string]domainauth.Session{},
	}
	return &UserRepository{store: store}, &SessionRepository{store: store}
}

type UserRepository struct {
	store *authStore
}

func (r *UserRepository) Create(ctx context.Context, user domainauth.User) error {
	_ = ctx
	r.store.mu.Lock()
	defer r.store.mu.Unlock()
	if _, exists := r.store.emailToUser[user.Email]; exists {
		return domainauth.ErrEmailAlreadyExist
	}
	r.store.users[user.ID] = user
	r.store.emailToUser[user.Email] = user.ID
	return nil
}

func (r *UserRepository) GetByEmail(ctx context.Context, email string) (domainauth.User, error) {
	_ = ctx
	r.store.mu.RLock()
	defer r.store.mu.RUnlock()
	id, ok := r.store.emailToUser[domainauth.NormalizeEmail(email)]
	if !ok {
		return domainauth.User{}, domainauth.ErrNotFound
	}
	user, ok := r.store.users[id]
	if !ok {
		return domainauth.User{}, domainauth.ErrNotFound
	}
	return user, nil
}

func (r *UserRepository) GetByID(ctx context.Context, id string) (domainauth.User, error) {
	_ = ctx
	r.store.mu.RLock()
	defer r.store.mu.RUnlock()
	user, ok := r.store.users[id]
	if !ok {
		return domainauth.User{}, domainauth.ErrNotFound
	}
	return user, nil
}

type SessionRepository struct {
	store *authStore
}

func (r *SessionRepository) Create(ctx context.Context, session domainauth.Session) error {
	_ = ctx
	r.store.mu.Lock()
	defer r.store.mu.Unlock()
	r.store.sessions[session.Token] = session
	return nil
}

func (r *SessionRepository) GetByToken(ctx context.Context, token string) (domainauth.Session, error) {
	_ = ctx
	r.store.mu.RLock()
	defer r.store.mu.RUnlock()
	session, ok := r.store.sessions[token]
	if !ok {
		return domainauth.Session{}, domainauth.ErrNotFound
	}
	return session, nil
}

func (r *SessionRepository) DeleteByToken(ctx context.Context, token string) error {
	_ = ctx
	r.store.mu.Lock()
	defer r.store.mu.Unlock()
	delete(r.store.sessions, token)
	return nil
}
