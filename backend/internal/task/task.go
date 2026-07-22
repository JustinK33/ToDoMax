// Package task is the core todo/habit store: one-off and recurring tasks,
// their per-day occurrences, and the completion state for each. Recurrence is
// kept small (daily, or specific weekdays) rather than a full calendar-style
// rule engine.
package task

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"
)

// Task is the API-facing representation of a task template. DueDate/DueTime
// use plain strings ("2026-07-05" / "14:30:00") rather than time.Time so a
// task with no time component doesn't need a fake one, and so JSON stays
// exactly what the frontend sent. For a recurring task, DueDate is the
// anchor date recurrence starts from, not "the" due date.
type Task struct {
	ID                    string    `json:"id"`
	Title                 string    `json:"title"`
	Notes                 *string   `json:"notes"`
	Category              *string   `json:"category"`
	DueDate               *string   `json:"due_date"`
	DueTime               *string   `json:"due_time"`
	RecurrenceType        string    `json:"recurrence_type"`
	RecurrenceDays        []int     `json:"recurrence_days,omitempty"`
	ReminderMinutesBefore *int      `json:"reminder_minutes_before"`
	Completed             bool      `json:"completed"`
	CreatedAt             time.Time `json:"created_at"`
	UpdatedAt             time.Time `json:"updated_at"`
}

// Occurrence is a single day's instance of a task, as shown in the
// today/week/upcoming views. For a non-recurring task there's exactly one
// occurrence (its due date); for a recurring task, one per matching day.
type Occurrence struct {
	TaskID                string  `json:"id"`
	Title                 string  `json:"title"`
	Notes                 *string `json:"notes"`
	Category              *string `json:"category"`
	DueDate               string  `json:"due_date"`
	DueTime               *string `json:"due_time"`
	RecurrenceType        string  `json:"recurrence_type"`
	RecurrenceDays        []int   `json:"recurrence_days,omitempty"`
	ReminderMinutesBefore *int    `json:"reminder_minutes_before"`
	Completed             bool    `json:"completed"`
}

// Input is the subset of Task fields a client may set on create/update.
type Input struct {
	Title                 string  `json:"title"`
	Notes                 *string `json:"notes"`
	Category              *string `json:"category"`
	DueDate               *string `json:"due_date"`
	DueTime               *string `json:"due_time"`
	RecurrenceType        string  `json:"recurrence_type"`
	RecurrenceDays        []int   `json:"recurrence_days"`
	ReminderMinutesBefore *int    `json:"reminder_minutes_before"`
}

// ErrInvalidInput is returned when input fields don't parse or don't satisfy
// a validation rule (e.g. weekly recurrence with no days given).
type ErrInvalidInput struct{ Reason string }

func (e ErrInvalidInput) Error() string { return e.Reason }

func parseDate(s *string) (pgtype.Date, error) {
	if s == nil {
		return pgtype.Date{}, nil
	}
	t, err := time.Parse("2006-01-02", *s)
	if err != nil {
		return pgtype.Date{}, ErrInvalidInput{"due_date must be YYYY-MM-DD"}
	}
	return pgtype.Date{Time: t, Valid: true}, nil
}

func parseTime(s *string) (pgtype.Time, error) {
	if s == nil {
		return pgtype.Time{}, nil
	}
	t, err := time.Parse("15:04:05", *s)
	if err != nil {
		t, err = time.Parse("15:04", *s)
	}
	if err != nil {
		return pgtype.Time{}, ErrInvalidInput{"due_time must be HH:MM or HH:MM:SS"}
	}
	micros := int64(t.Hour())*3600e6 + int64(t.Minute())*60e6 + int64(t.Second())*1e6
	return pgtype.Time{Microseconds: micros, Valid: true}, nil
}

