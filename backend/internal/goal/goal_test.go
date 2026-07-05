package goal

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
// throwaway auth.users row so the goals.user_id foreign key is satisfiable,
// exactly like a real Supabase-authenticated user would be.
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

func TestCreateGetUpdateDeleteComplete(t *testing.T) {
	store, userID := newTestStore(t)
	ctx := context.Background()

	created, err := store.Create(ctx, userID, Input{
		Title: "Read 12 books", Notes: strPtr("fiction and non-fiction"), Timeframe: "year", TargetDate: strPtr("2026-12-31"),
	})
	if err != nil {
		t.Fatalf("Create failed: %v", err)
	}
	if created.Timeframe != "year" || created.Completed {
		t.Fatalf("unexpected created goal: %+v", created)
	}

	got, err := store.Get(ctx, userID, created.ID)
	if err != nil {
		t.Fatalf("Get failed: %v", err)
	}
	if got.Title != "Read 12 books" {
		t.Fatalf("Get returned wrong goal: %+v", got)
	}

	updated, err := store.Update(ctx, userID, created.ID, Input{
		Title: "Read 24 books", Timeframe: "year", TargetDate: strPtr("2026-12-31"),
	})
	if err != nil {
		t.Fatalf("Update failed: %v", err)
	}
	if updated.Title != "Read 24 books" {
		t.Fatalf("Update didn't apply: %+v", updated)
	}

	completed, err := store.SetCompleted(ctx, userID, created.ID, true)
	if err != nil {
		t.Fatalf("SetCompleted failed: %v", err)
	}
	if !completed.Completed {
		t.Fatalf("expected goal to be completed: %+v", completed)
	}

	if err := store.Delete(ctx, userID, created.ID); err != nil {
		t.Fatalf("Delete failed: %v", err)
	}
	if _, err := store.Get(ctx, userID, created.ID); !errors.Is(err, pgx.ErrNoRows) {
		t.Fatalf("expected ErrNoRows after delete, got %v", err)
	}
}

func TestInvalidTimeframeRejected(t *testing.T) {
	store, userID := newTestStore(t)
	ctx := context.Background()

	_, err := store.Create(ctx, userID, Input{Title: "Bad goal", Timeframe: "decade"})
	var invalid ErrInvalidInput
	if !errors.As(err, &invalid) {
		t.Fatalf("expected ErrInvalidInput for bad timeframe, got %v", err)
	}
}

func TestListOrderedByTimeframe(t *testing.T) {
	store, userID := newTestStore(t)
	ctx := context.Background()

	for _, tf := range []string{"lifetime", "week", "year", "month"} {
		if _, err := store.Create(ctx, userID, Input{Title: "Goal: " + tf, Timeframe: tf}); err != nil {
			t.Fatalf("Create(%s) failed: %v", tf, err)
		}
	}

	goals, err := store.List(ctx, userID)
	if err != nil {
		t.Fatalf("List failed: %v", err)
	}
	if len(goals) != 4 {
		t.Fatalf("expected 4 goals, got %d", len(goals))
	}
	want := []string{"week", "month", "year", "lifetime"}
	for i, g := range goals {
		if g.Timeframe != want[i] {
			t.Fatalf("expected timeframe order %v, got position %d = %s", want, i, g.Timeframe)
		}
	}
}

func TestScopedToUser(t *testing.T) {
	store, userA := newTestStore(t)
	_, userB := newTestStore(t)
	ctx := context.Background()

	created, err := store.Create(ctx, userA, Input{Title: "A's goal", Timeframe: "week"})
	if err != nil {
		t.Fatalf("Create failed: %v", err)
	}

	if _, err := store.Get(ctx, userB, created.ID); !errors.Is(err, pgx.ErrNoRows) {
		t.Fatalf("expected user B to not see user A's goal, got %v", err)
	}

	goalsB, err := store.List(ctx, userB)
	if err != nil {
		t.Fatalf("List failed: %v", err)
	}
	if len(goalsB) != 0 {
		t.Fatalf("expected user B to have no goals, got %+v", goalsB)
	}
}
