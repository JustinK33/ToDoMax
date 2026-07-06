package training

import (
	"context"

	"github.com/jackc/pgx/v5"
)

// TemplateItem is one exercise slot within a workout template, expanded on
// read. Order matters (e.g. squats before accessory work), unlike a meal's
// food items, so it's carried via the row's position in Items.
type TemplateItem struct {
	ExerciseID   string `json:"exercise_id"`
	ExerciseName string `json:"exercise_name"`
}

// TemplateItemInput is what a client sends to compose a template.
type TemplateItemInput struct {
	ExerciseID string `json:"exercise_id"`
}

// Template is a reusable, user-defined workout - a named, ordered list of
// exercises the user builds themselves rather than a fixed Push/Pull/Legs
// split.
type Template struct {
	ID    string         `json:"id"`
	Name  string         `json:"name"`
	Items []TemplateItem `json:"items"`
}

// TemplateInput is the subset of Template fields a client may set on
// create/update.
type TemplateInput struct {
	Name  string              `json:"name"`
	Items []TemplateItemInput `json:"items"`
}

func validateTemplateInput(in TemplateInput) error {
	if in.Name == "" {
		return ErrInvalidInput{"name is required"}
	}
	for _, item := range in.Items {
		if item.ExerciseID == "" {
			return ErrInvalidInput{"each template item needs an exercise"}
		}
	}
	return nil
}

// ownedExerciseIDs returns which of the given exercise IDs belong to userID -
// used to reject a template item referencing someone else's (or a
// nonexistent) exercise, since the workout_template_items table's foreign
// key only checks existence, not ownership.
func (s *Store) ownedExerciseIDs(ctx context.Context, userID string, exerciseIDs []string) (map[string]bool, error) {
	rows, err := s.db.Query(ctx, `select id from exercises where user_id = $1 and id = any($2::uuid[])`, userID, exerciseIDs)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	owned := map[string]bool{}
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		owned[id] = true
	}
	return owned, rows.Err()
}

func (s *Store) writeTemplateItems(ctx context.Context, tx pgx.Tx, userID, templateID string, items []TemplateItemInput) error {
	if len(items) == 0 {
		return nil
	}
	exerciseIDs := make([]string, len(items))
	for i, item := range items {
		exerciseIDs[i] = item.ExerciseID
	}
	owned, err := s.ownedExerciseIDs(ctx, userID, exerciseIDs)
	if err != nil {
		return err
	}
	for i, item := range items {
		if !owned[item.ExerciseID] {
			return ErrInvalidInput{"exercise not found"}
		}
		if _, err := tx.Exec(ctx, `insert into workout_template_items (template_id, exercise_id, position) values ($1, $2, $3)`,
			templateID, item.ExerciseID, i); err != nil {
			return err
		}
	}
	return nil
}

func (s *Store) CreateTemplate(ctx context.Context, userID string, in TemplateInput) (Template, error) {
	if err := validateTemplateInput(in); err != nil {
		return Template{}, err
	}

	tx, err := s.db.Begin(ctx)
	if err != nil {
		return Template{}, err
	}
	defer tx.Rollback(ctx)

	var templateID string
	if err := tx.QueryRow(ctx, `insert into workout_templates (user_id, name) values ($1, $2) returning id`, userID, in.Name).Scan(&templateID); err != nil {
		return Template{}, err
	}
	if err := s.writeTemplateItems(ctx, tx, userID, templateID, in.Items); err != nil {
		return Template{}, err
	}
	if err := tx.Commit(ctx); err != nil {
		return Template{}, err
	}
	return s.GetTemplate(ctx, userID, templateID)
}

func (s *Store) UpdateTemplate(ctx context.Context, userID, id string, in TemplateInput) (Template, error) {
	if err := validateTemplateInput(in); err != nil {
		return Template{}, err
	}

	tx, err := s.db.Begin(ctx)
	if err != nil {
		return Template{}, err
	}
	defer tx.Rollback(ctx)

	tag, err := tx.Exec(ctx, `update workout_templates set name = $3, updated_at = now() where user_id = $1 and id = $2`, userID, id, in.Name)
	if err != nil {
		return Template{}, err
	}
	if tag.RowsAffected() == 0 {
		return Template{}, pgx.ErrNoRows
	}
	if _, err := tx.Exec(ctx, `delete from workout_template_items where template_id = $1`, id); err != nil {
		return Template{}, err
	}
	if err := s.writeTemplateItems(ctx, tx, userID, id, in.Items); err != nil {
		return Template{}, err
	}
	if err := tx.Commit(ctx); err != nil {
		return Template{}, err
	}
	return s.GetTemplate(ctx, userID, id)
}

func (s *Store) ListTemplates(ctx context.Context, userID string) ([]Template, error) {
	rows, err := s.db.Query(ctx, `select id, name from workout_templates where user_id = $1 order by name`, userID)
	if err != nil {
		return nil, err
	}
	type templateRow struct{ id, name string }
	var templateRows []templateRow
	for rows.Next() {
		var tr templateRow
		if err := rows.Scan(&tr.id, &tr.name); err != nil {
			rows.Close()
			return nil, err
		}
		templateRows = append(templateRows, tr)
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return nil, err
	}

	ids := make([]string, len(templateRows))
	for i, tr := range templateRows {
		ids[i] = tr.id
	}
	itemsByTemplate, err := s.templateItemsByTemplate(ctx, ids)
	if err != nil {
		return nil, err
	}

	templates := make([]Template, len(templateRows))
	for i, tr := range templateRows {
		templates[i] = Template{ID: tr.id, Name: tr.name, Items: itemsByTemplate[tr.id]}
	}
	return templates, nil
}

func (s *Store) GetTemplate(ctx context.Context, userID, id string) (Template, error) {
	var name string
	if err := s.db.QueryRow(ctx, `select name from workout_templates where user_id = $1 and id = $2`, userID, id).Scan(&name); err != nil {
		return Template{}, err
	}
	itemsByTemplate, err := s.templateItemsByTemplate(ctx, []string{id})
	if err != nil {
		return Template{}, err
	}
	return Template{ID: id, Name: name, Items: itemsByTemplate[id]}, nil
}

func (s *Store) DeleteTemplate(ctx context.Context, userID, id string) error {
	tag, err := s.db.Exec(ctx, `delete from workout_templates where user_id = $1 and id = $2`, userID, id)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return pgx.ErrNoRows
	}
	return nil
}

func (s *Store) templateItemsByTemplate(ctx context.Context, templateIDs []string) (map[string][]TemplateItem, error) {
	if len(templateIDs) == 0 {
		return map[string][]TemplateItem{}, nil
	}
	rows, err := s.db.Query(ctx, `
		select wti.template_id, e.id, e.name
		from workout_template_items wti
		join exercises e on e.id = wti.exercise_id
		where wti.template_id = any($1::uuid[])
		order by wti.position`,
		templateIDs,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	byTemplate := map[string][]TemplateItem{}
	for rows.Next() {
		var templateID string
		var item TemplateItem
		if err := rows.Scan(&templateID, &item.ExerciseID, &item.ExerciseName); err != nil {
			return nil, err
		}
		byTemplate[templateID] = append(byTemplate[templateID], item)
	}
	return byTemplate, rows.Err()
}