// parseRecurrence validates recurrence_type/recurrence_days and converts the
// days to the smallint width Postgres stores them as.
func parseRecurrence(in Input) (string, []int16, error) {
	rt := in.RecurrenceType
	if rt == "" {
		rt = "none"
	}
	switch rt {
	case "none", "daily":
		return rt, nil, nil
	case "weekly":
		if len(in.RecurrenceDays) == 0 {
			return "", nil, ErrInvalidInput{"weekly recurrence requires recurrence_days"}
		}
		days := make([]int16, len(in.RecurrenceDays))
		for i, d := range in.RecurrenceDays {
			if d < 1 || d > 7 {
				return "", nil, ErrInvalidInput{"recurrence_days must be 1-7 (ISO weekday, 1=Monday)"}
			}
			days[i] = int16(d)
		}
		return rt, days, nil
	default:
		return "", nil, ErrInvalidInput{"recurrence_type must be none, daily, or weekly"}
	}
}

// validateReminder rejects a negative reminder_minutes_before - the
// reminder-scanning math (dueAt minus the offset) would otherwise place the
// reminder window after the due time, so the reminder could never fire and
// the bad value would just sit there silently.
func validateReminder(in Input) error {
	if in.ReminderMinutesBefore != nil && *in.ReminderMinutesBefore < 0 {
		return ErrInvalidInput{"reminder_minutes_before must not be negative"}
	}
	return nil
}

func formatDate(d pgtype.Date) *string {
	if !d.Valid {
		return nil
	}
	s := d.Time.Format("2006-01-02")
	return &s
}

func formatTime(t pgtype.Time) *string {
	if !t.Valid {
		return nil
	}
	d := time.Duration(t.Microseconds) * time.Microsecond
	s := fmt.Sprintf("%02d:%02d:%02d", int(d.Hours())%24, int(d.Minutes())%60, int(d.Seconds())%60)
	return &s
}

func int16sToInts(in []int16) []int {
	if in == nil {
		return nil
	}
	out := make([]int, len(in))
	for i, v := range in {
		out[i] = int(v)
	}
	return out
}

type Store struct {
	db *pgxpool.Pool
}

func NewStore(db *pgxpool.Pool) *Store {
	return &Store{db: db}
}

const taskColumns = "id, title, notes, category, due_date, due_time, recurrence_type, recurrence_days, reminder_minutes_before, completed, created_at, updated_at"

func scanTask(row pgx.Row) (Task, error) {
	var t Task
	var dueDate pgtype.Date
	var dueTime pgtype.Time
	var recurrenceDays []int16
	err := row.Scan(&t.ID, &t.Title, &t.Notes, &t.Category, &dueDate, &dueTime,
		&t.RecurrenceType, &recurrenceDays, &t.ReminderMinutesBefore, &t.Completed, &t.CreatedAt, &t.UpdatedAt)
	t.DueDate = formatDate(dueDate)
	t.DueTime = formatTime(dueTime)
	t.RecurrenceDays = int16sToInts(recurrenceDays)
	return t, err
}

func (s *Store) Create(ctx context.Context, userID string, in Input) (Task, error) {
	dueDate, err := parseDate(in.DueDate)
	if err != nil {
		return Task{}, err
	}
	dueTime, err := parseTime(in.DueTime)
	if err != nil {
		return Task{}, err
	}
	recurrenceType, recurrenceDays, err := parseRecurrence(in)
	if err != nil {
		return Task{}, err
	}
	if err := validateReminder(in); err != nil {
		return Task{}, err
	}

	row := s.db.QueryRow(ctx, `
		insert into tasks (user_id, title, notes, category, due_date, due_time, recurrence_type, recurrence_days, reminder_minutes_before)
		values ($1, $2, $3, $4, $5, $6, $7, $8, $9)
		returning `+taskColumns,
		userID, in.Title, in.Notes, in.Category, dueDate, dueTime, recurrenceType, recurrenceDays, in.ReminderMinutesBefore,
	)
	return scanTask(row)
}

// Filter selects which tasks List/ListOccurrences return. View is one of
// "today"/"week"/"overdue"/"upcoming"/"all" (default "all" for zero value).
// Now must be the caller's current time in the user's configured timezone,
// so "today"/"week" boundaries land on the right calendar day.
type Filter struct {
	View     string
	Category string
	Now      time.Time
}

