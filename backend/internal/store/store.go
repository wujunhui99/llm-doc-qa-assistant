package store

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"

	"llm-doc-qa-assistant/backend/internal/types"
)

type Store struct {
	mu        sync.RWMutex
	statePath string
	auditPath string
	state     types.State
}

func New(statePath, auditPath string) (*Store, error) {
	if err := os.MkdirAll(filepath.Dir(statePath), 0o755); err != nil {
		return nil, err
	}
	if err := os.MkdirAll(filepath.Dir(auditPath), 0o755); err != nil {
		return nil, err
	}

	s := &Store{
		statePath: statePath,
		auditPath: auditPath,
	}
	if err := s.load(); err != nil {
		return nil, err
	}
	return s, nil
}

func (s *Store) load() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	data, err := os.ReadFile(s.statePath)
	if err != nil {
		if !errors.Is(err, os.ErrNotExist) {
			return err
		}
		s.state = defaultState()
		return s.saveLocked()
	}

	if len(data) == 0 {
		s.state = defaultState()
		return s.saveLocked()
	}

	var state types.State
	if err := json.Unmarshal(data, &state); err != nil {
		return fmt.Errorf("failed to parse state: %w", err)
	}
	ensureMaps(&state)
	if !state.Initialized {
		state = defaultState()
	}
	s.state = state
	return nil
}

func defaultState() types.State {
	return types.State{
		Users:     map[string]types.User{},
		Sessions:  map[string]types.Session{},
		Documents: map[string]types.Document{},
		Chunks:    map[string][]types.Chunk{},
		Threads:   map[string]types.Thread{},
		Turns:     map[string]types.Turn{},
		TurnItems: map[string][]types.TurnItem{},
		Provider: types.ProviderConfig{
			ActiveProvider: "mock",
			Available:      []string{"mock", "openai", "claude", "local"},
		},
		EmailToUser: map[string]string{},
		Initialized: true,
	}
}

func ensureMaps(state *types.State) {
	if state.Users == nil {
		state.Users = map[string]types.User{}
	}
	if state.Sessions == nil {
		state.Sessions = map[string]types.Session{}
	}
	if state.Documents == nil {
		state.Documents = map[string]types.Document{}
	}
	if state.Chunks == nil {
		state.Chunks = map[string][]types.Chunk{}
	}
	if state.Threads == nil {
		state.Threads = map[string]types.Thread{}
	}
	if state.Turns == nil {
		state.Turns = map[string]types.Turn{}
	}
	if state.TurnItems == nil {
		state.TurnItems = map[string][]types.TurnItem{}
	}
	if state.EmailToUser == nil {
		state.EmailToUser = map[string]string{}
	}
	if len(state.Provider.Available) == 0 {
		state.Provider.Available = []string{"mock", "openai", "claude", "local"}
	}
	if state.Provider.ActiveProvider == "" {
		state.Provider.ActiveProvider = "mock"
	}
}

func (s *Store) saveLocked() error {
	s.state.LastSavedUTC = time.Now().UTC()
	data, err := json.MarshalIndent(s.state, "", "  ")
	if err != nil {
		return err
	}
	tmpPath := s.statePath + ".tmp"
	if err := os.WriteFile(tmpPath, data, 0o644); err != nil {
		return err
	}
	return os.Rename(tmpPath, s.statePath)
}

func (s *Store) appendAuditLocked(event types.AuditEvent) {
	if event.ID == "" {
		event.ID = types.NewID("audit")
	}
	event.CreatedAt = time.Now().UTC()
	if event.Metadata == nil {
		event.Metadata = map[string]interface{}{}
	}
	line, _ := json.Marshal(event)
	f, err := os.OpenFile(s.auditPath, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o644)
	if err != nil {
		return
	}
	defer f.Close()
	_, _ = f.Write(append(line, '\n'))
}

func (s *Store) RecordAudit(eventType, actorID, targetID string, metadata map[string]interface{}) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.appendAuditLocked(types.AuditEvent{
		EventType: eventType,
		ActorID:   actorID,
		TargetID:  targetID,
		Metadata:  metadata,
	})
}

