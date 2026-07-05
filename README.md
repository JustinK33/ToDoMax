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

### Docker Compose (alternative to the manual steps above)

Runs the backend and frontend as containers instead of `go run` / a manual
static server. The database/auth stack is still `supabase start` - it
already manages its own containers, so compose only covers the two pieces
that otherwise need manual commands:

```bash
supabase start
docker compose up --build
```

Then open `http://localhost:8090`. The frontend container serves
`frontend/` with `js/config.docker.js` mounted over `js/config.js`, pointed
at `localhost:8080` (backend) and `localhost:54321` (local Supabase) - the
real `frontend/js/config.js` (pointed at production) is untouched. The
backend container reaches the `supabase start` stack via
`host.docker.internal`.

### Reminders

The backend ticks every minute and emails a reminder shortly before a task's
due time (only for tasks with both a due time and a reminder set). It calls
[Resend](https://resend.com)'s REST API directly - no SDK. Set
`RESEND_API_KEY`, `REMINDER_FROM_EMAIL`, `REMINDER_TO_EMAIL` in `.env` to
enable sending; without a key it just logs what it would have sent, so local
dev works without a Resend account.

## Testing

```bash
cd backend && go vet ./... && go test ./...
```

Runs in CI on every push via `.github/workflows/ci.yml` against a Postgres
service container.

## Deployment

### Frontend (Vercel)

1. Import the GitHub repo into Vercel.
2. In the project's Settings, set **Root Directory** to `frontend/`. Leave
   the build command empty - it's static files, nothing to build.
3. Update `frontend/js/config.js` with the real (non-local) Supabase project
   URL, anon key, and the deployed backend's URL before pushing, since
   there's no build step to inject environment variables at deploy time.
4. Add the deployed Vercel domain to the backend's `FRONTEND_ORIGINS` env var
   so CORS allows it.

### Backend (Fly.io)

`backend/Dockerfile` builds a static Go binary on `distroless`; `fly.toml`
sets `min_machines_running = 1` and `auto_stop_machines = false` - unlike a
typical stateless API, this one needs to keep running continuously so the
reminder ticker goroutine doesn't stop between requests.

```bash
cd backend
fly launch --no-deploy   # creates/links the Fly app from fly.toml
fly secrets set \
  DATABASE_URL=... \
  SUPABASE_URL=... \
  FRONTEND_ORIGINS=https://your-app.vercel.app \
  RESEND_API_KEY=... \
  REMINDER_FROM_EMAIL=... \
  REMINDER_TO_EMAIL=... \
  TZ=America/Los_Angeles
fly deploy
```

Verified locally by building the image and running it against the local
Supabase stack's Docker network before trusting it in CI/production.

### Database (Supabase)

Point the Supabase CLI at the real project and push the migrations already
in `supabase/migrations/`:

```bash
supabase link --project-ref <your-project-ref>
supabase db push
```
