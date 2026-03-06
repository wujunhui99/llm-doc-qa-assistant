package handler

import (
	"net/http"

	"llm-doc-qa-assistant/backend/services/api-go/internal/middleware"
	apitypes "llm-doc-qa-assistant/backend/services/api-go/internal/types"
)

func (s *Server) handleRoot(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, map[string]interface{}{
		"name":    "Smart Document QA Assistant API",
		"version": "0.3.0",
	})
}

func (s *Server) handleRegister(w http.ResponseWriter, r *http.Request) {
	var req apitypes.RegisterRequest
	if err := decodeJSONBody(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_json", err.Error())
		return
	}

	resp, err := s.authLogic.Register(r.Context(), req.Email, req.Password)
	if err != nil {
		writeGRPCError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, map[string]interface{}{"user": protoUserMap(resp.GetUser())})
}

func (s *Server) handleLogin(w http.ResponseWriter, r *http.Request) {
	var req apitypes.LoginRequest
	if err := decodeJSONBody(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_json", err.Error())
		return
	}

	resp, err := s.authLogic.Login(r.Context(), req.Email, req.Password)
	if err != nil {
		writeGRPCError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]interface{}{
		"token":      resp.GetToken(),
		"expires_at": resp.GetExpiresAt(),
		"user":       protoUserMap(resp.GetUser()),
	})
}

func (s *Server) handleLogout(w http.ResponseWriter, r *http.Request) {
	token := middleware.TokenFromContext(r.Context())
	if err := s.authLogic.Logout(r.Context(), token); err != nil {
		writeGRPCError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]interface{}{"ok": true})
}

func (s *Server) handleMe(w http.ResponseWriter, r *http.Request) {
	token := middleware.TokenFromContext(r.Context())
	resp, err := s.authLogic.Me(r.Context(), token)
	if err != nil {
		writeGRPCError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]interface{}{"user": protoUserMap(resp.GetUser())})
}