func (s *Store) CreateUser(user types.User) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if _, ok := s.state.EmailToUser[user.Email]; ok {
		return errors.New("email already exists")
	}
	s.state.Users[user.ID] = user
	s.state.EmailToUser[user.Email] = user.ID
	if err := s.saveLocked(); err != nil {
		return err
	}
	s.appendAuditLocked(types.AuditEvent{EventType: "auth.register", ActorID: user.ID, TargetID: user.ID})
	return nil
}

func (s *Store) GetUserByEmail(email string) (types.User, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	userID, ok := s.state.EmailToUser[email]
	if !ok {
		return types.User{}, false
	}
	user, ok := s.state.Users[userID]
	return user, ok
}

func (s *Store) GetUserByID(userID string) (types.User, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	user, ok := s.state.Users[userID]
	return user, ok
}

func (s *Store) CreateSession(session types.Session) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.state.Sessions[session.Token] = session
	return s.saveLocked()
}

func (s *Store) DeleteSession(token string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.state.Sessions, token)
	return s.saveLocked()
}

func (s *Store) GetSession(token string) (types.Session, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	session, ok := s.state.Sessions[token]
	return session, ok
}

func (s *Store) UpsertDocument(doc types.Document, chunks []types.Chunk) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.state.Documents[doc.ID] = doc
	if chunks != nil {
		s.state.Chunks[doc.ID] = chunks
	}
	return s.saveLocked()
}

func (s *Store) GetDocument(docID string) (types.Document, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	doc, ok := s.state.Documents[docID]
	return doc, ok
}

func (s *Store) ListDocumentsByOwner(userID string) []types.Document {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := make([]types.Document, 0)
	for _, doc := range s.state.Documents {
		if doc.OwnerUserID == userID {
			out = append(out, doc)
		}
	}
	return out
}

func (s *Store) GetChunksForDoc(docID string) []types.Chunk {
	s.mu.RLock()
	defer s.mu.RUnlock()
	chunks := s.state.Chunks[docID]
	out := make([]types.Chunk, len(chunks))
	copy(out, chunks)
	return out
}

func (s *Store) DeleteDocument(docID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.state.Documents, docID)
	delete(s.state.Chunks, docID)
	return s.saveLocked()
}

func (s *Store) CreateThread(thread types.Thread) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.state.Threads[thread.ID] = thread
	return s.saveLocked()
}

func (s *Store) ListThreadsByOwner(userID string) []types.Thread {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := make([]types.Thread, 0)
	for _, t := range s.state.Threads {
		if t.OwnerUserID == userID {
			out = append(out, t)
		}
	}
	return out
}

func (s *Store) GetThread(threadID string) (types.Thread, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	t, ok := s.state.Threads[threadID]
	return t, ok
}

func (s *Store) CreateOrUpdateTurn(turn types.Turn, items []types.TurnItem) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.state.Turns[turn.ID] = turn
	if items != nil {
		s.state.TurnItems[turn.ID] = items
	}
	return s.saveLocked()
}

func (s *Store) GetTurn(turnID string) (types.Turn, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	turn, ok := s.state.Turns[turnID]
	return turn, ok
}

func (s *Store) ListTurnsByThread(threadID string) []types.Turn {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := make([]types.Turn, 0)
	for _, turn := range s.state.Turns {
		if turn.ThreadID == threadID {
			out = append(out, turn)
		}
	}
	return out
}

func (s *Store) GetTurnItems(turnID string) []types.TurnItem {
	s.mu.RLock()
	defer s.mu.RUnlock()
	items := s.state.TurnItems[turnID]
	out := make([]types.TurnItem, len(items))
	copy(out, items)
	return out
}

func (s *Store) GetProvider() types.ProviderConfig {
	s.mu.RLock()
	defer s.mu.RUnlock()
	provider := s.state.Provider
	provider.Available = append([]string(nil), provider.Available...)
	return provider
}

func (s *Store) SetProvider(active string, actorID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	allowed := false
	for _, provider := range s.state.Provider.Available {
		if provider == active {
			allowed = true
			break
		}
	}
	if !allowed {
		return errors.New("unsupported provider")
	}
	s.state.Provider.ActiveProvider = active
	if err := s.saveLocked(); err != nil {
		return err
	}
	s.appendAuditLocked(types.AuditEvent{
		EventType: "config.provider_change",
		ActorID:   actorID,
		TargetID:  active,
	})
	return nil
}
