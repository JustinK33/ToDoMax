create table public.exercises (
  id uuid primary key default gen_random_uuid(),
  user_id uuid not null references auth.users(id) on delete cascade,
  name text not null,
  category text not null default 'Strength',
  notes text,
  created_at timestamptz not null default now(),
  updated_at timestamptz not null default now()
);

create index exercises_user_id_idx on public.exercises (user_id);

create table public.workout_sets (
  id uuid primary key default gen_random_uuid(),
  user_id uuid not null references auth.users(id) on delete cascade,
  exercise_id uuid not null references public.exercises(id) on delete cascade,
  performed_on date not null,
  weight numeric not null default 0 check (weight >= 0),
  reps int not null check (reps > 0),
  rpe numeric check (rpe is null or (rpe >= 1 and rpe <= 10)),
  notes text,
  created_at timestamptz not null default now()
);

create index workout_sets_user_date_idx on public.workout_sets (user_id, performed_on);
create index workout_sets_exercise_id_idx on public.workout_sets (exercise_id);

alter table public.exercises enable row level security;
alter table public.workout_sets enable row level security;

create policy "exercises_owner_all" on public.exercises
  for all using (auth.uid() = user_id) with check (auth.uid() = user_id);

create policy "workout_sets_owner_all" on public.workout_sets
  for all using (auth.uid() = user_id) with check (auth.uid() = user_id);
