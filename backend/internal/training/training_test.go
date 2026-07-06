package training

import (
	"context"
	"errors"
	"os"
	"testing"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

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

func TestExerciseAndSetSummary(t *testing.T) {
	store, userID := newTestStore(t)
	ctx := context.Background()

	bench, err := store.CreateExercise(ctx, userID, ExerciseInput{Name: "Bench press", Category: "Push"})
	if err != nil {
		t.Fatalf("CreateExercise failed: %v", err)
	}

	if _, err := store.CreateSet(ctx, userID, SetInput{ExerciseID: bench.ID, PerformedOn: "2026-07-05", Weight: 135, Reps: 5}); err != nil {
		t.Fatalf("CreateSet old failed: %v", err)
	}
	todaySet, err := store.CreateSet(ctx, userID, SetInput{ExerciseID: bench.ID, PerformedOn: "2026-07-06", Weight: 145, Reps: 5})
	if err != nil {
		t.Fatalf("CreateSet today failed: %v", err)
	}
	if todaySet.Volume != 725 {
		t.Fatalf("unexpected set volume: %+v", todaySet)
	}

	summary, err := store.Summary(ctx, userID, "2026-07-06")
	if err != nil {
		t.Fatalf("Summary failed: %v", err)
	}
	if len(summary.TodaySets) != 1 || summary.WeekSets != 2 || summary.WeekVolume != 1400 {
		t.Fatalf("unexpected summary: %+v", summary)
	}
	if len(summary.PRs) != 1 || summary.PRs[0].Weight != 145 {
		t.Fatalf("expected 145lb PR, got %+v", summary.PRs)
	}
}

func TestSetRejectsOtherUsersExercise(t *testing.T) {
	store, userA := newTestStore(t)
	_, userB := newTestStore(t)
	ctx := context.Background()

	otherExercise, err := store.CreateExercise(ctx, userB, ExerciseInput{Name: "Squat"})
	if err != nil {
		t.Fatalf("CreateExercise for user B failed: %v", err)
	}

	_, err = store.CreateSet(ctx, userA, SetInput{
		ExerciseID:  otherExercise.ID,
		PerformedOn: "2026-07-06",
		Weight:      225,
		Reps:        3,
	})
	var invalid ErrInvalidInput
	if !errors.As(err, &invalid) {
		t.Fatalf("expected ErrInvalidInput for another user's exercise, got %v", err)
	}
}

func TestExerciseValidationAndDelete(t *testing.T) {
	store, userID := newTestStore(t)
	ctx := context.Background()

	_, err := store.CreateExercise(ctx, userID, ExerciseInput{Name: ""})
	var invalid ErrInvalidInput
	if !errors.As(err, &invalid) {
		t.Fatalf("expected ErrInvalidInput for empty name, got %v", err)
	}

	exercise, err := store.CreateExercise(ctx, userID, ExerciseInput{Name: "Deadlift"})
	if err != nil {
		t.Fatalf("CreateExercise failed: %v", err)
	}
	if err := store.DeleteExercise(ctx, userID, exercise.ID); err != nil {
		t.Fatalf("DeleteExercise failed: %v", err)
	}
	if _, err := store.GetExercise(ctx, userID, exercise.ID); !errors.Is(err, pgx.ErrNoRows) {
		t.Fatalf("expected ErrNoRows after delete, got %v", err)
	}
}

func TestTemplateCRUDAndOrder(t *testing.T) {
	store, userID := newTestStore(t)
	ctx := context.Background()

	squat, err := store.CreateExercise(ctx, userID, ExerciseInput{Name: "Squat"})
	if err != nil {
		t.Fatalf("CreateExercise squat failed: %v", err)
	}
	bench, err := store.CreateExercise(ctx, userID, ExerciseInput{Name: "Bench press"})
	if err != nil {
		t.Fatalf("CreateExercise bench failed: %v", err)
	}

	// "Bench press" sorts before "Squat" alphabetically - use squat-then-bench
	// order to prove position (not name) drives ordering.
	template, err := store.CreateTemplate(ctx, userID, TemplateInput{
		Name:  "Leg Day A",
		Items: []TemplateItemInput{{ExerciseID: squat.ID}, {ExerciseID: bench.ID}},
	})
	if err != nil {
		t.Fatalf("CreateTemplate failed: %v", err)
	}
	if len(template.Items) != 2 || template.Items[0].ExerciseID != squat.ID || template.Items[1].ExerciseID != bench.ID {
		t.Fatalf("expected [squat, bench] order, got %+v", template.Items)
	}

	fetched, err := store.GetTemplate(ctx, userID, template.ID)
	if err != nil {
		t.Fatalf("GetTemplate failed: %v", err)
	}
	if len(fetched.Items) != 2 || fetched.Items[0].ExerciseID != squat.ID || fetched.Items[1].ExerciseID != bench.ID {
		t.Fatalf("expected GetTemplate to preserve order, got %+v", fetched.Items)
	}

	list, err := store.ListTemplates(ctx, userID)
	if err != nil {
		t.Fatalf("ListTemplates failed: %v", err)
	}
	if len(list) != 1 || len(list[0].Items) != 2 || list[0].Items[0].ExerciseID != squat.ID {
		t.Fatalf("expected ListTemplates to preserve order, got %+v", list)
	}

	if err := store.DeleteTemplate(ctx, userID, template.ID); err != nil {
		t.Fatalf("DeleteTemplate failed: %v", err)
	}
	if _, err := store.GetTemplate(ctx, userID, template.ID); !errors.Is(err, pgx.ErrNoRows) {
		t.Fatalf("expected ErrNoRows after delete, got %v", err)
	}
}

func TestTemplateRejectsOtherUsersExercise(t *testing.T) {
	store, userA := newTestStore(t)
	_, userB := newTestStore(t)
	ctx := context.Background()

	otherExercise, err := store.CreateExercise(ctx, userB, ExerciseInput{Name: "Leg press"})
	if err != nil {
		t.Fatalf("CreateExercise for user B failed: %v", err)
	}

	_, err = store.CreateTemplate(ctx, userA, TemplateInput{
		Name:  "Leg Day A",
		Items: []TemplateItemInput{{ExerciseID: otherExercise.ID}},
	})
	var invalid ErrInvalidInput
	if !errors.As(err, &invalid) {
		t.Fatalf("expected ErrInvalidInput for another user's exercise, got %v", err)
	}
}
