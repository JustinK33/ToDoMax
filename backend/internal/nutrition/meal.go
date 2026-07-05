package nutrition

import (
	"context"

	"github.com/jackc/pgx/v5"
)

// MealItem is one food+quantity line within a meal, expanded on read.
type MealItem struct {
	FoodID   string  `json:"food_id"`
	FoodName string  `json:"food_name"`
	Servings float64 `json:"servings"`
}

// MealItemInput is what a client sends to compose a meal.
type MealItemInput struct {
	FoodID   string  `json:"food_id"`
	Servings float64 `json:"servings"`
}

// Meal is a reusable named combination of foods. Totals are pre-summed
// server-side (servings-weighted) so the frontend doesn't need to re-derive
// them from Items.
type Meal struct {
	ID            string     `json:"id"`
	Name          string     `json:"name"`
	Items         []MealItem `json:"items"`
	TotalCalories float64    `json:"total_calories"`
	TotalProteinG float64    `json:"total_protein_g"`
	TotalCarbsG   float64    `json:"total_carbs_g"`
	TotalFatG     float64    `json:"total_fat_g"`
}

// MealInput is the subset of Meal fields a client may set on create/update.
type MealInput struct {
	Name  string          `json:"name"`
	Items []MealItemInput `json:"items"`
}

func validateMealInput(in MealInput) error {
	if in.Name == "" {
		return ErrInvalidInput{"name is required"}
	}
	for _, item := range in.Items {
		if item.FoodID == "" || item.Servings <= 0 {
			return ErrInvalidInput{"each meal item needs a food and a positive servings value"}
		}
	}
	return nil
}

// ownedFoodIDs returns which of the given food IDs belong to userID - used
// to reject a meal item referencing someone else's (or a nonexistent) food,
// since the meal_items table's foreign key only checks existence, not
// ownership.
func (s *Store) ownedFoodIDs(ctx context.Context, userID string, foodIDs []string) (map[string]bool, error) {
	rows, err := s.db.Query(ctx, `select id from foods where user_id = $1 and id = any($2::uuid[])`, userID, foodIDs)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	owned := map[string]bool{}
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		owned[id] = true
	}
	return owned, rows.Err()
}

func (s *Store) writeMealItems(ctx context.Context, tx pgx.Tx, userID, mealID string, items []MealItemInput) error {
	if len(items) == 0 {
		return nil
	}
	foodIDs := make([]string, len(items))
	for i, item := range items {
		foodIDs[i] = item.FoodID
	}
	owned, err := s.ownedFoodIDs(ctx, userID, foodIDs)
	if err != nil {
		return err
	}
	for _, item := range items {
		if !owned[item.FoodID] {
			return ErrInvalidInput{"food not found"}
		}
		if _, err := tx.Exec(ctx, `insert into meal_items (meal_id, food_id, servings) values ($1, $2, $3)`,
			mealID, item.FoodID, item.Servings); err != nil {
			return err
		}
	}
	return nil
}

func (s *Store) CreateMeal(ctx context.Context, userID string, in MealInput) (Meal, error) {
	if err := validateMealInput(in); err != nil {
		return Meal{}, err
	}

	tx, err := s.db.Begin(ctx)
	if err != nil {
		return Meal{}, err
	}
	defer tx.Rollback(ctx)

	var mealID string
	if err := tx.QueryRow(ctx, `insert into meals (user_id, name) values ($1, $2) returning id`, userID, in.Name).Scan(&mealID); err != nil {
		return Meal{}, err
	}
	if err := s.writeMealItems(ctx, tx, userID, mealID, in.Items); err != nil {
		return Meal{}, err
	}
	if err := tx.Commit(ctx); err != nil {
		return Meal{}, err
	}
	return s.GetMeal(ctx, userID, mealID)
}

