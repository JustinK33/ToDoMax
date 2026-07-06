package nutrition

import (
	"context"
	"errors"
	"os"
	"testing"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// newTestStore connects to the database configured by DATABASE_URL (the
// local Supabase Postgres in dev, a service container in CI) and creates a
// throwaway auth.users row so the foreign keys are satisfiable, exactly
// like a real Supabase-authenticated user would be.
func newTestStore(t *testing.T) (*Store, string) {
	t.Helper()

	dsn := os.Getenv("DATABASE_URL")
	if dsn == "" {
		t.Skip("DATABASE_URL not set, skipping integration test")
	}

	db, err := pgxpool.New(context.Background(), dsn)
	if err != nil {
		t.Fatalf("failed to connect: %v", err)
	}
	t.Cleanup(db.Close)

	userID := uuid.New().String()
	if _, err := db.Exec(context.Background(), `insert into auth.users (id) values ($1)`, userID); err != nil {
		t.Fatalf("failed to create test user: %v", err)
	}
	t.Cleanup(func() {
		db.Exec(context.Background(), `delete from auth.users where id = $1`, userID)
	})

	return NewStore(db), userID
}

func TestFoodCRUD(t *testing.T) {
	store, userID := newTestStore(t)
	ctx := context.Background()

	created, err := store.CreateFood(ctx, userID, FoodInput{
		Name: "Chicken breast", ServingLabel: "100g", Calories: 165, ProteinG: 31, CarbsG: 0, FatG: 3.6,
	})
	if err != nil {
		t.Fatalf("CreateFood failed: %v", err)
	}
	if created.Name != "Chicken breast" || created.ProteinG != 31 {
		t.Fatalf("unexpected created food: %+v", created)
	}

	foods, err := store.ListFoods(ctx, userID)
	if err != nil {
		t.Fatalf("ListFoods failed: %v", err)
	}
	if len(foods) != 1 {
		t.Fatalf("expected 1 food, got %d", len(foods))
	}

	updated, err := store.UpdateFood(ctx, userID, created.ID, FoodInput{
		Name: "Chicken breast", ServingLabel: "100g", Calories: 165, ProteinG: 32, CarbsG: 0, FatG: 3.6,
	})
	if err != nil {
		t.Fatalf("UpdateFood failed: %v", err)
	}
	if updated.ProteinG != 32 {
		t.Fatalf("UpdateFood didn't apply: %+v", updated)
	}

	if err := store.DeleteFood(ctx, userID, created.ID); err != nil {
		t.Fatalf("DeleteFood failed: %v", err)
	}
	if _, err := store.GetFood(ctx, userID, created.ID); !errors.Is(err, pgx.ErrNoRows) {
		t.Fatalf("expected ErrNoRows after delete, got %v", err)
	}
}

func TestFoodValidation(t *testing.T) {
	store, userID := newTestStore(t)
	ctx := context.Background()

	_, err := store.CreateFood(ctx, userID, FoodInput{Name: "", Calories: 100})
	var invalid ErrInvalidInput
	if !errors.As(err, &invalid) {
		t.Fatalf("expected ErrInvalidInput for empty name, got %v", err)
	}

	_, err = store.CreateFood(ctx, userID, FoodInput{Name: "Bad food", Calories: -5})
	if !errors.As(err, &invalid) {
		t.Fatalf("expected ErrInvalidInput for negative calories, got %v", err)
	}
}

func TestMealRejectsOtherUsersFood(t *testing.T) {
	store, userA := newTestStore(t)
	_, userB := newTestStore(t)
	ctx := context.Background()

	bFood, err := store.CreateFood(ctx, userB, FoodInput{Name: "B's food", Calories: 100})
	if err != nil {
		t.Fatalf("CreateFood for user B failed: %v", err)
	}

	_, err = store.CreateMeal(ctx, userA, MealInput{
		Name:  "A's meal",
		Items: []MealItemInput{{FoodID: bFood.ID, Servings: 1}},
	})
	var invalid ErrInvalidInput
	if !errors.As(err, &invalid) {
		t.Fatalf("expected ErrInvalidInput when meal references another user's food, got %v", err)
	}
}

func TestMealTotalsAndDaySummary(t *testing.T) {
	store, userID := newTestStore(t)
	ctx := context.Background()

	chicken, err := store.CreateFood(ctx, userID, FoodInput{
		Name: "Chicken", ServingLabel: "100g", Calories: 165, ProteinG: 31, CarbsG: 0, FatG: 3.6,
	})
	if err != nil {
		t.Fatalf("CreateFood(chicken) failed: %v", err)
	}
	rice, err := store.CreateFood(ctx, userID, FoodInput{
		Name: "Rice", ServingLabel: "1 cup", Calories: 200, ProteinG: 4, CarbsG: 45, FatG: 0.4,
	})
	if err != nil {
		t.Fatalf("CreateFood(rice) failed: %v", err)
	}

	meal, err := store.CreateMeal(ctx, userID, MealInput{
		Name: "Chicken and rice",
		Items: []MealItemInput{
			{FoodID: chicken.ID, Servings: 2}, // 330 cal, 62g protein
			{FoodID: rice.ID, Servings: 1},    // 200 cal, 4g protein
		},
	})
	if err != nil {
		t.Fatalf("CreateMeal failed: %v", err)
	}
	if meal.TotalCalories != 530 || meal.TotalProteinG != 66 {
		t.Fatalf("unexpected meal totals: %+v", meal)
	}

	today := "2026-07-06"
	if _, err := store.CreateLogEntry(ctx, userID, LogEntryInput{LogDate: today, FoodID: &rice.ID, Servings: 1}); err != nil {
		t.Fatalf("CreateLogEntry(food) failed: %v", err)
	}
	if _, err := store.CreateLogEntry(ctx, userID, LogEntryInput{LogDate: today, MealID: &meal.ID, Servings: 1}); err != nil {
		t.Fatalf("CreateLogEntry(meal) failed: %v", err)
	}

	summary, err := store.DaySummary(ctx, userID, today)
	if err != nil {
		t.Fatalf("DaySummary failed: %v", err)
	}
	if len(summary.Entries) != 2 {
		t.Fatalf("expected 2 log entries, got %d", len(summary.Entries))
	}
	// rice (200 cal, 4g protein) + meal (530 cal, 66g protein) = 730 cal, 70g protein
	if summary.Totals.Calories != 730 || summary.Totals.ProteinG != 70 {
		t.Fatalf("unexpected day totals: %+v", summary.Totals)
	}
}

func TestUpdateLogEntry(t *testing.T) {
	store, userID := newTestStore(t)
	ctx := context.Background()

	rice, err := store.CreateFood(ctx, userID, FoodInput{
		Name: "Rice", ServingLabel: "1 cup", Calories: 200, ProteinG: 4, CarbsG: 45, FatG: 0.4,
	})
	if err != nil {
		t.Fatalf("CreateFood failed: %v", err)
	}

	entry, err := store.CreateLogEntry(ctx, userID, LogEntryInput{LogDate: "2026-07-06", FoodID: &rice.ID, Servings: 1})
	if err != nil {
		t.Fatalf("CreateLogEntry failed: %v", err)
	}

	updated, err := store.UpdateLogEntry(ctx, userID, entry.ID, LogEntryInput{LogDate: "2026-07-07", FoodID: &rice.ID, Servings: 2})
	if err != nil {
		t.Fatalf("UpdateLogEntry failed: %v", err)
	}
	if updated.LogDate != "2026-07-07" || updated.Servings != 2 || updated.Calories != 400 {
		t.Fatalf("unexpected updated entry: %+v", updated)
	}

	oldDay, err := store.DaySummary(ctx, userID, "2026-07-06")
	if err != nil {
		t.Fatalf("DaySummary(old date) failed: %v", err)
	}
	if len(oldDay.Entries) != 0 {
		t.Fatalf("expected no entries on the old date, got %+v", oldDay.Entries)
	}

	newDay, err := store.DaySummary(ctx, userID, "2026-07-07")
	if err != nil {
		t.Fatalf("DaySummary(new date) failed: %v", err)
	}
	if len(newDay.Entries) != 1 || newDay.Totals.Calories != 400 {
		t.Fatalf("expected the moved entry on the new date, got %+v", newDay)
	}
}

func TestUpsertTargetDoesNotDuplicate(t *testing.T) {
	store, userID := newTestStore(t)
	ctx := context.Background()

	protein := 150
	if _, err := store.UpsertTarget(ctx, userID, Target{ProteinG: &protein}); err != nil {
		t.Fatalf("first UpsertTarget failed: %v", err)
	}
	calories := 2200
	if _, err := store.UpsertTarget(ctx, userID, Target{Calories: &calories, ProteinG: &protein}); err != nil {
		t.Fatalf("second UpsertTarget failed: %v", err)
	}

	target, err := store.GetTarget(ctx, userID)
	if err != nil {
		t.Fatalf("GetTarget failed: %v", err)
	}
	if target.Calories == nil || *target.Calories != 2200 || target.ProteinG == nil || *target.ProteinG != 150 {
		t.Fatalf("unexpected target after upsert: %+v", target)
	}
}

func TestGetTargetWithNoneSetIsEmptyNotError(t *testing.T) {
	store, userID := newTestStore(t)
	ctx := context.Background()

	target, err := store.GetTarget(ctx, userID)
	if err != nil {
		t.Fatalf("GetTarget failed: %v", err)
	}
	if target.Calories != nil || target.ProteinG != nil {
		t.Fatalf("expected empty target, got %+v", target)
	}
}
