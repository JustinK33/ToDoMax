package nutrition

import (
	"context"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
)

// LogEntry is one day's logged food or meal, with macros resolved at read
// time (see package doc - live-computed, not frozen at log time).
type LogEntry struct {
	ID       string  `json:"id"`
	LogDate  string  `json:"log_date"`
	FoodID   *string `json:"food_id"`
	MealID   *string `json:"meal_id"`
	Name     string  `json:"name"`
	Servings float64 `json:"servings"`
	Calories float64 `json:"calories"`
	ProteinG float64 `json:"protein_g"`
	CarbsG   float64 `json:"carbs_g"`
	FatG     float64 `json:"fat_g"`
}

// LogEntryInput is what a client sends to log a food or meal for a day.
type LogEntryInput struct {
	LogDate  string  `json:"log_date"`
	FoodID   *string `json:"food_id"`
	MealID   *string `json:"meal_id"`
	Servings float64 `json:"servings"`
}

// Macros is a plain totals bundle, used both for a day's summed totals and
// for a user's target.
type Macros struct {
	Calories float64 `json:"calories"`
	ProteinG float64 `json:"protein_g"`
	CarbsG   float64 `json:"carbs_g"`
	FatG     float64 `json:"fat_g"`
}

// DaySummary backs the nutrition hero display: what was logged today, the
// totals, and the target to compare against.
type DaySummary struct {
	LogDate string     `json:"log_date"`
	Entries []LogEntry `json:"entries"`
	Totals  Macros     `json:"totals"`
	Target  Target     `json:"target"`
}

func validateLogEntryInput(in LogEntryInput) error {
	if (in.FoodID == nil) == (in.MealID == nil) {
		return ErrInvalidInput{"exactly one of food_id or meal_id is required"}
	}
	if in.Servings <= 0 {
		return ErrInvalidInput{"servings must be positive"}
	}
	if _, err := time.Parse("2006-01-02", in.LogDate); err != nil {
		return ErrInvalidInput{"log_date must be YYYY-MM-DD"}
	}
	return nil
}

func (s *Store) CreateLogEntry(ctx context.Context, userID string, in LogEntryInput) (LogEntry, error) {
	if err := validateLogEntryInput(in); err != nil {
		return LogEntry{}, err
	}

	// Confirm the referenced food/meal belongs to this user - a bad
	// reference is a validation problem (400), not "not found" (404).
	if in.FoodID != nil {
		if _, err := s.GetFood(ctx, userID, *in.FoodID); err != nil {
			return LogEntry{}, ErrInvalidInput{"food not found"}
		}
	} else {
		if _, err := s.GetMeal(ctx, userID, *in.MealID); err != nil {
			return LogEntry{}, ErrInvalidInput{"meal not found"}
		}
	}

	var id string
	err := s.db.QueryRow(ctx, `
		insert into food_log_entries (user_id, log_date, food_id, meal_id, servings)
		values ($1, $2, $3, $4, $5)
		returning id`,
		userID, in.LogDate, in.FoodID, in.MealID, in.Servings,
	).Scan(&id)
	if err != nil {
		return LogEntry{}, err
	}
	return s.getLogEntry(ctx, userID, id)
}

func (s *Store) UpdateLogEntry(ctx context.Context, userID, id string, in LogEntryInput) (LogEntry, error) {
	if err := validateLogEntryInput(in); err != nil {
		return LogEntry{}, err
	}

	if in.FoodID != nil {
		if _, err := s.GetFood(ctx, userID, *in.FoodID); err != nil {
			return LogEntry{}, ErrInvalidInput{"food not found"}
		}
	} else {
		if _, err := s.GetMeal(ctx, userID, *in.MealID); err != nil {
			return LogEntry{}, ErrInvalidInput{"meal not found"}
		}
	}

	tag, err := s.db.Exec(ctx, `
		update food_log_entries
		set log_date = $3, food_id = $4, meal_id = $5, servings = $6
		where user_id = $1 and id = $2`,
		userID, id, in.LogDate, in.FoodID, in.MealID, in.Servings,
	)
	if err != nil {
		return LogEntry{}, err
	}
	if tag.RowsAffected() == 0 {
		return LogEntry{}, pgx.ErrNoRows
	}
	return s.getLogEntry(ctx, userID, id)
}