func (s *Store) UpdateMeal(ctx context.Context, userID, id string, in MealInput) (Meal, error) {
	if err := validateMealInput(in); err != nil {
		return Meal{}, err
	}

	tx, err := s.db.Begin(ctx)
	if err != nil {
		return Meal{}, err
	}
	defer tx.Rollback(ctx)

	tag, err := tx.Exec(ctx, `update meals set name = $3, updated_at = now() where user_id = $1 and id = $2`, userID, id, in.Name)
	if err != nil {
		return Meal{}, err
	}
	if tag.RowsAffected() == 0 {
		return Meal{}, pgx.ErrNoRows
	}
	if _, err := tx.Exec(ctx, `delete from meal_items where meal_id = $1`, id); err != nil {
		return Meal{}, err
	}
	if err := s.writeMealItems(ctx, tx, userID, id, in.Items); err != nil {
		return Meal{}, err
	}
	if err := tx.Commit(ctx); err != nil {
		return Meal{}, err
	}
	return s.GetMeal(ctx, userID, id)
}

func (s *Store) ListMeals(ctx context.Context, userID string) ([]Meal, error) {
	rows, err := s.db.Query(ctx, `select id, name from meals where user_id = $1 order by name`, userID)
	if err != nil {
		return nil, err
	}
	type mealRow struct{ id, name string }
	var mealRows []mealRow
	for rows.Next() {
		var mr mealRow
		if err := rows.Scan(&mr.id, &mr.name); err != nil {
			rows.Close()
			return nil, err
		}
		mealRows = append(mealRows, mr)
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return nil, err
	}

	ids := make([]string, len(mealRows))
	for i, mr := range mealRows {
		ids[i] = mr.id
	}
	itemsByMeal, err := s.mealItemsWithMacros(ctx, ids)
	if err != nil {
		return nil, err
	}

	meals := make([]Meal, len(mealRows))
	for i, mr := range mealRows {
		meals[i] = totalMeal(mr.id, mr.name, itemsByMeal[mr.id])
	}
	return meals, nil
}

func (s *Store) GetMeal(ctx context.Context, userID, id string) (Meal, error) {
	var name string
	if err := s.db.QueryRow(ctx, `select name from meals where user_id = $1 and id = $2`, userID, id).Scan(&name); err != nil {
		return Meal{}, err
	}
	itemsByMeal, err := s.mealItemsWithMacros(ctx, []string{id})
	if err != nil {
		return Meal{}, err
	}
	return totalMeal(id, name, itemsByMeal[id]), nil
}

func (s *Store) DeleteMeal(ctx context.Context, userID, id string) error {
	tag, err := s.db.Exec(ctx, `delete from meals where user_id = $1 and id = $2`, userID, id)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return pgx.ErrNoRows
	}
	return nil
}

type mealItemWithMacros struct {
	MealItem
	calories, proteinG, carbsG, fatG float64
}

func (s *Store) mealItemsWithMacros(ctx context.Context, mealIDs []string) (map[string][]mealItemWithMacros, error) {
	if len(mealIDs) == 0 {
		return map[string][]mealItemWithMacros{}, nil
	}
	rows, err := s.db.Query(ctx, `
		select mi.meal_id, f.id, f.name, mi.servings, f.calories, f.protein_g, f.carbs_g, f.fat_g
		from meal_items mi
		join foods f on f.id = mi.food_id
		where mi.meal_id = any($1::uuid[])
		order by f.name`,
		mealIDs,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	byMeal := map[string][]mealItemWithMacros{}
	for rows.Next() {
		var mealID string
		var item mealItemWithMacros
		if err := rows.Scan(&mealID, &item.FoodID, &item.FoodName, &item.Servings,
			&item.calories, &item.proteinG, &item.carbsG, &item.fatG); err != nil {
			return nil, err
		}
		byMeal[mealID] = append(byMeal[mealID], item)
	}
	return byMeal, rows.Err()
}

func totalMeal(id, name string, items []mealItemWithMacros) Meal {
	m := Meal{ID: id, Name: name}
	for _, item := range items {
		m.Items = append(m.Items, item.MealItem)
		m.TotalCalories += item.calories * item.Servings
		m.TotalProteinG += item.proteinG * item.Servings
		m.TotalCarbsG += item.carbsG * item.Servings
		m.TotalFatG += item.fatG * item.Servings
	}
	return m
}
