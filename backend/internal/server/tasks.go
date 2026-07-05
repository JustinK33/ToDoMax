package server

import (
	"encoding/json"
	"errors"
	"net/http"

	"github.com/jackc/pgx/v5"

	"github.com/justin/todomax/backend/internal/auth"
	"github.com/justin/todomax/backend/internal/task"
)

func writeJSON(w http.ResponseWriter, status int, v interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(v)
}

func writeErr(w http.ResponseWriter, status int, msg string) {
	http.Error(w, msg, status)
}

func taskErrStatus(err error) int {
	if errors.Is(err, pgx.ErrNoRows) {
		return http.StatusNotFound
	}
	var invalid task.ErrInvalidInput
	if errors.As(err, &invalid) {
		return http.StatusBadRequest
	}
	return http.StatusInternalServerError
}

func (s *Server) handleCreateTask(w http.ResponseWriter, r *http.Request) {
	userID, _ := auth.UserID(r.Context())

	var in task.Input
	if err := json.NewDecoder(r.Body).Decode(&in); err != nil {
		writeErr(w, http.StatusBadRequest, "invalid json body")
		return
	}
	if in.Title == "" {
		writeErr(w, http.StatusBadRequest, "title is required")
		return
	}

	t, err := s.tasks.Create(r.Context(), userID, in)
	if err != nil {
		writeErr(w, taskErrStatus(err), err.Error())
		return
	}
	writeJSON(w, http.StatusCreated, t)
}

func (s *Server) handleListTasks(w http.ResponseWriter, r *http.Request) {
	userID, _ := auth.UserID(r.Context())

	tasks, err := s.tasks.List(r.Context(), userID)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, err.Error())
		return
	}
	if tasks == nil {
		tasks = []task.Task{}
	}
	writeJSON(w, http.StatusOK, tasks)
}

func (s *Server) handleGetTask(w http.ResponseWriter, r *http.Request) {
	userID, _ := auth.UserID(r.Context())
	id := r.PathValue("id")

	t, err := s.tasks.Get(r.Context(), userID, id)
	if err != nil {
		writeErr(w, taskErrStatus(err), "task not found")
		return
	}
	writeJSON(w, http.StatusOK, t)
}

func (s *Server) handleUpdateTask(w http.ResponseWriter, r *http.Request) {
	userID, _ := auth.UserID(r.Context())
	id := r.PathValue("id")

	var in task.Input
	if err := json.NewDecoder(r.Body).Decode(&in); err != nil {
		writeErr(w, http.StatusBadRequest, "invalid json body")
		return
	}
	if in.Title == "" {
		writeErr(w, http.StatusBadRequest, "title is required")
		return
	}

	t, err := s.tasks.Update(r.Context(), userID, id, in)
	if err != nil {
		writeErr(w, taskErrStatus(err), err.Error())
		return
	}
	writeJSON(w, http.StatusOK, t)
}

func (s *Server) handleDeleteTask(w http.ResponseWriter, r *http.Request) {
	userID, _ := auth.UserID(r.Context())
	id := r.PathValue("id")

	if err := s.tasks.Delete(r.Context(), userID, id); err != nil {
		writeErr(w, taskErrStatus(err), "task not found")
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) handleCompleteTask(w http.ResponseWriter, r *http.Request) {
	s.setCompleted(w, r, true)
}

func (s *Server) handleUncompleteTask(w http.ResponseWriter, r *http.Request) {
	s.setCompleted(w, r, false)
}

func (s *Server) setCompleted(w http.ResponseWriter, r *http.Request, completed bool) {
	userID, _ := auth.UserID(r.Context())
	id := r.PathValue("id")

	t, err := s.tasks.SetCompleted(r.Context(), userID, id, completed)
	if err != nil {
		writeErr(w, taskErrStatus(err), "task not found")
		return
	}
	writeJSON(w, http.StatusOK, t)
}
