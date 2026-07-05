package task

import (
	"context"
	"errors"
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

func TestRecurringOccurrences(t *testing.T) {
	store, userID := newTestStore(t)
	ctx := context.Background()

	now := time.Date(2026, 7, 8, 12, 0, 0, 0, time.UTC)
	for now.Weekday() != time.Wednesday {
		now = now.AddDate(0, 0, 1)
	}
	monday := mondayOf(now)
	todayStr := now.Format("2006-01-02")

	daily, err := store.Create(ctx, userID, Input{Title: "daily habit", RecurrenceType: "daily"})
	if err != nil {
		t.Fatalf("Create daily failed: %v", err)
	}

	// Monday(1) and Saturday(6): matches this week's Monday and Saturday only.
	weekly, err := store.Create(ctx, userID, Input{Title: "gym", RecurrenceType: "weekly", RecurrenceDays: []int{1, 6}})
	if err != nil {
		t.Fatalf("Create weekly failed: %v", err)
	}

	t.Run("weekly recurrence requires days", func(t *testing.T) {
		_, err := store.Create(ctx, userID, Input{Title: "bad", RecurrenceType: "weekly"})
		var invalid ErrInvalidInput
		if !errors.As(err, &invalid) {
			t.Fatalf("expected ErrInvalidInput, got %v", err)
		}
	})

	t.Run("daily appears today and rest of week", func(t *testing.T) {
		today, err := store.ListOccurrences(ctx, userID, Filter{View: "today", Now: now})
		if err != nil {
			t.Fatalf("ListOccurrences(today) failed: %v", err)
		}
		if len(today) != 1 || today[0].TaskID != daily.ID || today[0].DueDate != todayStr {
			t.Fatalf("expected daily habit today, got %+v", today)
		}
	})

	t.Run("weekly only matches its configured days", func(t *testing.T) {
		week, err := store.ListOccurrences(ctx, userID, Filter{View: "week", Now: now})
		if err != nil {
			t.Fatalf("ListOccurrences(week) failed: %v", err)
		}
		gymDates := map[string]bool{}
		for _, occ := range week {
			if occ.TaskID == weekly.ID {
				gymDates[occ.DueDate] = true
			}
		}
		wantMonday := monday.Format("2006-01-02")
		wantSaturday := monday.AddDate(0, 0, 5).Format("2006-01-02")
		if len(gymDates) != 2 || !gymDates[wantMonday] || !gymDates[wantSaturday] {
			t.Fatalf("expected gym on %s and %s, got %v", wantMonday, wantSaturday, gymDates)
		}
	})

	t.Run("recurring tasks are never overdue", func(t *testing.T) {
		overdue, err := store.List(ctx, userID, Filter{View: "overdue", Now: now})
		if err != nil {
			t.Fatalf("List(overdue) failed: %v", err)
		}
		for _, tk := range overdue {
			if tk.ID == daily.ID || tk.ID == weekly.ID {
				t.Fatalf("recurring task %s should never appear in overdue", tk.Title)
			}
		}
	})

	t.Run("completing one occurrence does not affect other days", func(t *testing.T) {
		if _, err := store.SetOccurrenceCompleted(ctx, userID, daily.ID, todayStr, true); err != nil {
			t.Fatalf("SetOccurrenceCompleted failed: %v", err)
		}

		today, err := store.ListOccurrences(ctx, userID, Filter{View: "today", Now: now})
		if err != nil {
			t.Fatalf("ListOccurrences(today) failed: %v", err)
		}
		if len(today) != 1 || !today[0].Completed {
			t.Fatalf("expected today's daily occurrence completed, got %+v", today)
		}

		tomorrow := now.AddDate(0, 0, 1)
		nextDay, err := store.ListOccurrences(ctx, userID, Filter{View: "today", Now: tomorrow})
		if err != nil {
			t.Fatalf("ListOccurrences(tomorrow) failed: %v", err)
		}
		if len(nextDay) != 1 || nextDay[0].Completed {
			t.Fatalf("expected tomorrow's daily occurrence NOT completed, got %+v", nextDay)
		}

		if _, err := store.SetOccurrenceCompleted(ctx, userID, daily.ID, todayStr, false); err != nil {
			t.Fatalf("un-complete failed: %v", err)
		}
		today, err = store.ListOccurrences(ctx, userID, Filter{View: "today", Now: now})
		if err != nil {
			t.Fatalf("ListOccurrences(today) failed: %v", err)
		}
		if len(today) != 1 || today[0].Completed {
			t.Fatalf("expected today's occurrence un-completed again, got %+v", today)
		}
	})
}

func minutesPtr(m int) *int { return &m }

// TestOccurrenceCarriesReminderField guards against Occurrence (returned by
// the today/week/upcoming views the frontend edits from) silently dropping
// fields that only exist on Task - this exact bug shipped once already:
// reminder_minutes_before was on Task but missing from Occurrence, so the
// edit modal always showed "no reminder" for anything opened from those views.
func TestOccurrenceCarriesReminderField(t *testing.T) {
	store, userID := newTestStore(t)
	ctx := context.Background()
	now := time.Date(2026, 7, 8, 12, 0, 0, 0, time.UTC)
	today := now.Format("2006-01-02")

	nonRecurring, err := store.Create(ctx, userID, Input{
		Title: "standup", DueDate: &today, DueTime: strPtr("09:00"), ReminderMinutesBefore: minutesPtr(20),
	})
	if err != nil {
		t.Fatalf("Create failed: %v", err)
	}

	occurrences, err := store.ListOccurrences(ctx, userID, Filter{View: "today", Now: now})
	if err != nil {
		t.Fatalf("ListOccurrences failed: %v", err)
	}
	if len(occurrences) != 1 || occurrences[0].ReminderMinutesBefore == nil || *occurrences[0].ReminderMinutesBefore != 20 {
		t.Fatalf("expected occurrence to carry reminder_minutes_before=20, got %+v", occurrences)
	}

	occ, err := store.SetOccurrenceCompleted(ctx, userID, nonRecurring.ID, today, true)
	if err != nil {
		t.Fatalf("SetOccurrenceCompleted failed: %v", err)
	}
	if occ.ReminderMinutesBefore == nil || *occ.ReminderMinutesBefore != 20 {
		t.Fatalf("expected SetOccurrenceCompleted result to carry reminder_minutes_before=20, got %+v", occ)
	}
}

func TestDueReminders(t *testing.T) {
	store, userID := newTestStore(t)
	ctx := context.Background()

	now := time.Date(2026, 7, 8, 14, 0, 0, 0, time.UTC) // 2:00pm
	today := now.Format("2006-01-02")

	inWindow, err := store.Create(ctx, userID, Input{
		Title: "in window", DueDate: &today, DueTime: strPtr("14:10:00"), ReminderMinutesBefore: minutesPtr(15),
	})
	if err != nil {
		t.Fatalf("Create in-window failed: %v", err)
	}
	tooEarly, err := store.Create(ctx, userID, Input{
		Title: "too early", DueDate: &today, DueTime: strPtr("15:00:00"), ReminderMinutesBefore: minutesPtr(5),
	})
	if err != nil {
		t.Fatalf("Create too-early failed: %v", err)
	}
	alreadyPassed, err := store.Create(ctx, userID, Input{
		Title: "already passed", DueDate: &today, DueTime: strPtr("13:00:00"), ReminderMinutesBefore: minutesPtr(10),
	})
	if err != nil {
		t.Fatalf("Create already-passed failed: %v", err)
	}
	noReminder, err := store.Create(ctx, userID, Input{
		Title: "no reminder set", DueDate: &today, DueTime: strPtr("14:05:00"),
	})
	if err != nil {
		t.Fatalf("Create no-reminder failed: %v", err)
	}

	// Weekly recurring, but not scheduled for whatever weekday `now` falls on -
	// must not fire even though the time-of-day matches.
	wrongDay := int(now.Weekday()) + 2
	if wrongDay > 7 {
		wrongDay -= 7
	}
	recurringOtherDay, err := store.Create(ctx, userID, Input{
		Title: "recurring other day", RecurrenceType: "weekly", RecurrenceDays: []int{wrongDay},
		DueTime: strPtr("14:05:00"), ReminderMinutesBefore: minutesPtr(30),
	})
	if err != nil {
		t.Fatalf("Create recurring-other-day failed: %v", err)
	}

	candidates, err := store.DueReminders(ctx, now)
	if err != nil {
		t.Fatalf("DueReminders failed: %v", err)
	}

	byID := map[string]bool{}
	for _, c := range candidates {
		byID[c.TaskID] = true
	}
	if !byID[inWindow.ID] {
		t.Fatalf("expected in-window task to be a candidate, got %+v", candidates)
	}
	for _, notExpected := range []Task{tooEarly, alreadyPassed, noReminder, recurringOtherDay} {
		if byID[notExpected.ID] {
			t.Fatalf("did not expect %q to be a reminder candidate", notExpected.Title)
		}
	}

	t.Run("claim is exactly-once", func(t *testing.T) {
		claimed, err := store.ClaimReminder(ctx, inWindow.ID, today)
		if err != nil {
			t.Fatalf("ClaimReminder failed: %v", err)
		}
		if !claimed {
			t.Fatalf("expected first claim to succeed")
		}
		claimedAgain, err := store.ClaimReminder(ctx, inWindow.ID, today)
		if err != nil {
			t.Fatalf("ClaimReminder (again) failed: %v", err)
		}
		if claimedAgain {
			t.Fatalf("expected second claim for the same task+date to fail")
		}
	})
}

func TestWeekSummary(t *testing.T) {
	store, userID := newTestStore(t)
	ctx := context.Background()

	now := time.Date(2026, 7, 8, 12, 0, 0, 0, time.UTC)
	for now.Weekday() != time.Wednesday {
		now = now.AddDate(0, 0, 1)
	}
	todayStr := now.Format("2006-01-02")

	healthCat := "health"
	workCat := "work"

	// Daily habit in "health": expected every day this week (7), complete today only.
	daily, err := store.Create(ctx, userID, Input{Title: "drink water", RecurrenceType: "daily", Category: &healthCat})
	if err != nil {
		t.Fatalf("Create daily failed: %v", err)
	}
	if _, err := store.SetOccurrenceCompleted(ctx, userID, daily.ID, todayStr, true); err != nil {
		t.Fatalf("SetOccurrenceCompleted failed: %v", err)
	}

	// One-off task due today in "work", left incomplete.
	if _, err := store.Create(ctx, userID, Input{Title: "send report", DueDate: &todayStr, Category: &workCat}); err != nil {
		t.Fatalf("Create one-off failed: %v", err)
	}

	summary, err := store.WeekSummary(ctx, userID, now)
	if err != nil {
		t.Fatalf("WeekSummary failed: %v", err)
	}

	if summary.Expected != 8 { // 7 daily occurrences + 1 one-off
		t.Fatalf("expected 8 total occurrences this week, got %d (%+v)", summary.Expected, summary)
	}
	if summary.Completed != 1 {
		t.Fatalf("expected 1 completed occurrence, got %d (%+v)", summary.Completed, summary)
	}

	var health, work *CategoryProgress
	for i := range summary.ByCategory {
		switch summary.ByCategory[i].Category {
		case "health":
			health = &summary.ByCategory[i]
		case "work":
			work = &summary.ByCategory[i]
		}
	}
	if health == nil || health.Expected != 7 || health.Completed != 1 {
		t.Fatalf("expected health: 7 expected/1 completed, got %+v", health)
	}
	if work == nil || work.Expected != 1 || work.Completed != 0 {
		t.Fatalf("expected work: 1 expected/0 completed, got %+v", work)
	}
}

// TestWeekSummaryEmptyByCategoryIsNotNil guards against a real bug: a nil Go
// slice marshals to JSON `null`, and the frontend calls .map() on
// by_category unconditionally - `null.map()` throws, which aborted the
// summary render silently (the DOM update line never ran) before this was
// fixed to always initialize ByCategory to an empty slice.
func TestWeekSummaryEmptyByCategoryIsNotNil(t *testing.T) {
	store, userID := newTestStore(t)
	ctx := context.Background()
	now := time.Date(2026, 7, 8, 12, 0, 0, 0, time.UTC)

	summary, err := store.WeekSummary(ctx, userID, now)
	if err != nil {
		t.Fatalf("WeekSummary failed: %v", err)
	}
	if summary.ByCategory == nil {
		t.Fatalf("expected ByCategory to be an empty slice, not nil")
	}
	if len(summary.ByCategory) != 0 {
		t.Fatalf("expected no categories for a user with no tasks, got %+v", summary.ByCategory)
	}
}
