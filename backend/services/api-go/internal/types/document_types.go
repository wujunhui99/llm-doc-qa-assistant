package types

type ListDocumentsResponse struct {
	Documents []map[string]interface{} `json:"documents"`
	Total     int32                    `json:"total"`
	Page      int32                    `json:"page"`
	PageSize  int32                    `json:"page_size"`
}
