# ToDoMax

A small, dark-themed personal todo/habit tracker.
Go backend, Supabase Postgres + Auth, static frontend for Vercel.

## Stack

- Backend: Go (stdlib `net/http`, `pgx`, `golang-jwt`), one binary, one process.
- Database/Auth: Supabase Postgres + Supabase Auth.
- Frontend: plain HTML/CSS/JS, no build step, Supabase JS client via ESM CDN.
- Reminders: an in-process ticker + Resend email.

## Local development

Requires: Go 1.26+, Docker, the Supabase CLI (`supabase`).

```bash
supabase start                 # local Postgres + Auth stack
cp .env.example .env            # fill in values printed by `supabase start`
cd backend && go run ./cmd/server
```

Serve `frontend/` with any static file server (e.g. `npx serve frontend`) and
open it in a browser.

## Testing

```bash
cd backend && go vet ./... && go test ./...
```

Runs in CI on every push via `.github/workflows/ci.yml` against a Postgres
service container.

## Deployment (milestone 10)

- Frontend: deployed on Vercel with the project's **Root Directory** setting
  pointed at `frontend/` (no build command needed - it's static files).
- Backend: containerized (`backend/Dockerfile`) and deployed to Fly.io, which
  keeps the reminder ticker goroutine alive as a long-running process.
