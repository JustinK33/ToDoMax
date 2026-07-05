// Package nutrition is a calorie/macro-tracking MVP: reusable Foods and
// Meals, a per-day log of what was eaten, and an optional daily target
// (calories/protein/carbs/fat) to compare against. Macros are computed live
// from each food's current values at read time, not frozen when logged -
// editing a food's macros later changes past logged days' totals too. This
// is the simpler MVP option; a snapshot-at-log-time design would need a
// denormalized copy table and more write-path code.
package nutrition

import (
	"context"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// Food is a reusable definition of something a user eats, with macros
// per one serving (whatever "serving" means to the user - ServingLabel is
// just a display string like "100g" or "1 cup", not a parsed unit).
type Food struct {
	ID           string  `json:"id"`
	Name         string  `json:"name"`
	ServingLabel string  `json:"serving_label"`
	Calories     float64 `json:"calories"`
	ProteinG     float64 `json:"protein_g"`
	CarbsG       float64 `json:"carbs_g"`
	FatG         float64 `json:"fat_g"`
}

// FoodInput is the subset of Food fields a client may set on create/update.
type FoodInput struct {
	Name         string  `json:"name"`
	ServingLabel string  `json:"serving_label"`
	Calories     float64 `json:"calories"`
	ProteinG     float64 `json:"protein_g"`
	CarbsG       float64 `json:"carbs_g"`
	FatG         float64 `json:"fat_g"`
}

// ErrInvalidInput is returned when input fields don't satisfy a validation
// rule.
type ErrInvalidInput struct{ Reason string }

func (e ErrInvalidInput) Error() string { return e.Reason }

func validateFoodInput(in FoodInput) error {
	if in.Name == "" {
		return ErrInvalidInput{"name is required"}
	}
	if in.Calories < 0 || in.ProteinG < 0 || in.CarbsG < 0 || in.FatG < 0 {
		return ErrInvalidInput{"macros must not be negative"}
	}
	return nil
}

type Store struct {
	db *pgxpool.Pool
}

func NewStore(db *pgxpool.Pool) *Store { return &Store{db: db} }

const foodColumns = "id, name, serving_label, calories, protein_g, carbs_g, fat_g"

func scanFood(row pgx.Row) (Food, error) {
	var f Food
	err := row.Scan(&f.ID, &f.Name, &f.ServingLabel, &f.Calories, &f.ProteinG, &f.CarbsG, &f.FatG)
	return f, err
}

func (s *Store) CreateFood(ctx context.Context, userID string, in FoodInput) (Food, error) {
	if err := validateFoodInput(in); err != nil {
		return Food{}, err
	}
	if in.ServingLabel == "" {
		in.ServingLabel = "serving"
	}

	row := s.db.QueryRow(ctx, `
		insert into foods (user_id, name, serving_label, calories, protein_g, carbs_g, fat_g)
		values ($1, $2, $3, $4, $5, $6, $7)
		returning `+foodColumns,
		userID, in.Name, in.ServingLabel, in.Calories, in.ProteinG, in.CarbsG, in.FatG,
	)
	return scanFood(row)
}

func (s *Store) ListFoods(ctx context.Context, userID string) ([]Food, error) {
	rows, err := s.db.Query(ctx, `select `+foodColumns+` from foods where user_id = $1 order by name`, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var foods []Food
	for rows.Next() {
		f, err := scanFood(rows)
		if err != nil {
			return nil, err
		}
		foods = append(foods, f)
	}
	return foods, rows.Err()
}

func (s *Store) GetFood(ctx context.Context, userID, id string) (Food, error) {
	row := s.db.QueryRow(ctx, `select `+foodColumns+` from foods where user_id = $1 and id = $2`, userID, id)
	return scanFood(row)
}

func (s *Store) UpdateFood(ctx context.Context, userID, id string, in FoodInput) (Food, error) {
	if err := validateFoodInput(in); err != nil {
		return Food{}, err
	}
	if in.ServingLabel == "" {
		in.ServingLabel = "serving"
	}

	row := s.db.QueryRow(ctx, `
		update foods
		set name = $3, serving_label = $4, calories = $5, protein_g = $6, carbs_g = $7, fat_g = $8, updated_at = now()
		where user_id = $1 and id = $2
		returning `+foodColumns,
		userID, id, in.Name, in.ServingLabel, in.Calories, in.ProteinG, in.CarbsG, in.FatG,
	)
	return scanFood(row)
}

func (s *Store) DeleteFood(ctx context.Context, userID, id string) error {
	tag, err := s.db.Exec(ctx, `delete from foods where user_id = $1 and id = $2`, userID, id)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return pgx.ErrNoRows
	}
	return nil
}
