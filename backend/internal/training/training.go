// Package training stores strength work: reusable exercises, logged sets,
// and a small dashboard summary for progressive overload.
package training

import (
	"context"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"
)

type Exercise struct {
	ID        string    `json:"id"`
	Name      string    `json:"name"`
	Category  string    `json:"category"`
	Notes     *string   `json:"notes"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

type ExerciseInput struct {
	Name     string  `json:"name"`
	Category string  `json:"category"`
	Notes    *string `json:"notes"`
}

type Set struct {
	ID           string    `json:"id"`
	ExerciseID   string    `json:"exercise_id"`
	ExerciseName string    `json:"exercise_name"`
	PerformedOn  string    `json:"performed_on"`
	Weight       float64   `json:"weight"`
	Reps         int       `json:"reps"`
	RPE          *float64  `json:"rpe"`
	Notes        *string   `json:"notes"`
	Volume       float64   `json:"volume"`
	CreatedAt    time.Time `json:"created_at"`
}

type SetInput struct {
	ExerciseID  string   `json:"exercise_id"`
	PerformedOn string   `json:"performed_on"`
	Weight      float64  `json:"weight"`
	Reps        int      `json:"reps"`
	RPE         *float64 `json:"rpe"`
	Notes       *string  `json:"notes"`
}

type Summary struct {
	PerformedOn string  `json:"performed_on"`
	TodaySets   []Set   `json:"today_sets"`
	WeekSets    int     `json:"week_sets"`
	WeekVolume  float64 `json:"week_volume"`
	PRs         []Set   `json:"prs"`
	Streak      int     `json:"streak"`
}

type ErrInvalidInput struct{ Reason string }

func (e ErrInvalidInput) Error() string { return e.Reason }

type Store struct {
	db *pgxpool.Pool
}

func NewStore(db *pgxpool.Pool) *Store { return &Store{db: db} }

const exerciseColumns = "id, name, category, notes, created_at, updated_at"
const setColumns = "ws.id, ws.exercise_id, e.name, ws.performed_on, ws.weight, ws.reps, ws.rpe, ws.notes, ws.created_at"

func validateExerciseInput(in ExerciseInput) error {
	if in.Name == "" {
		return ErrInvalidInput{"name is required"}
	}
	return nil
}

func validateSetInput(in SetInput) error {
	if in.ExerciseID == "" {
		return ErrInvalidInput{"exercise_id is required"}
	}
	if _, err := time.Parse("2006-01-02", in.PerformedOn); err != nil {
		return ErrInvalidInput{"performed_on must be YYYY-MM-DD"}
	}
	if in.Weight < 0 {
		return ErrInvalidInput{"weight must not be negative"}
	}
	if in.Reps <= 0 {
		return ErrInvalidInput{"reps must be positive"}
	}
	if in.RPE != nil && (*in.RPE < 1 || *in.RPE > 10) {
		return ErrInvalidInput{"rpe must be between 1 and 10"}
	}
	return nil
}

func scanExercise(row pgx.Row) (Exercise, error) {
	var exercise Exercise
	err := row.Scan(&exercise.ID, &exercise.Name, &exercise.Category, &exercise.Notes, &exercise.CreatedAt, &exercise.UpdatedAt)
	return exercise, err
}

func scanSet(row pgx.Row) (Set, error) {
	var set Set
	var performedOn pgtype.Date
	err := row.Scan(&set.ID, &set.ExerciseID, &set.ExerciseName, &performedOn, &set.Weight, &set.Reps, &set.RPE, &set.Notes, &set.CreatedAt)
	set.PerformedOn = performedOn.Time.Format("2006-01-02")
	set.Volume = set.Weight * float64(set.Reps)
	return set, err
}

func (s *Store) CreateExercise(ctx context.Context, userID string, in ExerciseInput) (Exercise, error) {
	if err := validateExerciseInput(in); err != nil {
		return Exercise{}, err
	}
	if in.Category == "" {
		in.Category = "Strength"
	}

	row := s.db.QueryRow(ctx, `
		insert into exercises (user_id, name, category, notes)
		values ($1, $2, $3, $4)
		returning `+exerciseColumns,
		userID, in.Name, in.Category, in.Notes,
	)
	return scanExercise(row)
}

func (s *Store) ListExercises(ctx context.Context, userID string) ([]Exercise, error) {
	rows, err := s.db.Query(ctx, `
		select `+exerciseColumns+`
		from exercises
		where user_id = $1
		order by category, name`,
		userID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var exercises []Exercise
	for rows.Next() {
		exercise, err := scanExercise(rows)
		if err != nil {
			return nil, err
		}
		exercises = append(exercises, exercise)
	}
	return exercises, rows.Err()
}

func (s *Store) GetExercise(ctx context.Context, userID, id string) (Exercise, error) {
	row := s.db.QueryRow(ctx, `
		select `+exerciseColumns+`
		from exercises
		where user_id = $1 and id = $2`,
		userID, id,
	)
	return scanExercise(row)
}

func (s *Store) UpdateExercise(ctx context.Context, userID, id string, in ExerciseInput) (Exercise, error) {
	if err := validateExerciseInput(in); err != nil {
		return Exercise{}, err
	}
	if in.Category == "" {
		in.Category = "Strength"
	}

	row := s.db.QueryRow(ctx, `
		update exercises
		set name = $3, category = $4, notes = $5, updated_at = now()
		where user_id = $1 and id = $2
		returning `+exerciseColumns,
		userID, id, in.Name, in.Category, in.Notes,
	)
	return scanExercise(row)
}

func (s *Store) DeleteExercise(ctx context.Context, userID, id string) error {
	tag, err := s.db.Exec(ctx, `delete from exercises where user_id = $1 and id = $2`, userID, id)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return pgx.ErrNoRows
	}
	return nil
}

func (s *Store) CreateSet(ctx context.Context, userID string, in SetInput) (Set, error) {
	if err := validateSetInput(in); err != nil {
		return Set{}, err
	}
	if _, err := s.GetExercise(ctx, userID, in.ExerciseID); err != nil {
		return Set{}, ErrInvalidInput{"exercise not found"}
	}

	var id string
	err := s.db.QueryRow(ctx, `
		insert into workout_sets (user_id, exercise_id, performed_on, weight, reps, rpe, notes)
		values ($1, $2, $3, $4, $5, $6, $7)
		returning id`,
		userID, in.ExerciseID, in.PerformedOn, in.Weight, in.Reps, in.RPE, in.Notes,
	).Scan(&id)
	if err != nil {
		return Set{}, err
	}
	return s.GetSet(ctx, userID, id)
}

func (s *Store) GetSet(ctx context.Context, userID, id string) (Set, error) {
	row := s.db.QueryRow(ctx, `
		select `+setColumns+`
		from workout_sets ws
		join exercises e on e.id = ws.exercise_id
		where ws.user_id = $1 and ws.id = $2`,
		userID, id,
	)
	return scanSet(row)
}

func (s *Store) ListSets(ctx context.Context, userID, performedOn string) ([]Set, error) {
	rows, err := s.db.Query(ctx, `
		select `+setColumns+`
		from workout_sets ws
		join exercises e on e.id = ws.exercise_id
		where ws.user_id = $1 and ws.performed_on = $2
		order by ws.created_at`,
		userID, performedOn,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var sets []Set
	for rows.Next() {
		set, err := scanSet(rows)
		if err != nil {
			return nil, err
		}
		sets = append(sets, set)
	}
	return sets, rows.Err()
}

func (s *Store) DeleteSet(ctx context.Context, userID, id string) error {
	tag, err := s.db.Exec(ctx, `delete from workout_sets where user_id = $1 and id = $2`, userID, id)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return pgx.ErrNoRows
	}
	return nil
}

func (s *Store) Summary(ctx context.Context, userID, performedOn string) (Summary, error) {
	todaySets, err := s.ListSets(ctx, userID, performedOn)
	if err != nil {
		return Summary{}, err
	}
	if todaySets == nil {
		todaySets = []Set{}
	}

	var weekSets int
	var weekVolume float64
	err = s.db.QueryRow(ctx, `
		select count(*), coalesce(sum(weight * reps), 0)
		from workout_sets
		where user_id = $1
			and performed_on >= ($2::date - interval '6 days')
			and performed_on <= $2::date`,
		userID, performedOn,
	).Scan(&weekSets, &weekVolume)
	if err != nil {
		return Summary{}, err
	}

	prs, err := s.prsForDate(ctx, userID, performedOn)
	if err != nil {
		return Summary{}, err
	}
	if prs == nil {
		prs = []Set{}
	}

	return Summary{PerformedOn: performedOn, TodaySets: todaySets, WeekSets: weekSets, WeekVolume: weekVolume, PRs: prs}, nil
}

func (s *Store) prsForDate(ctx context.Context, userID, performedOn string) ([]Set, error) {
	rows, err := s.db.Query(ctx, `
		select `+setColumns+`
		from workout_sets ws
		join exercises e on e.id = ws.exercise_id
		where ws.user_id = $1
			and ws.performed_on = $2
			and ws.weight = (
				select max(prev.weight)
				from workout_sets prev
				where prev.user_id = ws.user_id and prev.exercise_id = ws.exercise_id
			)
			and not exists (
				select 1
				from workout_sets older
				where older.user_id = ws.user_id
					and older.exercise_id = ws.exercise_id
					and older.performed_on < ws.performed_on
					and older.weight >= ws.weight
			)
		order by ws.created_at`,
		userID, performedOn,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var sets []Set
	for rows.Next() {
		set, err := scanSet(rows)
		if err != nil {
			return nil, err
		}
		sets = append(sets, set)
	}
	return sets, rows.Err()
}

// LoggingStreak counts consecutive days with at least one set logged,
// walking back from today. If today has no set yet, it starts the count
// from yesterday instead - not having logged yet today shouldn't zero out
// an otherwise-intact streak.
func (s *Store) LoggingStreak(ctx context.Context, userID string, today time.Time) (int, error) {
	rows, err := s.db.Query(ctx, `select distinct performed_on from workout_sets where user_id = $1`, userID)
	if err != nil {
		return 0, err
	}
	logged := map[string]bool{}
	for rows.Next() {
		var d time.Time
		if err := rows.Scan(&d); err != nil {
			rows.Close()
			return 0, err
		}
		logged[d.Format("2006-01-02")] = true
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return 0, err
	}

	cursor := today
	if !logged[cursor.Format("2006-01-02")] {
		cursor = cursor.AddDate(0, 0, -1)
	}
	streak := 0
	for logged[cursor.Format("2006-01-02")] {
		streak++
		cursor = cursor.AddDate(0, 0, -1)
	}
	return streak, nil
}

// DayVolume is one day's total set volume, used by VolumeHistory for a
// trend chart.
type DayVolume struct {
	PerformedOn string  `json:"performed_on"`
	Volume      float64 `json:"volume"`
	Sets        int     `json:"sets"`
}

// VolumeHistory returns `days` DayVolumes ending today, ascending by date,
// zero-filling days with nothing logged. Loops calling ListSets per day
// rather than a single grouped SQL query, matching nutrition.History's
// reuse-over-new-SQL approach - fine at this data scale.
func (s *Store) VolumeHistory(ctx context.Context, userID string, days int, today time.Time) ([]DayVolume, error) {
	out := make([]DayVolume, days)
	for i := 0; i < days; i++ {
		d := today.AddDate(0, 0, -(days - 1 - i))
		dateStr := d.Format("2006-01-02")
		sets, err := s.ListSets(ctx, userID, dateStr)
		if err != nil {
			return nil, err
		}
		dv := DayVolume{PerformedOn: dateStr, Sets: len(sets)}
		for _, set := range sets {
			dv.Volume += set.Volume
		}
		out[i] = dv
	}
	return out, nil
}
