package server

import (
	"encoding/json"
	"errors"
	"net/http"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"

	"github.com/justin/todomax/backend/internal/auth"
	"github.com/justin/todomax/backend/internal/nutrition"
)

func nutritionErrStatus(err error) int {
	if errors.Is(err, pgx.ErrNoRows) {
		return http.StatusNotFound
	}
	var invalid nutrition.ErrInvalidInput
	if errors.As(err, &invalid) {
		return http.StatusBadRequest
	}
	return http.StatusInternalServerError
}

func writeNutritionErr(w http.ResponseWriter, err error) {
	status := nutritionErrStatus(err)
	if status == http.StatusInternalServerError {
		writeUnexpectedErr(w, err)
		return
	}
	writeErr(w, status, err.Error())
}

// --- Foods ---

func (s *Server) handleCreateFood(w http.ResponseWriter, r *http.Request) {
	userID, _ := auth.UserID(r.Context())

	var in nutrition.FoodInput
	if err := json.NewDecoder(r.Body).Decode(&in); err != nil {
		writeErr(w, http.StatusBadRequest, "invalid json body")
		return
	}
	in.Name = strings.TrimSpace(in.Name)

	f, err := s.nutrition.CreateFood(r.Context(), userID, in)
	if err != nil {
		writeNutritionErr(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, f)
}

func (s *Server) handleListFoods(w http.ResponseWriter, r *http.Request) {
	userID, _ := auth.UserID(r.Context())

	foods, err := s.nutrition.ListFoods(r.Context(), userID)
	if err != nil {
		writeUnexpectedErr(w, err)
		return
	}
	if foods == nil {
		foods = []nutrition.Food{}
	}
	writeJSON(w, http.StatusOK, foods)
}

func (s *Server) handleGetFood(w http.ResponseWriter, r *http.Request) {
	userID, _ := auth.UserID(r.Context())
	id := r.PathValue("id")

	f, err := s.nutrition.GetFood(r.Context(), userID, id)
	if err != nil {
		writeErr(w, nutritionErrStatus(err), "food not found")
		return
	}
	writeJSON(w, http.StatusOK, f)
}

func (s *Server) handleUpdateFood(w http.ResponseWriter, r *http.Request) {
	userID, _ := auth.UserID(r.Context())
	id := r.PathValue("id")

	var in nutrition.FoodInput
	if err := json.NewDecoder(r.Body).Decode(&in); err != nil {
		writeErr(w, http.StatusBadRequest, "invalid json body")
		return
	}
	in.Name = strings.TrimSpace(in.Name)

	f, err := s.nutrition.UpdateFood(r.Context(), userID, id, in)
	if err != nil {
		writeNutritionErr(w, err)
		return
	}
	writeJSON(w, http.StatusOK, f)
}

func (s *Server) handleDeleteFood(w http.ResponseWriter, r *http.Request) {
	userID, _ := auth.UserID(r.Context())
	id := r.PathValue("id")

	if err := s.nutrition.DeleteFood(r.Context(), userID, id); err != nil {
		writeErr(w, nutritionErrStatus(err), "food not found")
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// --- Meals ---

func (s *Server) handleCreateMeal(w http.ResponseWriter, r *http.Request) {
	userID, _ := auth.UserID(r.Context())

	var in nutrition.MealInput
	if err := json.NewDecoder(r.Body).Decode(&in); err != nil {
		writeErr(w, http.StatusBadRequest, "invalid json body")
		return
	}
	in.Name = strings.TrimSpace(in.Name)

	m, err := s.nutrition.CreateMeal(r.Context(), userID, in)
	if err != nil {
		writeNutritionErr(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, m)
}

func (s *Server) handleListMeals(w http.ResponseWriter, r *http.Request) {
	userID, _ := auth.UserID(r.Context())

	meals, err := s.nutrition.ListMeals(r.Context(), userID)
	if err != nil {
		writeUnexpectedErr(w, err)
		return
	}
	if meals == nil {
		meals = []nutrition.Meal{}
	}
	writeJSON(w, http.StatusOK, meals)
}

func (s *Server) handleGetMeal(w http.ResponseWriter, r *http.Request) {
	userID, _ := auth.UserID(r.Context())
	id := r.PathValue("id")

	m, err := s.nutrition.GetMeal(r.Context(), userID, id)
	if err != nil {
		writeErr(w, nutritionErrStatus(err), "meal not found")
		return
	}
	writeJSON(w, http.StatusOK, m)
}

func (s *Server) handleUpdateMeal(w http.ResponseWriter, r *http.Request) {
	userID, _ := auth.UserID(r.Context())
	id := r.PathValue("id")

	var in nutrition.MealInput
	if err := json.NewDecoder(r.Body).Decode(&in); err != nil {
		writeErr(w, http.StatusBadRequest, "invalid json body")
		return
	}
	in.Name = strings.TrimSpace(in.Name)

	m, err := s.nutrition.UpdateMeal(r.Context(), userID, id, in)
	if err != nil {
		writeNutritionErr(w, err)
		return
	}
	writeJSON(w, http.StatusOK, m)
}

func (s *Server) handleDeleteMeal(w http.ResponseWriter, r *http.Request) {
	userID, _ := auth.UserID(r.Context())
	id := r.PathValue("id")

	if err := s.nutrition.DeleteMeal(r.Context(), userID, id); err != nil {
		writeErr(w, nutritionErrStatus(err), "meal not found")
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// --- Log entries + day summary ---

func (s *Server) handleCreateLogEntry(w http.ResponseWriter, r *http.Request) {
	userID, _ := auth.UserID(r.Context())

	var in nutrition.LogEntryInput
	if err := json.NewDecoder(r.Body).Decode(&in); err != nil {
		writeErr(w, http.StatusBadRequest, "invalid json body")
		return
	}

	entry, err := s.nutrition.CreateLogEntry(r.Context(), userID, in)
	if err != nil {
		writeNutritionErr(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, entry)
}

func (s *Server) handleDeleteLogEntry(w http.ResponseWriter, r *http.Request) {
	userID, _ := auth.UserID(r.Context())
	id := r.PathValue("id")

	if err := s.nutrition.DeleteLogEntry(r.Context(), userID, id); err != nil {
		writeErr(w, nutritionErrStatus(err), "log entry not found")
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) handleDaySummary(w http.ResponseWriter, r *http.Request) {
	userID, _ := auth.UserID(r.Context())

	date := r.URL.Query().Get("date")
	if date == "" {
		date = time.Now().In(s.cfg.Location).Format("2006-01-02")
	}

	summary, err := s.nutrition.DaySummary(r.Context(), userID, date)
	if err != nil {
		writeUnexpectedErr(w, err)
		return
	}
	writeJSON(w, http.StatusOK, summary)
}

// --- Target ---

func (s *Server) handleGetTarget(w http.ResponseWriter, r *http.Request) {
	userID, _ := auth.UserID(r.Context())

	target, err := s.nutrition.GetTarget(r.Context(), userID)
	if err != nil {
		writeUnexpectedErr(w, err)
		return
	}
	writeJSON(w, http.StatusOK, target)
}

func (s *Server) handleSetTarget(w http.ResponseWriter, r *http.Request) {
	userID, _ := auth.UserID(r.Context())

	var in nutrition.Target
	if err := json.NewDecoder(r.Body).Decode(&in); err != nil {
		writeErr(w, http.StatusBadRequest, "invalid json body")
		return
	}

	target, err := s.nutrition.UpsertTarget(r.Context(), userID, in)
	if err != nil {
		writeUnexpectedErr(w, err)
		return
	}
	writeJSON(w, http.StatusOK, target)
}
