package handler

import (
	"net/http"

	"llm-doc-qa-assistant/backend/services/api-go/internal/middleware"
	apitypes "llm-doc-qa-assistant/backend/services/api-go/internal/types"
)

func (s *Server) handleHealth(w http.ResponseWriter, r *http.Request) {
	resp, err := s.configLogic.Health(r.Context())
	if err != nil {
		writeGRPCError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]interface{}{"status": resp.GetStatus(), "time": resp.GetTime()})
}

func (s *Server) handleConfig(w http.ResponseWriter, r *http.Request) {
	token := middleware.TokenFromContext(r.Context())
	switch r.Method {
	case http.MethodGet:
		resp, err := s.configLogic.GetConfig(r.Context(), token)
		if err != nil {
			writeGRPCError(w, err)
			return
		}
		writeJSON(w, http.StatusOK, map[string]interface{}{
			"active_provider": resp.GetActiveProvider(),
			"available":       resp.GetAvailable(),
		})
	case http.MethodPut:
		var req apitypes.SetProviderRequest
		if err := decodeJSONBody(r, &req); err != nil {
			writeError(w, http.StatusBadRequest, "invalid_json", err.Error())
			return
		}
		resp, err := s.configLogic.SetConfig(r.Context(), token, req.ActiveProvider)
		if err != nil {
			writeGRPCError(w, err)
			return
		}
		writeJSON(w, http.StatusOK, map[string]interface{}{
			"active_provider": resp.GetActiveProvider(),
			"available":       resp.GetAvailable(),
		})
	case http.MethodOptions:
		w.WriteHeader(http.StatusNoContent)
	default:
		writeError(w, http.StatusMethodNotAllowed, "method_not_allowed", "method not allowed")
	}
}

func (s *Server) handleConfigHealth(w http.ResponseWriter, r *http.Request) {
	s.handleHealth(w, r)
}
