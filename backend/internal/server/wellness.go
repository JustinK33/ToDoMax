package server

import (
	"encoding/json"
	"errors"
	"net/http"

	"github.com/jackc/pgx/v5"

	"github.com/justin/todomax/backend/internal/auth"
	"github.com/justin/todomax/backend/internal/wellness"
)

func wellnessErrStatus(err error) int {
	if errors.Is(err, pgx.ErrNoRows) {
		return http.StatusNotFound
	}
	var invalid wellness.ErrInvalidInput
	if errors.As(err, &invalid) {
		return http.StatusBadRequest
	}
	return http.StatusInternalServerError
}

func writeWellnessErr(w http.ResponseWriter, err error) {
	status := wellnessErrStatus(err)
	if status == http.StatusInternalServerError {
		writeUnexpectedErr(w, err)
		return
	}
	writeErr(w, status, err.Error())
}

// --- Body metrics ---

func (s *Server) handleCreateBodyMetric(w http.ResponseWriter, r *http.Request) {
	userID, _ := auth.UserID(r.Context())

	var in wellness.BodyMetricInput
	if err := json.NewDecoder(r.Body).Decode(&in); err != nil {
		writeErr(w, http.StatusBadRequest, "invalid json body")
		return
	}

	m, err := s.wellness.CreateBodyMetric(r.Context(), userID, in)
	if err != nil {
		writeWellnessErr(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, m)
}

func (s *Server) handleListBodyMetrics(w http.ResponseWriter, r *http.Request) {
	userID, _ := auth.UserID(r.Context())

	metrics, err := s.wellness.ListBodyMetrics(r.Context(), userID)
	if err != nil {
		writeUnexpectedErr(w, err)
		return
	}
	if metrics == nil {
		metrics = []wellness.BodyMetric{}
	}
	writeJSON(w, http.StatusOK, metrics)
}

func (s *Server) handleGetBodyMetric(w http.ResponseWriter, r *http.Request) {
	userID, _ := auth.UserID(r.Context())
	id := r.PathValue("id")

	m, err := s.wellness.GetBodyMetric(r.Context(), userID, id)
	if err != nil {
		writeErr(w, wellnessErrStatus(err), "body metric not found")
		return
	}
	writeJSON(w, http.StatusOK, m)
}

func (s *Server) handleUpdateBodyMetric(w http.ResponseWriter, r *http.Request) {
	userID, _ := auth.UserID(r.Context())
	id := r.PathValue("id")

	var in wellness.BodyMetricInput
	if err := json.NewDecoder(r.Body).Decode(&in); err != nil {
		writeErr(w, http.StatusBadRequest, "invalid json body")
		return
	}

	m, err := s.wellness.UpdateBodyMetric(r.Context(), userID, id, in)
	if err != nil {
		writeWellnessErr(w, err)
		return
	}
	writeJSON(w, http.StatusOK, m)
}

func (s *Server) handleDeleteBodyMetric(w http.ResponseWriter, r *http.Request) {
	userID, _ := auth.UserID(r.Context())
	id := r.PathValue("id")

	if err := s.wellness.DeleteBodyMetric(r.Context(), userID, id); err != nil {
		writeErr(w, wellnessErrStatus(err), "body metric not found")
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// --- Sleep ---

func (s *Server) handleCreateSleepLog(w http.ResponseWriter, r *http.Request) {
	userID, _ := auth.UserID(r.Context())

	var in wellness.SleepLogInput
	if err := json.NewDecoder(r.Body).Decode(&in); err != nil {
		writeErr(w, http.StatusBadRequest, "invalid json body")
		return
	}

	l, err := s.wellness.CreateSleepLog(r.Context(), userID, in)
	if err != nil {
		writeWellnessErr(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, l)
}

func (s *Server) handleListSleepLogs(w http.ResponseWriter, r *http.Request) {
	userID, _ := auth.UserID(r.Context())

	logs, err := s.wellness.ListSleepLogs(r.Context(), userID)
	if err != nil {
		writeUnexpectedErr(w, err)
		return
	}
	if logs == nil {
		logs = []wellness.SleepLog{}
	}
	writeJSON(w, http.StatusOK, logs)
}

func (s *Server) handleGetSleepLog(w http.ResponseWriter, r *http.Request) {
	userID, _ := auth.UserID(r.Context())
	id := r.PathValue("id")

	l, err := s.wellness.GetSleepLog(r.Context(), userID, id)
	if err != nil {
		writeErr(w, wellnessErrStatus(err), "sleep log not found")
		return
	}
	writeJSON(w, http.StatusOK, l)
}

func (s *Server) handleUpdateSleepLog(w http.ResponseWriter, r *http.Request) {
	userID, _ := auth.UserID(r.Context())
	id := r.PathValue("id")

	var in wellness.SleepLogInput
	if err := json.NewDecoder(r.Body).Decode(&in); err != nil {
		writeErr(w, http.StatusBadRequest, "invalid json body")
		return
	}

	l, err := s.wellness.UpdateSleepLog(r.Context(), userID, id, in)
	if err != nil {
		writeWellnessErr(w, err)
		return
	}
	writeJSON(w, http.StatusOK, l)
}

func (s *Server) handleDeleteSleepLog(w http.ResponseWriter, r *http.Request) {
	userID, _ := auth.UserID(r.Context())
	id := r.PathValue("id")

	if err := s.wellness.DeleteSleepLog(r.Context(), userID, id); err != nil {
		writeErr(w, wellnessErrStatus(err), "sleep log not found")
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// --- Mood ---

func (s *Server) handleCreateMoodLog(w http.ResponseWriter, r *http.Request) {
	userID, _ := auth.UserID(r.Context())

	var in wellness.MoodLogInput
	if err := json.NewDecoder(r.Body).Decode(&in); err != nil {
		writeErr(w, http.StatusBadRequest, "invalid json body")
		return
	}

	m, err := s.wellness.CreateMoodLog(r.Context(), userID, in)
	if err != nil {
		writeWellnessErr(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, m)
}

func (s *Server) handleListMoodLogs(w http.ResponseWriter, r *http.Request) {
	userID, _ := auth.UserID(r.Context())

	logs, err := s.wellness.ListMoodLogs(r.Context(), userID)
	if err != nil {
		writeUnexpectedErr(w, err)
		return
	}
	if logs == nil {
		logs = []wellness.MoodLog{}
	}
	writeJSON(w, http.StatusOK, logs)
}

func (s *Server) handleGetMoodLog(w http.ResponseWriter, r *http.Request) {
	userID, _ := auth.UserID(r.Context())
	id := r.PathValue("id")

	m, err := s.wellness.GetMoodLog(r.Context(), userID, id)
	if err != nil {
		writeErr(w, wellnessErrStatus(err), "mood log not found")
		return
	}
	writeJSON(w, http.StatusOK, m)
}

func (s *Server) handleUpdateMoodLog(w http.ResponseWriter, r *http.Request) {
	userID, _ := auth.UserID(r.Context())
	id := r.PathValue("id")

	var in wellness.MoodLogInput
	if err := json.NewDecoder(r.Body).Decode(&in); err != nil {
		writeErr(w, http.StatusBadRequest, "invalid json body")
		return
	}

	m, err := s.wellness.UpdateMoodLog(r.Context(), userID, id, in)
	if err != nil {
		writeWellnessErr(w, err)
		return
	}
	writeJSON(w, http.StatusOK, m)
}

func (s *Server) handleDeleteMoodLog(w http.ResponseWriter, r *http.Request) {
	userID, _ := auth.UserID(r.Context())
	id := r.PathValue("id")

	if err := s.wellness.DeleteMoodLog(r.Context(), userID, id); err != nil {
		writeErr(w, wellnessErrStatus(err), "mood log not found")
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
