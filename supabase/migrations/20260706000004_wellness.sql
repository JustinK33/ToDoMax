-- Unitless weight (like workout_sets.weight) and no per-day uniqueness
-- (like workout_sets, not nutrition_targets) - multiple logs/day allowed,
-- "today's value" is just the most recent by created_at.
create table public.body_metrics (
  id uuid primary key default gen_random_uuid(),
  user_id uuid not null references auth.users(id) on delete cascade,
  log_date date not null,
  weight numeric not null check (weight > 0),
  notes text,
  created_at timestamptz not null default now(),
  updated_at timestamptz not null default now()
);

create index body_metrics_user_date_idx on public.body_metrics (user_id, log_date);

create table public.sleep_logs (
  id uuid primary key default gen_random_uuid(),
  user_id uuid not null references auth.users(id) on delete cascade,
  log_date date not null,
  hours numeric not null check (hours >= 0 and hours <= 24),
  quality smallint check (quality between 1 and 5),
  notes text,
  created_at timestamptz not null default now(),
  updated_at timestamptz not null default now()
);

create index sleep_logs_user_date_idx on public.sleep_logs (user_id, log_date);

create table public.mood_logs (
  id uuid primary key default gen_random_uuid(),
  user_id uuid not null references auth.users(id) on delete cascade,
  log_date date not null,
  mood smallint not null check (mood between 1 and 5),
  notes text,
  created_at timestamptz not null default now(),
  updated_at timestamptz not null default now()
);

create index mood_logs_user_date_idx on public.mood_logs (user_id, log_date);

alter table public.body_metrics enable row level security;
alter table public.sleep_logs enable row level security;
alter table public.mood_logs enable row level security;

create policy "body_metrics_owner_all" on public.body_metrics
  for all using (auth.uid() = user_id) with check (auth.uid() = user_id);

create policy "sleep_logs_owner_all" on public.sleep_logs
  for all using (auth.uid() = user_id) with check (auth.uid() = user_id);

create policy "mood_logs_owner_all" on public.mood_logs
  for all using (auth.uid() = user_id) with check (auth.uid() = user_id);