func mondayOf(t time.Time) time.Time {
	offset := (int(t.Weekday()) + 6) % 7 // days since Monday (Sunday=0 -> 6)
	return t.AddDate(0, 0, -offset)
}

// List returns task templates - one row per task regardless of recurrence.
// Used for the "all" management view and the overdue view (recurring tasks
// never count as "overdue"; a missed recurring day just isn't shown as done).
func (s *Store) List(ctx context.Context, userID string, f Filter) ([]Task, error) {
	conditions := []string{"user_id = $1"}
	args := []interface{}{userID}
	arg := func(v interface{}) string {
		args = append(args, v)
		return fmt.Sprintf("$%d", len(args))
	}

	today := f.Now.Format("2006-01-02")
	switch f.View {
	case "today":
		conditions = append(conditions, "due_date = "+arg(today))
	case "week":
		monday := mondayOf(f.Now).Format("2006-01-02")
		sunday := mondayOf(f.Now).AddDate(0, 0, 6).Format("2006-01-02")
		conditions = append(conditions, "due_date between "+arg(monday)+" and "+arg(sunday))
	case "overdue":
		conditions = append(conditions, "due_date < "+arg(today)+" and completed = false and recurrence_type = 'none'")
	case "upcoming":
		conditions = append(conditions, "due_date > "+arg(today))
	}

	if f.Category != "" {
		conditions = append(conditions, "category = "+arg(f.Category))
	}

	query := `select ` + taskColumns + ` from tasks where ` + strings.Join(conditions, " and ") +
		` order by due_date nulls last, due_time nulls last, created_at`

	rows, err := s.db.Query(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var tasks []Task
	for rows.Next() {
		t, err := scanTask(rows)
		if err != nil {
			return nil, err
		}
		tasks = append(tasks, t)
	}
	return tasks, rows.Err()
}

// occursOn reports whether a task's recurrence rule produces an occurrence
// on date d (d and anchor are both midnight-normalized calendar dates).
func occursOn(t Task, anchor *time.Time, d time.Time) bool {
	if anchor != nil && d.Before(*anchor) {
		return false
	}
	switch t.RecurrenceType {
	case "daily":
		return true
	case "weekly":
		iso := int(d.Weekday())
		if iso == 0 {
			iso = 7 // Go's Sunday=0 -> ISO 7
		}
		for _, day := range t.RecurrenceDays {
			if day == iso {
				return true
			}
		}
	}
	return false
}

// ListOccurrences expands recurring tasks into per-day instances within the
// view's date window, alongside non-recurring tasks whose due date falls in
// that window. View must be "today", "week", or "upcoming" ("upcoming" looks
// 30 days ahead - recurrence has no natural end date, so it needs a bound).
func (s *Store) ListOccurrences(ctx context.Context, userID string, f Filter) ([]Occurrence, error) {
	var start, end time.Time
	switch f.View {
	case "today":
		start, end = f.Now, f.Now
	case "week":
		start = mondayOf(f.Now)
		end = start.AddDate(0, 0, 6)
	case "upcoming":
		start = f.Now.AddDate(0, 0, 1)
		end = f.Now.AddDate(0, 0, 30)
	default:
		start, end = f.Now, f.Now
	}
	start = time.Date(start.Year(), start.Month(), start.Day(), 0, 0, 0, 0, time.UTC)
	end = time.Date(end.Year(), end.Month(), end.Day(), 0, 0, 0, 0, time.UTC)

	conditions := []string{"user_id = $1", "(recurrence_type != 'none' or (due_date between $2 and $3))"}
	args := []interface{}{userID, start.Format("2006-01-02"), end.Format("2006-01-02")}
	if f.Category != "" {
		args = append(args, f.Category)
		conditions = append(conditions, fmt.Sprintf("category = $%d", len(args)))
	}

	rows, err := s.db.Query(ctx, `select `+taskColumns+` from tasks where `+strings.Join(conditions, " and "), args...)
	if err != nil {
		return nil, err
	}
	var tasks []Task
	for rows.Next() {
		t, err := scanTask(rows)
		if err != nil {
			rows.Close()
			return nil, err
		}
		tasks = append(tasks, t)
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return nil, err
	}

	recurringIDs := make([]string, 0, len(tasks))
	for _, t := range tasks {
		if t.RecurrenceType != "none" {
			recurringIDs = append(recurringIDs, t.ID)
		}
	}

	completions := map[string]bool{} // "taskID|date" -> completed
	if len(recurringIDs) > 0 {
		crows, err := s.db.Query(ctx, `
			select task_id, occurrence_date from task_completions
			where task_id = any($1::uuid[]) and occurrence_date between $2 and $3`,
			recurringIDs, start.Format("2006-01-02"), end.Format("2006-01-02"),
		)
		if err != nil {
			return nil, err
		}
		for crows.Next() {
			var taskID string
			var occDate time.Time
			if err := crows.Scan(&taskID, &occDate); err != nil {
				crows.Close()
				return nil, err
			}
			completions[taskID+"|"+occDate.Format("2006-01-02")] = true
		}
		crows.Close()
		if err := crows.Err(); err != nil {
			return nil, err
		}
	}

	var out []Occurrence
	for _, t := range tasks {
		if t.RecurrenceType == "none" {
			if t.DueDate == nil {
				continue
			}
			out = append(out, Occurrence{
				TaskID: t.ID, Title: t.Title, Notes: t.Notes, Category: t.Category,
				DueDate: *t.DueDate, DueTime: t.DueTime, RecurrenceType: t.RecurrenceType,
				ReminderMinutesBefore: t.ReminderMinutesBefore, Completed: t.Completed,
			})
			continue
		}

		var anchor *time.Time
		if t.DueDate != nil {
			a, err := time.Parse("2006-01-02", *t.DueDate)
			if err == nil {
				anchor = &a
			}
		}

		for d := start; !d.After(end); d = d.AddDate(0, 0, 1) {
			if !occursOn(t, anchor, d) {
				continue
			}
			dateStr := d.Format("2006-01-02")
			out = append(out, Occurrence{
				TaskID: t.ID, Title: t.Title, Notes: t.Notes, Category: t.Category,
				DueDate: dateStr, DueTime: t.DueTime, RecurrenceType: t.RecurrenceType,
				RecurrenceDays: t.RecurrenceDays, ReminderMinutesBefore: t.ReminderMinutesBefore,
				Completed: completions[t.ID+"|"+dateStr],
			})
		}
	}

	sort.Slice(out, func(i, j int) bool {
		if out[i].DueDate != out[j].DueDate {
			return out[i].DueDate < out[j].DueDate
		}
		return out[i].Title < out[j].Title
	})
	return out, nil
}

func (s *Store) Get(ctx context.Context, userID, id string) (Task, error) {
	row := s.db.QueryRow(ctx, `
		select `+taskColumns+`
		from tasks
		where user_id = $1 and id = $2`,
		userID, id,
	)
	return scanTask(row)
}

func (s *Store) Update(ctx context.Context, userID, id string, in Input) (Task, error) {
	dueDate, err := parseDate(in.DueDate)
	if err != nil {
		return Task{}, err
	}
	dueTime, err := parseTime(in.DueTime)
	if err != nil {
		return Task{}, err
	}
	recurrenceType, recurrenceDays, err := parseRecurrence(in)
	if err != nil {
		return Task{}, err
	}
	if err := validateReminder(in); err != nil {
		return Task{}, err
	}

	row := s.db.QueryRow(ctx, `
		update tasks
		set title = $3, notes = $4, category = $5, due_date = $6, due_time = $7,
		    recurrence_type = $8, recurrence_days = $9, reminder_minutes_before = $10, updated_at = now()
		where user_id = $1 and id = $2
		returning `+taskColumns,
		userID, id, in.Title, in.Notes, in.Category, dueDate, dueTime, recurrenceType, recurrenceDays, in.ReminderMinutesBefore,
	)
	return scanTask(row)
}

func (s *Store) Delete(ctx context.Context, userID, id string) error {
	tag, err := s.db.Exec(ctx, `delete from tasks where user_id = $1 and id = $2`, userID, id)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return pgx.ErrNoRows
	}
	return nil
}

