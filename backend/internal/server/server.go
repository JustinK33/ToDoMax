package server

import (
	"context"
	"net/http"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/justin/todomax/backend/internal/config"
)

type Server struct {
	cfg config.Config
	db  *pgxpool.Pool
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

	return &Server{cfg: cfg, db: db}, nil
}

func (s *Server) Close() {
	if s.db != nil {
		s.db.Close()
	}
}

func (s *Server) Routes() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /healthz", s.handleHealthz)
	return mux
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
