package task

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"
)

// Task is the API-facing representation of a task. DueDate/DueTime use plain
// strings ("2026-07-05" / "14:30:00") rather than time.Time so a task with no
// time component doesn't need a fake one, and so JSON stays exactly what the
// frontend sent.
type Task struct {
	ID        string    `json:"id"`
	Title     string    `json:"title"`
	Notes     *string   `json:"notes"`
	Category  *string   `json:"category"`
	DueDate   *string   `json:"due_date"`
	DueTime   *string   `json:"due_time"`
	Completed bool      `json:"completed"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

// Input is the subset of Task fields a client may set on create/update.
type Input struct {
	Title    string  `json:"title"`
	Notes    *string `json:"notes"`
	Category *string `json:"category"`
	DueDate  *string `json:"due_date"`
	DueTime  *string `json:"due_time"`
}

// ErrInvalidInput is returned when DueDate/DueTime don't parse.
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

type Store struct {
	db *pgxpool.Pool
}

func NewStore(db *pgxpool.Pool) *Store {
	return &Store{db: db}
}

const taskColumns = "id, title, notes, category, due_date, due_time, completed, created_at, updated_at"

func scanTask(row pgx.Row) (Task, error) {
	var t Task
	var dueDate pgtype.Date
	var dueTime pgtype.Time
	err := row.Scan(&t.ID, &t.Title, &t.Notes, &t.Category, &dueDate, &dueTime, &t.Completed, &t.CreatedAt, &t.UpdatedAt)
	t.DueDate = formatDate(dueDate)
	t.DueTime = formatTime(dueTime)
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

	row := s.db.QueryRow(ctx, `
		insert into tasks (user_id, title, notes, category, due_date, due_time)
		values ($1, $2, $3, $4, $5, $6)
		returning `+taskColumns,
		userID, in.Title, in.Notes, in.Category, dueDate, dueTime,
	)
	return scanTask(row)
}

// Filter selects which tasks List returns. View is one of
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
		conditions = append(conditions, "due_date < "+arg(today)+" and completed = false")
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

	row := s.db.QueryRow(ctx, `
		update tasks
		set title = $3, notes = $4, category = $5, due_date = $6, due_time = $7, updated_at = now()
		where user_id = $1 and id = $2
		returning `+taskColumns,
		userID, id, in.Title, in.Notes, in.Category, dueDate, dueTime,
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