func (s *Store) getLogEntry(ctx context.Context, userID, id string) (LogEntry, error) {
	entries, err := s.logEntriesForDate(ctx, userID, "", id)
	if err != nil {
		return LogEntry{}, err
	}
	if len(entries) == 0 {
		return LogEntry{}, pgx.ErrNoRows
	}
	return entries[0], nil
}

// logEntriesForDate resolves entries (each food-direct or meal-summed) into
// LogEntry with macros computed live. If id is non-empty, filters to that
// one entry regardless of date; otherwise filters by logDate.
func (s *Store) logEntriesForDate(ctx context.Context, userID, logDate, id string) ([]LogEntry, error) {
	var rows pgx.Rows
	var err error
	if id != "" {
		rows, err = s.db.Query(ctx, `
			select id, log_date, food_id, meal_id, servings
			from food_log_entries where user_id = $1 and id = $2`, userID, id)
	} else {
		rows, err = s.db.Query(ctx, `
			select id, log_date, food_id, meal_id, servings
			from food_log_entries where user_id = $1 and log_date = $2
			order by created_at`, userID, logDate)
	}
	if err != nil {
		return nil, err
	}
	type rawEntry struct {
		id             string
		logDate        pgtype.Date
		foodID, mealID *string
		servings       float64
	}
	var raw []rawEntry
	for rows.Next() {
		var re rawEntry
		if err := rows.Scan(&re.id, &re.logDate, &re.foodID, &re.mealID, &re.servings); err != nil {
			rows.Close()
			return nil, err
		}
		raw = append(raw, re)
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return nil, err
	}

	entries := make([]LogEntry, len(raw))
	for i, re := range raw {
		entries[i] = LogEntry{ID: re.id, LogDate: re.logDate.Time.Format("2006-01-02"), FoodID: re.foodID, MealID: re.mealID, Servings: re.servings}
		if re.foodID != nil {
			f, err := s.GetFood(ctx, userID, *re.foodID)
			if err != nil {
				return nil, err
			}
			entries[i].Name = f.Name
			entries[i].Calories = f.Calories * re.servings
			entries[i].ProteinG = f.ProteinG * re.servings
			entries[i].CarbsG = f.CarbsG * re.servings
			entries[i].FatG = f.FatG * re.servings
		} else {
			m, err := s.GetMeal(ctx, userID, *re.mealID)
			if err != nil {
				return nil, err
			}
			entries[i].Name = m.Name
			entries[i].Calories = m.TotalCalories * re.servings
			entries[i].ProteinG = m.TotalProteinG * re.servings
			entries[i].CarbsG = m.TotalCarbsG * re.servings
			entries[i].FatG = m.TotalFatG * re.servings
		}
	}
	return entries, nil
}

func (s *Store) DeleteLogEntry(ctx context.Context, userID, id string) error {
	tag, err := s.db.Exec(ctx, `delete from food_log_entries where user_id = $1 and id = $2`, userID, id)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return pgx.ErrNoRows
	}
	return nil
}

func (s *Store) DaySummary(ctx context.Context, userID, logDate string) (DaySummary, error) {
	entries, err := s.logEntriesForDate(ctx, userID, logDate, "")
	if err != nil {
		return DaySummary{}, err
	}
	if entries == nil {
		entries = []LogEntry{}
	}

	var totals Macros
	for _, e := range entries {
		totals.Calories += e.Calories
		totals.ProteinG += e.ProteinG
		totals.CarbsG += e.CarbsG
		totals.FatG += e.FatG
	}

	target, err := s.GetTarget(ctx, userID)
	if err != nil {
		return DaySummary{}, err
	}

	return DaySummary{LogDate: logDate, Entries: entries, Totals: totals, Target: target}, nil
}
