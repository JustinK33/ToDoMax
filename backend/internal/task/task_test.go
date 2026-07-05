package task

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

// newTestStore connects to the database configured by DATABASE_URL (the
// local Supabase Postgres in dev, a service container in CI) and creates a
// throwaway auth.users row so the tasks.user_id foreign key is satisfiable,
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
		Title:    "Cut nails",
		Notes:    strPtr("weekly grooming"),
		Category: strPtr("self-care"),
		DueDate:  strPtr("2026-07-12"),
		DueTime:  strPtr("09:00:00"),
	})
	if err != nil {
		t.Fatalf("Create failed: %v", err)
	}
	if created.Title != "Cut nails" || created.Completed {
		t.Fatalf("unexpected created task: %+v", created)
	}
	if created.DueDate == nil || *created.DueDate != "2026-07-12" {
		t.Fatalf("expected due_date 2026-07-12, got %v", created.DueDate)
	}

	got, err := store.Get(ctx, userID, created.ID)
	if err != nil {
		t.Fatalf("Get failed: %v", err)
	}
	if got.ID != created.ID {
		t.Fatalf("Get returned wrong task")
	}

	list, err := store.List(ctx, userID, Filter{})
	if err != nil {
		t.Fatalf("List failed: %v", err)
	}
	if len(list) != 1 {
		t.Fatalf("expected 1 task, got %d", len(list))
	}

	updated, err := store.Update(ctx, userID, created.ID, Input{Title: "Cut nails and toes"})
	if err != nil {
		t.Fatalf("Update failed: %v", err)
	}
	if updated.Title != "Cut nails and toes" {
		t.Fatalf("expected updated title, got %q", updated.Title)
	}
	if updated.Notes != nil {
		t.Fatalf("expected notes cleared by full update, got %v", updated.Notes)
	}

	completed, err := store.SetCompleted(ctx, userID, created.ID, true)
	if err != nil {
		t.Fatalf("SetCompleted failed: %v", err)
	}
	if !completed.Completed {
		t.Fatalf("expected task marked completed")
	}

	if err := store.Delete(ctx, userID, created.ID); err != nil {
		t.Fatalf("Delete failed: %v", err)
	}
	if _, err := store.Get(ctx, userID, created.ID); err == nil {
		t.Fatalf("expected error getting deleted task")
	}
}

func TestScopedToUser(t *testing.T) {
	store, userA := newTestStore(t)
	_, userB := newTestStore(t)
	ctx := context.Background()

	t.Cleanup(func() {
		store.db.Exec(ctx, `delete from auth.users where id = $1`, userB)
	})

	created, err := store.Create(ctx, userA, Input{Title: "user A's task"})
	if err != nil {
		t.Fatalf("Create failed: %v", err)
	}

	if _, err := store.Get(ctx, userB, created.ID); err == nil {
		t.Fatalf("expected user B to not see user A's task")
	}
}

func TestListViews(t *testing.T) {
	store, userID := newTestStore(t)
	ctx := context.Background()

	now := time.Date(2026, 7, 8, 12, 0, 0, 0, time.UTC)
	for now.Weekday() != time.Wednesday {
		now = now.AddDate(0, 0, 1)
	}
	monday := mondayOf(now)

	mustCreate := func(title string, due time.Time) Task {
		d := due.Format("2006-01-02")
		tk, err := store.Create(ctx, userID, Input{Title: title, DueDate: &d})
		if err != nil {
			t.Fatalf("Create %s failed: %v", title, err)
		}
		return tk
	}

	todayTask := mustCreate("today task", now)
	weekOnlyTask := mustCreate("week only task", monday.AddDate(0, 0, 4)) // Friday of this week: same week, still in the future relative to "today" (Wednesday)
	overdueTask := mustCreate("overdue task", monday.AddDate(0, 0, -7))
	upcomingTask := mustCreate("upcoming task", monday.AddDate(0, 0, 13))

	titles := func(tasks []Task) []string {
		out := make([]string, len(tasks))
		for i, tk := range tasks {
			out[i] = tk.Title
		}
		return out
	}

	assertTitles := func(t *testing.T, view string, want []string) {
		t.Helper()
		got, err := store.List(ctx, userID, Filter{View: view, Now: now})
		if err != nil {
			t.Fatalf("List(view=%s) failed: %v", view, err)
		}
		gotTitles := titles(got)
		if len(gotTitles) != len(want) {
			t.Fatalf("view=%s: expected %v, got %v", view, want, gotTitles)
		}
		for i := range want {
			if gotTitles[i] != want[i] {
				t.Fatalf("view=%s: expected %v, got %v", view, want, gotTitles)
			}
		}
	}

	t.Run("today", func(t *testing.T) {
		assertTitles(t, "today", []string{todayTask.Title})
	})
	t.Run("week", func(t *testing.T) {
		assertTitles(t, "week", []string{todayTask.Title, weekOnlyTask.Title})
	})
	t.Run("overdue", func(t *testing.T) {
		assertTitles(t, "overdue", []string{overdueTask.Title})
	})
	t.Run("upcoming", func(t *testing.T) {
		assertTitles(t, "upcoming", []string{weekOnlyTask.Title, upcomingTask.Title})
	})

	t.Run("category filter", func(t *testing.T) {
		cat := "self-care"
		created, err := store.Create(ctx, userID, Input{Title: "categorized", Category: &cat})
		if err != nil {
			t.Fatalf("Create failed: %v", err)
		}
		got, err := store.List(ctx, userID, Filter{Category: cat, Now: now})
		if err != nil {
			t.Fatalf("List(category) failed: %v", err)
		}
		if len(got) != 1 || got[0].ID != created.ID {
			t.Fatalf("expected only the categorized task, got %v", titles(got))
		}
	})
}
