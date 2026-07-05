// Package reminder runs an in-process ticker that emails a reminder shortly
// before a task's due time. Push notifications can be added later as a
// second branch inside send() - no interface is built ahead of that need.
package reminder

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"time"

	"github.com/justin/todomax/backend/internal/task"
)

type Config struct {
	ResendAPIKey string
	FromEmail    string
	ToEmail      string
}

type Runner struct {
	tasks *task.Store
	cfg   Config
	loc   *time.Location
}

func New(tasks *task.Store, cfg Config, loc *time.Location) *Runner {
	return &Runner{tasks: tasks, cfg: cfg, loc: loc}
}

// Run ticks every minute until ctx is canceled. Call it in its own goroutine.
func (r *Runner) Run(ctx context.Context) {
	ticker := time.NewTicker(time.Minute)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			r.safeTick(ctx)
		}
	}
}

// safeTick isolates a single tick: a panic here (e.g. an unexpected driver
// error) would otherwise take down the whole process, since this runs in the
// same process as the HTTP server, not a separate worker. A per-tick timeout
// also keeps a hung DB query or a stalled Resend call from freezing every
// reminder for the rest of the process's life - ticker.C drops missed ticks
// while the receiver is busy, so an unbounded tick() never gets unstuck.
func (r *Runner) safeTick(ctx context.Context) {
	defer func() {
		if p := recover(); p != nil {
			log.Printf("reminder: tick panicked: %v", p)
		}
	}()
	tickCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	r.tick(tickCtx)
}

func (r *Runner) tick(ctx context.Context) {
	now := time.Now().In(r.loc)

	candidates, err := r.tasks.DueReminders(ctx, now)
	if err != nil {
		log.Printf("reminder: scanning due reminders failed: %v", err)
		return
	}

	for _, c := range candidates {
		claimed, err := r.tasks.ClaimReminder(ctx, c.TaskID, c.OccurrenceDate)
		if err != nil {
			log.Printf("reminder: claiming %s failed: %v", c.TaskID, err)
			continue
		}
		if !claimed {
			continue // another tick already sent this one
		}

		subject := fmt.Sprintf("Reminder: %s", c.Title)
		body := fmt.Sprintf("%s is due at %s.", c.Title, c.DueAt.Format("3:04 PM"))
		if err := r.send(ctx, subject, body); err != nil {
			log.Printf("reminder: sending email for %s failed: %v", c.TaskID, err)
		}
	}
}

func (r *Runner) send(ctx context.Context, subject, body string) error {
	if r.cfg.ResendAPIKey == "" {
		log.Printf("reminder: RESEND_API_KEY not set, skipping send (%s)", subject)
		return nil
	}

	payload, _ := json.Marshal(map[string]any{
		"from":    r.cfg.FromEmail,
		"to":      []string{r.cfg.ToEmail},
		"subject": subject,
		"text":    body,
	})

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, "https://api.resend.com/emails", bytes.NewReader(payload))
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "Bearer "+r.cfg.ResendAPIKey)
	req.Header.Set("Content-Type", "application/json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 300 {
		respBody, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("resend responded %d: %s", resp.StatusCode, respBody)
	}
	return nil
}
