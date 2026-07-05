create table public.tasks (
  id uuid primary key default gen_random_uuid(),
  user_id uuid not null references auth.users(id) on delete cascade,
  title text not null,
  notes text,
  category text,
  due_date date,
  due_time time,
  recurrence_type text not null default 'none',
  recurrence_days smallint[],
  reminder_minutes_before int,
  completed boolean not null default false,
  created_at timestamptz not null default now(),
  updated_at timestamptz not null default now(),
  constraint recurrence_type_check check (recurrence_type in ('none', 'daily', 'weekly', 'weekdays'))
);

create index tasks_user_id_idx on public.tasks (user_id);

create table public.task_completions (
  id uuid primary key default gen_random_uuid(),
  task_id uuid not null references public.tasks(id) on delete cascade,
  occurrence_date date not null,
  completed_at timestamptz not null default now(),
  unique (task_id, occurrence_date)
);

create table public.reminder_log (
  id uuid primary key default gen_random_uuid(),
  task_id uuid not null references public.tasks(id) on delete cascade,
  occurrence_date date not null,
  sent_at timestamptz not null default now(),
  unique (task_id, occurrence_date)
);

alter table public.tasks enable row level security;
alter table public.task_completions enable row level security;
alter table public.reminder_log enable row level security;

create policy "tasks_owner_all" on public.tasks
  for all using (auth.uid() = user_id) with check (auth.uid() = user_id);

create policy "task_completions_owner_all" on public.task_completions
  for all using (
    exists (select 1 from public.tasks t where t.id = task_id and t.user_id = auth.uid())
  ) with check (
    exists (select 1 from public.tasks t where t.id = task_id and t.user_id = auth.uid())
  );

create policy "reminder_log_owner_all" on public.reminder_log
  for all using (
    exists (select 1 from public.tasks t where t.id = task_id and t.user_id = auth.uid())
  ) with check (
    exists (select 1 from public.tasks t where t.id = task_id and t.user_id = auth.uid())
  );
