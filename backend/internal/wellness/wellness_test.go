package wellness

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

func strPtr(s string) *string { return &s }
func intPtr(i int) *int       { return &i }

func TestBodyMetricCRUD(t *testing.T) {
	store, userID := newTestStore(t)
	ctx := context.Background()

	created, err := store.CreateBodyMetric(ctx, userID, BodyMetricInput{LogDate: "2026-07-06", Weight: 180.5, Notes: strPtr("morning")})
	if err != nil {
		t.Fatalf("CreateBodyMetric failed: %v", err)
	}
	if created.Weight != 180.5 {
		t.Fatalf("unexpected created metric: %+v", created)
	}

	updated, err := store.UpdateBodyMetric(ctx, userID, created.ID, BodyMetricInput{LogDate: "2026-07-06", Weight: 179.0})
	if err != nil {
		t.Fatalf("UpdateBodyMetric failed: %v", err)
	}
	if updated.Weight != 179.0 {
		t.Fatalf("Update didn't apply: %+v", updated)
	}

	if _, err := store.CreateBodyMetric(ctx, userID, BodyMetricInput{LogDate: "2026-07-06", Weight: -1}); err == nil {
		t.Fatalf("expected ErrInvalidInput for non-positive weight")
	}

	if err := store.DeleteBodyMetric(ctx, userID, created.ID); err != nil {
		t.Fatalf("DeleteBodyMetric failed: %v", err)
	}
	if _, err := store.GetBodyMetric(ctx, userID, created.ID); !errors.Is(err, pgx.ErrNoRows) {
		t.Fatalf("expected ErrNoRows after delete, got %v", err)
	}
}

func TestSleepLogCRUD(t *testing.T) {
	store, userID := newTestStore(t)
	ctx := context.Background()

	created, err := store.CreateSleepLog(ctx, userID, SleepLogInput{LogDate: "2026-07-06", Hours: 7.5, Quality: intPtr(4)})
	if err != nil {
		t.Fatalf("CreateSleepLog failed: %v", err)
	}
	if created.Hours != 7.5 || created.Quality == nil || *created.Quality != 4 {
		t.Fatalf("unexpected created log: %+v", created)
	}

	updated, err := store.UpdateSleepLog(ctx, userID, created.ID, SleepLogInput{LogDate: "2026-07-06", Hours: 8})
	if err != nil {
		t.Fatalf("UpdateSleepLog failed: %v", err)
	}
	if updated.Hours != 8 {
		t.Fatalf("Update didn't apply: %+v", updated)
	}

	if _, err := store.CreateSleepLog(ctx, userID, SleepLogInput{LogDate: "2026-07-06", Hours: 30}); err == nil {
		t.Fatalf("expected ErrInvalidInput for hours out of range")
	}

	if err := store.DeleteSleepLog(ctx, userID, created.ID); err != nil {
		t.Fatalf("DeleteSleepLog failed: %v", err)
	}
	if _, err := store.GetSleepLog(ctx, userID, created.ID); !errors.Is(err, pgx.ErrNoRows) {
		t.Fatalf("expected ErrNoRows after delete, got %v", err)
	}
}

func TestMoodLogCRUD(t *testing.T) {
	store, userID := newTestStore(t)
	ctx := context.Background()

	created, err := store.CreateMoodLog(ctx, userID, MoodLogInput{LogDate: "2026-07-06", Mood: 4, Notes: strPtr("good day")})
	if err != nil {
		t.Fatalf("CreateMoodLog failed: %v", err)
	}
	if created.Mood != 4 {
		t.Fatalf("unexpected created log: %+v", created)
	}

	updated, err := store.UpdateMoodLog(ctx, userID, created.ID, MoodLogInput{LogDate: "2026-07-06", Mood: 2})
	if err != nil {
		t.Fatalf("UpdateMoodLog failed: %v", err)
	}
	if updated.Mood != 2 {
		t.Fatalf("Update didn't apply: %+v", updated)
	}

	if _, err := store.CreateMoodLog(ctx, userID, MoodLogInput{LogDate: "2026-07-06", Mood: 9}); err == nil {
		t.Fatalf("expected ErrInvalidInput for mood out of range")
	}

	if err := store.DeleteMoodLog(ctx, userID, created.ID); err != nil {
		t.Fatalf("DeleteMoodLog failed: %v", err)
	}
	if _, err := store.GetMoodLog(ctx, userID, created.ID); !errors.Is(err, pgx.ErrNoRows) {
		t.Fatalf("expected ErrNoRows after delete, got %v", err)
	}
}
