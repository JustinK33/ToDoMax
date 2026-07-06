package training

import (
	"context"
	"errors"
	"os"
	"testing"
	"time"

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

func TestVolumeHistoryZeroFillsUnloggedDays(t *testing.T) {
	store, userID := newTestStore(t)
	ctx := context.Background()

	squat, err := store.CreateExercise(ctx, userID, ExerciseInput{Name: "Squat"})
	if err != nil {
		t.Fatalf("CreateExercise failed: %v", err)
	}

	today := time.Date(2026, 7, 6, 0, 0, 0, 0, time.UTC)
	day1 := today.AddDate(0, 0, -2).Format("2006-01-02")
	day3 := today.Format("2006-01-02")
	if _, err := store.CreateSet(ctx, userID, SetInput{ExerciseID: squat.ID, PerformedOn: day1, Weight: 200, Reps: 5}); err != nil {
		t.Fatalf("CreateSet(day1) failed: %v", err)
	}
	if _, err := store.CreateSet(ctx, userID, SetInput{ExerciseID: squat.ID, PerformedOn: day3, Weight: 200, Reps: 5}); err != nil {
		t.Fatalf("CreateSet(day3) failed: %v", err)
	}

	history, err := store.VolumeHistory(ctx, userID, 3, today)
	if err != nil {
		t.Fatalf("VolumeHistory failed: %v", err)
	}
	if len(history) != 3 {
		t.Fatalf("expected 3 days, got %d", len(history))
	}
	if history[0].Volume != 1000 || history[0].Sets != 1 {
		t.Fatalf("expected day1 (1000 volume, 1 set), got %+v", history[0])
	}
	if history[1].Volume != 0 || history[1].Sets != 0 {
		t.Fatalf("expected the middle day to be zero-filled, got %+v", history[1])
	}
	if history[2].Volume != 1000 {
		t.Fatalf("expected today (1000 volume), got %+v", history[2])
	}
}

func TestTrainingLoggingStreakNotBrokenByTodayUnlogged(t *testing.T) {
	store, userID := newTestStore(t)
	ctx := context.Background()

	squat, err := store.CreateExercise(ctx, userID, ExerciseInput{Name: "Squat"})
	if err != nil {
		t.Fatalf("CreateExercise failed: %v", err)
	}

	today := time.Date(2026, 7, 6, 0, 0, 0, 0, time.UTC)
	yesterday := today.AddDate(0, 0, -1).Format("2006-01-02")
	dayBefore := today.AddDate(0, 0, -2).Format("2006-01-02")
	for _, d := range []string{yesterday, dayBefore} {
		if _, err := store.CreateSet(ctx, userID, SetInput{ExerciseID: squat.ID, PerformedOn: d, Weight: 200, Reps: 5}); err != nil {
			t.Fatalf("CreateSet(%s) failed: %v", d, err)
		}
	}

	streak, err := store.LoggingStreak(ctx, userID, today)
	if err != nil {
		t.Fatalf("LoggingStreak failed: %v", err)
	}
	if streak != 2 {
		t.Fatalf("expected streak of 2 (today not logged yet shouldn't break it), got %d", streak)
	}
}