func (s *Store) SetCompleted(ctx context.Context, userID, id string, completed bool) (Task, error) {
	row := s.db.QueryRow(ctx, `
		update tasks
		set completed = $3, updated_at = now()
		where user_id = $1 and id = $2
		returning `+taskColumns,
		userID, id, completed,
	)
	return scanTask(row)
}

// SetOccurrenceCompleted marks a single day complete/incomplete. For a
// non-recurring task this is equivalent to SetCompleted (occurrenceDate is
// ignored). For a recurring task it adds/removes a task_completions row for
// that specific day, leaving other days' completion history untouched.
func (s *Store) SetOccurrenceCompleted(ctx context.Context, userID, id, occurrenceDate string, completed bool) (Occurrence, error) {
	t, err := s.Get(ctx, userID, id)
	if err != nil {
		return Occurrence{}, err
	}

	if t.RecurrenceType == "none" {
		updated, err := s.SetCompleted(ctx, userID, id, completed)
		if err != nil {
			return Occurrence{}, err
		}
		date := occurrenceDate
		if updated.DueDate != nil {
			date = *updated.DueDate
		}
		return Occurrence{
			TaskID: updated.ID, Title: updated.Title, Notes: updated.Notes, Category: updated.Category,
			DueDate: date, DueTime: updated.DueTime, RecurrenceType: updated.RecurrenceType,
			ReminderMinutesBefore: updated.ReminderMinutesBefore, Completed: updated.Completed,
		}, nil
	}

	if _, err := time.Parse("2006-01-02", occurrenceDate); err != nil {
		return Occurrence{}, ErrInvalidInput{"occurrence_date must be YYYY-MM-DD"}
	}

	if completed {
		_, err = s.db.Exec(ctx, `
			insert into task_completions (task_id, occurrence_date) values ($1, $2)
			on conflict (task_id, occurrence_date) do nothing`,
			id, occurrenceDate,
		)
	} else {
		_, err = s.db.Exec(ctx, `delete from task_completions where task_id = $1 and occurrence_date = $2`, id, occurrenceDate)
	}
	if err != nil {
		return Occurrence{}, err
	}

	return Occurrence{
		TaskID: t.ID, Title: t.Title, Notes: t.Notes, Category: t.Category,
		DueDate: occurrenceDate, DueTime: t.DueTime, RecurrenceType: t.RecurrenceType,
		RecurrenceDays: t.RecurrenceDays, ReminderMinutesBefore: t.ReminderMinutesBefore, Completed: completed,
	}, nil
}

