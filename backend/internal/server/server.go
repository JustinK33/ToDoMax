package server

import (
	"context"
	"encoding/json"
	"net/http"
	"time"

	"github.com/MicahParks/keyfunc/v3"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/justin/todomax/backend/internal/auth"
	"github.com/justin/todomax/backend/internal/config"
)

type Server struct {
	cfg      config.Config
	db       *pgxpool.Pool
	authKeys keyfunc.Keyfunc
	stop     context.CancelFunc
}

func New() (*Server, error) {
	cfg := config.Load()

	var db *pgxpool.Pool
	if cfg.DatabaseURL != "" {
		pool, err := pgxpool.New(context.Background(), cfg.DatabaseURL)
		if err != nil {
			return nil, err
		}
		db = pool
	}

	ctx, cancel := context.WithCancel(context.Background())
	authKeys, err := auth.NewKeyfunc(ctx, cfg.SupabaseURL)
	if err != nil {
		cancel()
		return nil, err
	}

	return &Server{cfg: cfg, db: db, authKeys: authKeys, stop: cancel}, nil
}

func (s *Server) Close() {
	s.stop()
	if s.db != nil {
		s.db.Close()
	}
}

func (s *Server) Routes() http.Handler {
	requireAuth := auth.Middleware(s.authKeys)

	mux := http.NewServeMux()
	mux.HandleFunc("GET /healthz", s.handleHealthz)
	mux.Handle("GET /api/me", requireAuth(http.HandlerFunc(s.handleMe)))
	return s.cors(mux)
}

// cors reflects the request Origin back when it's in the configured
// allow-list, since the frontend (Vercel/localhost) and backend are on
// different origins and the browser calls fetch() directly cross-origin.
func (s *Server) cors(next http.Handler) http.Handler {
	allowed := make(map[string]bool, len(s.cfg.FrontendOrigins))
	for _, o := range s.cfg.FrontendOrigins {
		allowed[o] = true
	}

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		origin := r.Header.Get("Origin")
		if allowed[origin] {
			w.Header().Set("Access-Control-Allow-Origin", origin)
			w.Header().Set("Access-Control-Allow-Headers", "Authorization, Content-Type")
			w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
			w.Header().Set("Vary", "Origin")
		}
		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		next.ServeHTTP(w, r)
	})
}

func (s *Server) handleMe(w http.ResponseWriter, r *http.Request) {
	userID, _ := auth.UserID(r.Context())
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"user_id": userID})
}

func (s *Server) handleHealthz(w http.ResponseWriter, r *http.Request) {
	if s.db != nil {
		ctx, cancel := context.WithTimeout(r.Context(), 2*time.Second)
		defer cancel()
		if err := s.db.Ping(ctx); err != nil {
			w.WriteHeader(http.StatusServiceUnavailable)
			w.Write([]byte("db unreachable"))
			return
		}
	}
	w.WriteHeader(http.StatusOK)
	w.Write([]byte("ok"))
}
