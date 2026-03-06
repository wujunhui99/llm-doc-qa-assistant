package types

import "time"

type User struct {
	ID           string    `json:"id"`
	Email        string    `json:"email"`
	PasswordHash string    `json:"password_hash"`
	CreatedAt    time.Time `json:"created_at"`
}

type Session struct {
	Token     string    `json:"token"`
	UserID    string    `json:"user_id"`
	ExpiresAt time.Time `json:"expires_at"`
	CreatedAt time.Time `json:"created_at"`
}

type Chunk struct {
	ID      string `json:"id"`
	DocID   string `json:"doc_id"`
	Index   int    `json:"index"`
	Content string `json:"content"`
}

type Document struct {
	ID            string    `json:"id"`
	OwnerUserID   string    `json:"owner_user_id"`
	Name          string    `json:"name"`
	SizeBytes     int64     `json:"size_bytes"`
	MimeType      string    `json:"mime_type"`
	StoragePath   string    `json:"storage_path"`
	Status        string    `json:"status"`
	ChunkCount    int       `json:"chunk_count"`
	CreatedAt     time.Time `json:"created_at"`
	LastUpdatedAt time.Time `json:"last_updated_at"`
}

type Thread struct {
	ID          string    `json:"id"`
	OwnerUserID string    `json:"owner_user_id"`
	Title       string    `json:"title"`
	CreatedAt   time.Time `json:"created_at"`
}

type Turn struct {
	ID          string    `json:"id"`
	ThreadID    string    `json:"thread_id"`
	OwnerUserID string    `json:"owner_user_id"`
	Question    string    `json:"question"`
	Answer      string    `json:"answer"`
	Status      string    `json:"status"`
	ScopeType   string    `json:"scope_type"`
	ScopeDocIDs []string  `json:"scope_doc_ids"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

type TurnItem struct {
	ID        string                 `json:"id"`
	TurnID    string                 `json:"turn_id"`
	ItemType  string                 `json:"item_type"`
	Payload   map[string]interface{} `json:"payload"`
	CreatedAt time.Time              `json:"created_at"`
}

type Citation struct {
	DocID    string `json:"doc_id"`
	DocName  string `json:"doc_name"`
	ChunkID  string `json:"chunk_id"`
	Excerpt  string `json:"excerpt"`
	Score    int    `json:"score"`
	ChunkIdx int    `json:"chunk_index"`
}

type ProviderConfig struct {
	ActiveProvider string   `json:"active_provider"`
	Available      []string `json:"available"`
}

type AuditEvent struct {
	ID        string                 `json:"id"`
	EventType string                 `json:"event_type"`
	ActorID   string                 `json:"actor_id"`
	TargetID  string                 `json:"target_id"`
	Metadata  map[string]interface{} `json:"metadata"`
	CreatedAt time.Time              `json:"created_at"`
}

type State struct {
	Documents    map[string]Document   `json:"documents"`
	Chunks       map[string][]Chunk    `json:"chunks"`
	Threads      map[string]Thread     `json:"threads"`
	Turns        map[string]Turn       `json:"turns"`
	TurnItems    map[string][]TurnItem `json:"turn_items"`
	Provider     ProviderConfig        `json:"provider"`
	Initialized  bool                  `json:"initialized"`
	LastSavedUTC time.Time             `json:"last_saved_utc"`
}