// ReminderCandidate is a task whose reminder window has opened - now is
// between (due time - reminder_minutes_before) and (due time), for today's
// occurrence, and no reminder_log row exists for it yet.
type ReminderCandidate struct {
	TaskID         string
	Title          string
	OccurrenceDate string
	DueAt          time.Time
}

// DueReminders scans every task across all users (this runs as a background
// job on a direct DB connection, not a per-user API request) for ones whose
// reminder window has opened right now. Only tasks with both due_time and
// reminder_minutes_before set are eligible - "remind me N minutes before" is
// meaningless without a specific time of day.
func (s *Store) DueReminders(ctx context.Context, now time.Time) ([]ReminderCandidate, error) {
	rows, err := s.db.Query(ctx, `
		select `+taskColumns+` from tasks
		where reminder_minutes_before is not null and due_time is not null`,
	)
	if err != nil {
		return nil, err
	}
	var tasks []Task
	for rows.Next() {
		t, err := scanTask(rows)
		if err != nil {
			rows.Close()
			return nil, err
		}
		tasks = append(tasks, t)
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return nil, err
	}

	today := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, time.UTC)
	todayStr := today.Format("2006-01-02")

	// Recurring tasks track per-day completion in task_completions rather
	// than the tasks.completed column, so today's completions need a
	// separate lookup - without it, a task checked off before its reminder
	// window closes would still get emailed as if it were still due.
	var recurringIDs []string
	for _, t := range tasks {
		if t.RecurrenceType != "none" {
			recurringIDs = append(recurringIDs, t.ID)
		}
	}
	completedToday := map[string]bool{}
	if len(recurringIDs) > 0 {
		crows, err := s.db.Query(ctx, `
			select task_id from task_completions
			where task_id = any($1::uuid[]) and occurrence_date = $2`,
			recurringIDs, todayStr,
		)
		if err != nil {
			return nil, err
		}
		for crows.Next() {
			var taskID string
			if err := crows.Scan(&taskID); err != nil {
				crows.Close()
				return nil, err
			}
			completedToday[taskID] = true
		}
		crows.Close()
		if err := crows.Err(); err != nil {
			return nil, err
		}
	}

	var out []ReminderCandidate
	for _, t := range tasks {
		occDate := todayStr
		if t.RecurrenceType == "none" {
			if t.DueDate == nil || t.Completed {
				continue
			}
			occDate = *t.DueDate
		} else {
			var anchor *time.Time
			if t.DueDate != nil {
				a, err := time.Parse("2006-01-02", *t.DueDate)
				if err == nil {
					anchor = &a
				}
			}
			if !occursOn(t, anchor, today) || completedToday[t.ID] {
				continue
			}
		}

		dueAt, err := time.ParseInLocation("2006-01-02 15:04:05", occDate+" "+*t.DueTime, now.Location())
		if err != nil {
			continue
		}
		reminderAt := dueAt.Add(-time.Duration(*t.ReminderMinutesBefore) * time.Minute)
		if now.Before(reminderAt) || !now.Before(dueAt) {
			continue
		}

		out = append(out, ReminderCandidate{TaskID: t.ID, Title: t.Title, OccurrenceDate: occDate, DueAt: dueAt})
	}
	return out, nil
}

