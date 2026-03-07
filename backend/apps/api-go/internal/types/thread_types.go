package types

type CreateThreadRequest struct {
	Title string `json:"title"`
}

type CreateTurnRequest struct {
	Message     string   `json:"message"`
	ScopeType   string   `json:"scope_type"`
	ScopeDocIDs []string `json:"scope_doc_ids"`
	ThinkMode   bool     `json:"think_mode"`
}
