package handler

import (
	"log"
	"net/http"

	qav1 "llm-doc-qa-assistant/backend/proto/gen/go/qa/v1"
	"llm-doc-qa-assistant/backend/apps/api-go/internal/logic"
	"llm-doc-qa-assistant/backend/apps/api-go/internal/middleware"
	"llm-doc-qa-assistant/backend/apps/api-go/internal/svc"
)

type Server struct {
	svcCtx *svc.ServiceContext

	authLogic   *logic.AuthLogic
	docLogic    *logic.DocumentLogic
	threadLogic *logic.ThreadLogic
	configLogic *logic.ConfigLogic
}

// NewServer keeps compatibility for tests and lightweight wiring.
func NewServer(core qav1.CoreServiceClient, logger *log.Logger) *Server {
	cfg := svc.LoadConfig("")
	svcCtx := svc.NewServiceContext(cfg, core, logger)
	return NewServerWithContext(svcCtx)
}

func NewServerWithContext(svcCtx *svc.ServiceContext) *Server {
	if svcCtx == nil {
		panic("service context cannot be nil")
	}
	return &Server{
		svcCtx:      svcCtx,
		authLogic:   logic.NewAuthLogic(svcCtx),
		docLogic:    logic.NewDocumentLogic(svcCtx),
		threadLogic: logic.NewThreadLogic(svcCtx),
		configLogic: logic.NewConfigLogic(svcCtx),
	}
}

func (s *Server) Routes() http.Handler {
	mux := http.NewServeMux()

	mux.HandleFunc("/", s.handleRoot)
	mux.HandleFunc("/api/health", s.withMethod(http.MethodGet, s.handleHealth))

	mux.HandleFunc("/api/auth/register", s.withMethod(http.MethodPost, s.handleRegister))
	mux.HandleFunc("/api/auth/login", s.withMethod(http.MethodPost, s.handleLogin))
	mux.HandleFunc("/api/auth/logout", middleware.WithAuth(s.withMethod(http.MethodPost, s.handleLogout)))
	mux.HandleFunc("/api/auth/me", middleware.WithAuth(s.withMethod(http.MethodGet, s.handleMe)))

	mux.HandleFunc("/api/documents/upload", middleware.WithAuth(s.withMethod(http.MethodPost, s.handleUploadDocument)))
	mux.HandleFunc("/api/documents", middleware.WithAuth(s.handleDocumentsRoot))
	mux.HandleFunc("/api/documents/", middleware.WithAuth(s.handleDocumentRoutes))

	mux.HandleFunc("/api/threads", middleware.WithAuth(s.handleThreadsRoot))
	mux.HandleFunc("/api/threads/", middleware.WithAuth(s.handleThreadsSubRoutes))

	mux.HandleFunc("/api/config", middleware.WithAuth(s.handleConfig))
	mux.HandleFunc("/api/config/health", s.withMethod(http.MethodGet, s.handleConfigHealth))

	var handler http.Handler = mux
	handler = middleware.WithRequestLogging(s.svcCtx.Logger, handler)
	handler = middleware.WithCORS(handler)
	return handler
}

func (s *Server) withMethod(method string, next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		if r.Method != method {
			writeError(w, http.StatusMethodNotAllowed, "method_not_allowed", "method not allowed")
			return
		}
		next(w, r)
	}
}
