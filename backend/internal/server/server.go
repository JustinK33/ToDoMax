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
	"github.com/justin/todomax/backend/internal/goal"
	"github.com/justin/todomax/backend/internal/reminder"
	"github.com/justin/todomax/backend/internal/task"
)

type Server struct {
	cfg      config.Config
	db       *pgxpool.Pool
	authKeys keyfunc.Keyfunc
	tasks    *task.Store
	goals    *goal.Store
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

	tasks := task.NewStore(db)
	goals := goal.NewStore(db)

	if db != nil {
		reminderRunner := reminder.New(tasks, reminder.Config{
			ResendAPIKey: cfg.ResendAPIKey,
			FromEmail:    cfg.ReminderFromEmail,
			ToEmail:      cfg.ReminderToEmail,
		}, cfg.Location)
		go reminderRunner.Run(ctx)
	}

	return &Server{cfg: cfg, db: db, authKeys: authKeys, tasks: tasks, goals: goals, stop: cancel}, nil
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
	mux.Handle("POST /api/tasks", requireAuth(http.HandlerFunc(s.handleCreateTask)))
	mux.Handle("GET /api/tasks", requireAuth(http.HandlerFunc(s.handleListTasks)))
	mux.Handle("GET /api/tasks/{id}", requireAuth(http.HandlerFunc(s.handleGetTask)))
	mux.Handle("PUT /api/tasks/{id}", requireAuth(http.HandlerFunc(s.handleUpdateTask)))
	mux.Handle("DELETE /api/tasks/{id}", requireAuth(http.HandlerFunc(s.handleDeleteTask)))
	mux.Handle("POST /api/tasks/{id}/complete", requireAuth(http.HandlerFunc(s.handleCompleteTask)))
	mux.Handle("POST /api/tasks/{id}/uncomplete", requireAuth(http.HandlerFunc(s.handleUncompleteTask)))
	mux.Handle("GET /api/summary/week", requireAuth(http.HandlerFunc(s.handleWeekSummary)))

	mux.Handle("POST /api/goals", requireAuth(http.HandlerFunc(s.handleCreateGoal)))
	mux.Handle("GET /api/goals", requireAuth(http.HandlerFunc(s.handleListGoals)))
	mux.Handle("GET /api/goals/{id}", requireAuth(http.HandlerFunc(s.handleGetGoal)))
	mux.Handle("PUT /api/goals/{id}", requireAuth(http.HandlerFunc(s.handleUpdateGoal)))
	mux.Handle("DELETE /api/goals/{id}", requireAuth(http.HandlerFunc(s.handleDeleteGoal)))
	mux.Handle("POST /api/goals/{id}/complete", requireAuth(http.HandlerFunc(s.handleCompleteGoal)))
	mux.Handle("POST /api/goals/{id}/uncomplete", requireAuth(http.HandlerFunc(s.handleUncompleteGoal)))

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
