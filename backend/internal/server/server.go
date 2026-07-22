// Package server wires everything together: it opens the database pool,
// starts the reminder ticker, and exposes the HTTP API. Routes are grouped by
// feature across the other files here (tasks, goals, nutrition, training,
// wellness), all sitting behind Supabase JWT auth and CORS.
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
	"github.com/justin/todomax/backend/internal/nutrition"
	"github.com/justin/todomax/backend/internal/reminder"
	"github.com/justin/todomax/backend/internal/task"
	"github.com/justin/todomax/backend/internal/training"
	"github.com/justin/todomax/backend/internal/wellness"
)

type Server struct {
	cfg       config.Config
	db        *pgxpool.Pool
	authKeys  keyfunc.Keyfunc
	tasks     *task.Store
	goals     *goal.Store
	nutrition *nutrition.Store
	training  *training.Store
	wellness  *wellness.Store
	stop      context.CancelFunc
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
	nutritionStore := nutrition.NewStore(db)
	trainingStore := training.NewStore(db)
	wellnessStore := wellness.NewStore(db)

	if db != nil {
		reminderRunner := reminder.New(tasks, reminder.Config{
			ResendAPIKey: cfg.ResendAPIKey,
			FromEmail:    cfg.ReminderFromEmail,
			ToEmail:      cfg.ReminderToEmail,
		}, cfg.Location)
		go reminderRunner.Run(ctx)
	}

	return &Server{cfg: cfg, db: db, authKeys: authKeys, tasks: tasks, goals: goals, nutrition: nutritionStore, training: trainingStore, wellness: wellnessStore, stop: cancel}, nil
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
	mux.Handle("GET /api/tasks/habits", requireAuth(http.HandlerFunc(s.handleListHabits)))

	mux.Handle("POST /api/goals", requireAuth(http.HandlerFunc(s.handleCreateGoal)))
	mux.Handle("GET /api/goals", requireAuth(http.HandlerFunc(s.handleListGoals)))
	mux.Handle("GET /api/goals/{id}", requireAuth(http.HandlerFunc(s.handleGetGoal)))
	mux.Handle("PUT /api/goals/{id}", requireAuth(http.HandlerFunc(s.handleUpdateGoal)))
	mux.Handle("DELETE /api/goals/{id}", requireAuth(http.HandlerFunc(s.handleDeleteGoal)))
	mux.Handle("POST /api/goals/{id}/complete", requireAuth(http.HandlerFunc(s.handleCompleteGoal)))
	mux.Handle("POST /api/goals/{id}/uncomplete", requireAuth(http.HandlerFunc(s.handleUncompleteGoal)))

	mux.Handle("POST /api/foods", requireAuth(http.HandlerFunc(s.handleCreateFood)))
	mux.Handle("GET /api/foods", requireAuth(http.HandlerFunc(s.handleListFoods)))
	mux.Handle("GET /api/foods/{id}", requireAuth(http.HandlerFunc(s.handleGetFood)))
	mux.Handle("PUT /api/foods/{id}", requireAuth(http.HandlerFunc(s.handleUpdateFood)))
	mux.Handle("DELETE /api/foods/{id}", requireAuth(http.HandlerFunc(s.handleDeleteFood)))

	mux.Handle("POST /api/meals", requireAuth(http.HandlerFunc(s.handleCreateMeal)))
	mux.Handle("GET /api/meals", requireAuth(http.HandlerFunc(s.handleListMeals)))
	mux.Handle("GET /api/meals/{id}", requireAuth(http.HandlerFunc(s.handleGetMeal)))
	mux.Handle("PUT /api/meals/{id}", requireAuth(http.HandlerFunc(s.handleUpdateMeal)))
	mux.Handle("DELETE /api/meals/{id}", requireAuth(http.HandlerFunc(s.handleDeleteMeal)))

	mux.Handle("POST /api/nutrition/log", requireAuth(http.HandlerFunc(s.handleCreateLogEntry)))
	mux.Handle("PUT /api/nutrition/log/{id}", requireAuth(http.HandlerFunc(s.handleUpdateLogEntry)))
	mux.Handle("DELETE /api/nutrition/log/{id}", requireAuth(http.HandlerFunc(s.handleDeleteLogEntry)))
	mux.Handle("GET /api/nutrition/day", requireAuth(http.HandlerFunc(s.handleDaySummary)))
	mux.Handle("GET /api/nutrition/history", requireAuth(http.HandlerFunc(s.handleNutritionHistory)))
	mux.Handle("GET /api/nutrition/target", requireAuth(http.HandlerFunc(s.handleGetTarget)))
	mux.Handle("PUT /api/nutrition/target", requireAuth(http.HandlerFunc(s.handleSetTarget)))

	mux.Handle("POST /api/exercises", requireAuth(http.HandlerFunc(s.handleCreateExercise)))
	mux.Handle("GET /api/exercises", requireAuth(http.HandlerFunc(s.handleListExercises)))
	mux.Handle("GET /api/exercises/{id}", requireAuth(http.HandlerFunc(s.handleGetExercise)))
	mux.Handle("PUT /api/exercises/{id}", requireAuth(http.HandlerFunc(s.handleUpdateExercise)))
	mux.Handle("DELETE /api/exercises/{id}", requireAuth(http.HandlerFunc(s.handleDeleteExercise)))
	mux.Handle("POST /api/workout-sets", requireAuth(http.HandlerFunc(s.handleCreateWorkoutSet)))
	mux.Handle("DELETE /api/workout-sets/{id}", requireAuth(http.HandlerFunc(s.handleDeleteWorkoutSet)))
	mux.Handle("POST /api/workout-templates", requireAuth(http.HandlerFunc(s.handleCreateTemplate)))
	mux.Handle("GET /api/workout-templates", requireAuth(http.HandlerFunc(s.handleListTemplates)))
	mux.Handle("GET /api/workout-templates/{id}", requireAuth(http.HandlerFunc(s.handleGetTemplate)))
	mux.Handle("PUT /api/workout-templates/{id}", requireAuth(http.HandlerFunc(s.handleUpdateTemplate)))
	mux.Handle("DELETE /api/workout-templates/{id}", requireAuth(http.HandlerFunc(s.handleDeleteTemplate)))
	mux.Handle("GET /api/training/summary", requireAuth(http.HandlerFunc(s.handleTrainingSummary)))
	mux.Handle("GET /api/training/history", requireAuth(http.HandlerFunc(s.handleTrainingHistory)))

	mux.Handle("POST /api/wellness/body-metrics", requireAuth(http.HandlerFunc(s.handleCreateBodyMetric)))
	mux.Handle("GET /api/wellness/body-metrics", requireAuth(http.HandlerFunc(s.handleListBodyMetrics)))
	mux.Handle("GET /api/wellness/body-metrics/{id}", requireAuth(http.HandlerFunc(s.handleGetBodyMetric)))
	mux.Handle("PUT /api/wellness/body-metrics/{id}", requireAuth(http.HandlerFunc(s.handleUpdateBodyMetric)))
	mux.Handle("DELETE /api/wellness/body-metrics/{id}", requireAuth(http.HandlerFunc(s.handleDeleteBodyMetric)))

	mux.Handle("POST /api/wellness/sleep", requireAuth(http.HandlerFunc(s.handleCreateSleepLog)))
	mux.Handle("GET /api/wellness/sleep", requireAuth(http.HandlerFunc(s.handleListSleepLogs)))
	mux.Handle("GET /api/wellness/sleep/{id}", requireAuth(http.HandlerFunc(s.handleGetSleepLog)))
	mux.Handle("PUT /api/wellness/sleep/{id}", requireAuth(http.HandlerFunc(s.handleUpdateSleepLog)))
	mux.Handle("DELETE /api/wellness/sleep/{id}", requireAuth(http.HandlerFunc(s.handleDeleteSleepLog)))

	mux.Handle("POST /api/wellness/mood", requireAuth(http.HandlerFunc(s.handleCreateMoodLog)))
	mux.Handle("GET /api/wellness/mood", requireAuth(http.HandlerFunc(s.handleListMoodLogs)))
	mux.Handle("GET /api/wellness/mood/{id}", requireAuth(http.HandlerFunc(s.handleGetMoodLog)))
	mux.Handle("PUT /api/wellness/mood/{id}", requireAuth(http.HandlerFunc(s.handleUpdateMoodLog)))
	mux.Handle("DELETE /api/wellness/mood/{id}", requireAuth(http.HandlerFunc(s.handleDeleteMoodLog)))

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
