package server

import (
	"encoding/json"
	"errors"
	"net/http"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"

	"github.com/justin/todomax/backend/internal/auth"
	"github.com/justin/todomax/backend/internal/training"
)

func trainingErrStatus(err error) int {
	if errors.Is(err, pgx.ErrNoRows) {
		return http.StatusNotFound
	}
	var invalid training.ErrInvalidInput
	if errors.As(err, &invalid) {
		return http.StatusBadRequest
	}
	return http.StatusInternalServerError
}

func writeTrainingErr(w http.ResponseWriter, err error) {
	status := trainingErrStatus(err)
	if status == http.StatusInternalServerError {
		writeUnexpectedErr(w, err)
		return
	}
	writeErr(w, status, err.Error())
}

func (s *Server) handleCreateExercise(w http.ResponseWriter, r *http.Request) {
	userID, _ := auth.UserID(r.Context())

	var in training.ExerciseInput
	if err := json.NewDecoder(r.Body).Decode(&in); err != nil {
		writeErr(w, http.StatusBadRequest, "invalid json body")
		return
	}
	in.Name = strings.TrimSpace(in.Name)
	in.Category = strings.TrimSpace(in.Category)

	exercise, err := s.training.CreateExercise(r.Context(), userID, in)
	if err != nil {
		writeTrainingErr(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, exercise)
}

func (s *Server) handleListExercises(w http.ResponseWriter, r *http.Request) {
	userID, _ := auth.UserID(r.Context())

	exercises, err := s.training.ListExercises(r.Context(), userID)
	if err != nil {
		writeUnexpectedErr(w, err)
		return
	}
	if exercises == nil {
		exercises = []training.Exercise{}
	}
	writeJSON(w, http.StatusOK, exercises)
}

func (s *Server) handleGetExercise(w http.ResponseWriter, r *http.Request) {
	userID, _ := auth.UserID(r.Context())
	id := r.PathValue("id")

	exercise, err := s.training.GetExercise(r.Context(), userID, id)
	if err != nil {
		writeErr(w, trainingErrStatus(err), "exercise not found")
		return
	}
	writeJSON(w, http.StatusOK, exercise)
}

func (s *Server) handleUpdateExercise(w http.ResponseWriter, r *http.Request) {
	userID, _ := auth.UserID(r.Context())
	id := r.PathValue("id")

	var in training.ExerciseInput
	if err := json.NewDecoder(r.Body).Decode(&in); err != nil {
		writeErr(w, http.StatusBadRequest, "invalid json body")
		return
	}
	in.Name = strings.TrimSpace(in.Name)
	in.Category = strings.TrimSpace(in.Category)

	exercise, err := s.training.UpdateExercise(r.Context(), userID, id, in)
	if err != nil {
		writeTrainingErr(w, err)
		return
	}
	writeJSON(w, http.StatusOK, exercise)
}

func (s *Server) handleDeleteExercise(w http.ResponseWriter, r *http.Request) {
	userID, _ := auth.UserID(r.Context())
	id := r.PathValue("id")

	if err := s.training.DeleteExercise(r.Context(), userID, id); err != nil {
		writeErr(w, trainingErrStatus(err), "exercise not found")
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) handleCreateWorkoutSet(w http.ResponseWriter, r *http.Request) {
	userID, _ := auth.UserID(r.Context())

	var in training.SetInput
	if err := json.NewDecoder(r.Body).Decode(&in); err != nil {
		writeErr(w, http.StatusBadRequest, "invalid json body")
		return
	}

	set, err := s.training.CreateSet(r.Context(), userID, in)
	if err != nil {
		writeTrainingErr(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, set)
}

func (s *Server) handleDeleteWorkoutSet(w http.ResponseWriter, r *http.Request) {
	userID, _ := auth.UserID(r.Context())
	id := r.PathValue("id")

	if err := s.training.DeleteSet(r.Context(), userID, id); err != nil {
		writeErr(w, trainingErrStatus(err), "set not found")
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) handleTrainingSummary(w http.ResponseWriter, r *http.Request) {
	userID, _ := auth.UserID(r.Context())

	date := r.URL.Query().Get("date")
	if date == "" {
		date = time.Now().In(s.cfg.Location).Format("2006-01-02")
	}

	summary, err := s.training.Summary(r.Context(), userID, date)
	if err != nil {
		writeUnexpectedErr(w, err)
		return
	}
	writeJSON(w, http.StatusOK, summary)
}
