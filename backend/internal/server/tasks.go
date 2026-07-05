package server

import (
	"encoding/json"
	"errors"
	"log"
	"net/http"
	"strings"
	"time"

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

// writeUnexpectedErr logs the real error server-side (a 500 here is
// otherwise invisible to anyone but the caller) and returns a message with
// no internal detail - raw driver/query errors can include table or column
// names that shouldn't reach the client.
func writeUnexpectedErr(w http.ResponseWriter, err error) {
	log.Printf("server: unexpected error: %v", err)
	writeErr(w, http.StatusInternalServerError, "internal server error")
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

// writeTaskErr writes a Store error as an HTTP response - ErrInvalidInput's
// message is meant to be user-facing (it names the validation that failed),
// but anything that maps to a 500 goes through writeUnexpectedErr instead of
// exposing the raw error.
func writeTaskErr(w http.ResponseWriter, err error) {
	status := taskErrStatus(err)
	if status == http.StatusInternalServerError {
		writeUnexpectedErr(w, err)
		return
	}
	writeErr(w, status, err.Error())
}

func (s *Server) handleCreateTask(w http.ResponseWriter, r *http.Request) {
	userID, _ := auth.UserID(r.Context())

	var in task.Input
	if err := json.NewDecoder(r.Body).Decode(&in); err != nil {
		writeErr(w, http.StatusBadRequest, "invalid json body")
		return
	}
	in.Title = strings.TrimSpace(in.Title)
	if in.Title == "" {
		writeErr(w, http.StatusBadRequest, "title is required")
		return
	}

	t, err := s.tasks.Create(r.Context(), userID, in)
	if err != nil {
		writeTaskErr(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, t)
}

func (s *Server) handleListTasks(w http.ResponseWriter, r *http.Request) {
	userID, _ := auth.UserID(r.Context())

	filter := task.Filter{
		View:     r.URL.Query().Get("view"),
		Category: r.URL.Query().Get("category"),
		Now:      time.Now().In(s.cfg.Location),
	}

	// today/week/upcoming expand recurring tasks into per-day occurrences;
	// overdue/all/"" list task templates (a recurring task is never itself
	// "overdue", and the All tab manages the templates, not their instances).
	switch filter.View {
	case "today", "week", "upcoming":
		occurrences, err := s.tasks.ListOccurrences(r.Context(), userID, filter)
		if err != nil {
			writeUnexpectedErr(w, err)
			return
		}
		if occurrences == nil {
			occurrences = []task.Occurrence{}
		}
		writeJSON(w, http.StatusOK, occurrences)
	default:
		tasks, err := s.tasks.List(r.Context(), userID, filter)
		if err != nil {
			writeUnexpectedErr(w, err)
			return
		}
		if tasks == nil {
			tasks = []task.Task{}
		}
		writeJSON(w, http.StatusOK, tasks)
	}
}

func (s *Server) handleWeekSummary(w http.ResponseWriter, r *http.Request) {
	userID, _ := auth.UserID(r.Context())

	summary, err := s.tasks.WeekSummary(r.Context(), userID, time.Now().In(s.cfg.Location))
	if err != nil {
		writeUnexpectedErr(w, err)
		return
	}
	writeJSON(w, http.StatusOK, summary)
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
	in.Title = strings.TrimSpace(in.Title)
	if in.Title == "" {
		writeErr(w, http.StatusBadRequest, "title is required")
		return
	}

	t, err := s.tasks.Update(r.Context(), userID, id, in)
	if err != nil {
		writeTaskErr(w, err)
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

	var body struct {
		OccurrenceDate string `json:"occurrence_date"`
	}
	json.NewDecoder(r.Body).Decode(&body) // no body sent = zero value, defaulted below

	occurrenceDate := body.OccurrenceDate
	if occurrenceDate == "" {
		occurrenceDate = time.Now().In(s.cfg.Location).Format("2006-01-02")
	}

	occ, err := s.tasks.SetOccurrenceCompleted(r.Context(), userID, id, occurrenceDate, completed)
	if err != nil {
		writeErr(w, taskErrStatus(err), "task not found")
		return
	}
	writeJSON(w, http.StatusOK, occ)
}
