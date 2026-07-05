package nutrition

import (
	"context"
	"errors"

	"github.com/jackc/pgx/v5"
)

// Target is a user's optional daily nutrition target. All fields nullable -
// a user can set just a protein target and leave the rest unset.
type Target struct {
	Calories *int `json:"calories"`
	ProteinG *int `json:"protein_g"`
	CarbsG   *int `json:"carbs_g"`
	FatG     *int `json:"fat_g"`
}

func (s *Store) GetTarget(ctx context.Context, userID string) (Target, error) {
	var t Target
	err := s.db.QueryRow(ctx, `
		select calories, protein_g, carbs_g, fat_g from nutrition_targets where user_id = $1`,
		userID,
	).Scan(&t.Calories, &t.ProteinG, &t.CarbsG, &t.FatG)
	if err != nil {
		// No target set yet is a normal, expected state - not an error.
		if errors.Is(err, pgx.ErrNoRows) {
			return Target{}, nil
		}
		return Target{}, err
	}
	return t, nil
}

func (s *Store) UpsertTarget(ctx context.Context, userID string, t Target) (Target, error) {
	_, err := s.db.Exec(ctx, `
		insert into nutrition_targets (user_id, calories, protein_g, carbs_g, fat_g)
		values ($1, $2, $3, $4, $5)
		on conflict (user_id) do update
		set calories = $2, protein_g = $3, carbs_g = $4, fat_g = $5, updated_at = now()`,
		userID, t.Calories, t.ProteinG, t.CarbsG, t.FatG,
	)
	if err != nil {
		return Target{}, err
	}
	return t, nil
}
