package server

import (
	"encoding/json"
	"errors"
	"net/http"
	"strings"

	"github.com/jackc/pgx/v5"

	"github.com/justin/todomax/backend/internal/auth"
	"github.com/justin/todomax/backend/internal/goal"
)

func goalErrStatus(err error) int {
	if errors.Is(err, pgx.ErrNoRows) {
		return http.StatusNotFound
	}
	var invalid goal.ErrInvalidInput
	if errors.As(err, &invalid) {
		return http.StatusBadRequest
	}
	return http.StatusInternalServerError
}

func writeGoalErr(w http.ResponseWriter, err error) {
	status := goalErrStatus(err)
	if status == http.StatusInternalServerError {
		writeUnexpectedErr(w, err)
		return
	}
	writeErr(w, status, err.Error())
}

func (s *Server) handleCreateGoal(w http.ResponseWriter, r *http.Request) {
	userID, _ := auth.UserID(r.Context())

	var in goal.Input
	if err := json.NewDecoder(r.Body).Decode(&in); err != nil {
		writeErr(w, http.StatusBadRequest, "invalid json body")
		return
	}
	in.Title = strings.TrimSpace(in.Title)
	if in.Title == "" {
		writeErr(w, http.StatusBadRequest, "title is required")
		return
	}

	g, err := s.goals.Create(r.Context(), userID, in)
	if err != nil {
		writeGoalErr(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, g)
}

func (s *Server) handleListGoals(w http.ResponseWriter, r *http.Request) {
	userID, _ := auth.UserID(r.Context())

	goals, err := s.goals.List(r.Context(), userID)
	if err != nil {
		writeUnexpectedErr(w, err)
		return
	}
	if goals == nil {
		goals = []goal.Goal{}
	}
	writeJSON(w, http.StatusOK, goals)
}

func (s *Server) handleGetGoal(w http.ResponseWriter, r *http.Request) {
	userID, _ := auth.UserID(r.Context())
	id := r.PathValue("id")

	g, err := s.goals.Get(r.Context(), userID, id)
	if err != nil {
		writeErr(w, goalErrStatus(err), "goal not found")
		return
	}
	writeJSON(w, http.StatusOK, g)
}

func (s *Server) handleUpdateGoal(w http.ResponseWriter, r *http.Request) {
	userID, _ := auth.UserID(r.Context())
	id := r.PathValue("id")

	var in goal.Input
	if err := json.NewDecoder(r.Body).Decode(&in); err != nil {
		writeErr(w, http.StatusBadRequest, "invalid json body")
		return
	}
	in.Title = strings.TrimSpace(in.Title)
	if in.Title == "" {
		writeErr(w, http.StatusBadRequest, "title is required")
		return
	}

	g, err := s.goals.Update(r.Context(), userID, id, in)
	if err != nil {
		writeGoalErr(w, err)
		return
	}
	writeJSON(w, http.StatusOK, g)
}

func (s *Server) handleDeleteGoal(w http.ResponseWriter, r *http.Request) {
	userID, _ := auth.UserID(r.Context())
	id := r.PathValue("id")

	if err := s.goals.Delete(r.Context(), userID, id); err != nil {
		writeErr(w, goalErrStatus(err), "goal not found")
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) handleCompleteGoal(w http.ResponseWriter, r *http.Request) {
	s.setGoalCompleted(w, r, true)
}

func (s *Server) handleUncompleteGoal(w http.ResponseWriter, r *http.Request) {
	s.setGoalCompleted(w, r, false)
}

func (s *Server) setGoalCompleted(w http.ResponseWriter, r *http.Request, completed bool) {
	userID, _ := auth.UserID(r.Context())
	id := r.PathValue("id")

	g, err := s.goals.SetCompleted(r.Context(), userID, id, completed)
	if err != nil {
		writeErr(w, goalErrStatus(err), "goal not found")
		return
	}
	writeJSON(w, http.StatusOK, g)
}