// ClaimReminder atomically marks a reminder as sent, returning true only if
// this call was the one that claimed it (false means another tick already
// sent it - the unique constraint on reminder_log is what makes this safe).
func (s *Store) ClaimReminder(ctx context.Context, taskID, occurrenceDate string) (bool, error) {
	tag, err := s.db.Exec(ctx, `
		insert into reminder_log (task_id, occurrence_date) values ($1, $2)
		on conflict (task_id, occurrence_date) do nothing`,
		taskID, occurrenceDate,
	)
	if err != nil {
		return false, err
	}
	return tag.RowsAffected() > 0, nil
}

// CategoryProgress is one category's completion count for a WeekSummary.
type CategoryProgress struct {
	Category  string `json:"category"`
	Expected  int    `json:"expected"`
	Completed int    `json:"completed"`
}

// WeekSummary is how many of this week's expected occurrences (recurring
// tasks expanded per matching day, plus non-recurring tasks due this week)
// have been completed, overall and broken down by category.
type WeekSummary struct {
	Expected   int                `json:"expected"`
	Completed  int                `json:"completed"`
	ByCategory []CategoryProgress `json:"by_category"`
}

func (s *Store) WeekSummary(ctx context.Context, userID string, now time.Time) (WeekSummary, error) {
	occurrences, err := s.ListOccurrences(ctx, userID, Filter{View: "week", Now: now})
	if err != nil {
		return WeekSummary{}, err
	}

	byCategory := map[string]*CategoryProgress{}
	order := []string{}
	summary := WeekSummary{ByCategory: []CategoryProgress{}}

	for _, occ := range occurrences {
		summary.Expected++
		if occ.Completed {
			summary.Completed++
		}

		cat := "uncategorized"
		if occ.Category != nil && *occ.Category != "" {
			cat = *occ.Category
		}
		cp, ok := byCategory[cat]
		if !ok {
			cp = &CategoryProgress{Category: cat}
			byCategory[cat] = cp
			order = append(order, cat)
		}
		cp.Expected++
		if occ.Completed {
			cp.Completed++
		}
	}

	sort.Strings(order)
	for _, cat := range order {
		summary.ByCategory = append(summary.ByCategory, *byCategory[cat])
	}
	return summary, nil
}

// Habit is a recurring task shown on the Wellness page's Habits tab, with
// today's completion state and current streak attached. There's no separate
// habits table - a "habit" is just a task with recurrence_type != "none".
type Habit struct {
	TaskID         string  `json:"id"`
	Title          string  `json:"title"`
	Category       *string `json:"category"`
	RecurrenceType string  `json:"recurrence_type"`
	RecurrenceDays []int   `json:"recurrence_days,omitempty"`
	CompletedToday bool    `json:"completed_today"`
	Streak         int     `json:"streak"`
}

// streakFor counts consecutive scheduled days completed, walking backward
// from today. Non-scheduled days are skipped without breaking the streak.
// Today is special-cased: if scheduled but not yet completed, it's treated
// as neutral (the day isn't over yet) rather than breaking yesterday's
// streak.
func streakFor(t Task, anchor *time.Time, completed map[string]bool, today time.Time) int {
	streak := 0
	d := today
	for i := 0; i < 400; i++ { // bounds the walk-back; far more than any realistic streak
		if anchor != nil && d.Before(*anchor) {
			break
		}
		if occursOn(t, anchor, d) {
			dateStr := d.Format("2006-01-02")
			if completed[dateStr] {
				streak++
			} else if !d.Equal(today) {
				break
			}
		}
		d = d.AddDate(0, 0, -1)
	}
	return streak
}

// Habits returns every recurring task with today's completion state and
// current streak, for the Wellness page. Completions for all recurring
// tasks are fetched in one query (not one per task) to avoid N+1s.
func (s *Store) Habits(ctx context.Context, userID string, now time.Time) ([]Habit, error) {
	tasks, err := s.List(ctx, userID, Filter{View: "all", Now: now})
	if err != nil {
		return nil, err
	}

	var recurring []Task
	recurringIDs := make([]string, 0, len(tasks))
	for _, t := range tasks {
		if t.RecurrenceType != "none" {
			recurring = append(recurring, t)
			recurringIDs = append(recurringIDs, t.ID)
		}
	}

	completions := map[string]map[string]bool{} // taskID -> date -> completed
	if len(recurringIDs) > 0 {
		since := now.AddDate(0, 0, -400).Format("2006-01-02")
		rows, err := s.db.Query(ctx, `
			select task_id, occurrence_date from task_completions
			where task_id = any($1::uuid[]) and occurrence_date >= $2`,
			recurringIDs, since,
		)
		if err != nil {
			return nil, err
		}
		for rows.Next() {
			var taskID string
			var occDate time.Time
			if err := rows.Scan(&taskID, &occDate); err != nil {
				rows.Close()
				return nil, err
			}
			if completions[taskID] == nil {
				completions[taskID] = map[string]bool{}
			}
			completions[taskID][occDate.Format("2006-01-02")] = true
		}
		rows.Close()
		if err := rows.Err(); err != nil {
			return nil, err
		}
	}

	today := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, time.UTC)
	todayStr := today.Format("2006-01-02")

	habits := make([]Habit, len(recurring))
	for i, t := range recurring {
		var anchor *time.Time
		if t.DueDate != nil {
			if a, err := time.Parse("2006-01-02", *t.DueDate); err == nil {
				anchor = &a
			}
		}
		done := completions[t.ID]
		habits[i] = Habit{
			TaskID:         t.ID,
			Title:          t.Title,
			Category:       t.Category,
			RecurrenceType: t.RecurrenceType,
			RecurrenceDays: t.RecurrenceDays,
			CompletedToday: done[todayStr],
			Streak:         streakFor(t, anchor, done, today),
		}
	}
	return habits, nil
}
