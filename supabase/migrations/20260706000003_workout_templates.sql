create table public.workout_templates (
  id uuid primary key default gen_random_uuid(),
  user_id uuid not null references auth.users(id) on delete cascade,
  name text not null,
  created_at timestamptz not null default now(),
  updated_at timestamptz not null default now()
);

create index workout_templates_user_id_idx on public.workout_templates (user_id);

-- Unlike meal_items, order matters here (e.g. squats before accessory
-- work), so this table keeps an explicit position column.
create table public.workout_template_items (
  id uuid primary key default gen_random_uuid(),
  template_id uuid not null references public.workout_templates(id) on delete cascade,
  exercise_id uuid not null references public.exercises(id) on delete cascade,
  position int not null default 0
);

create index workout_template_items_template_id_idx on public.workout_template_items (template_id);

alter table public.workout_templates enable row level security;
alter table public.workout_template_items enable row level security;

create policy "workout_templates_owner_all" on public.workout_templates
  for all using (auth.uid() = user_id) with check (auth.uid() = user_id);

create policy "workout_template_items_owner_all" on public.workout_template_items
  for all using (exists (select 1 from public.workout_templates t where t.id = template_id and t.user_id = auth.uid()))
  with check (exists (select 1 from public.workout_templates t where t.id = template_id and t.user_id = auth.uid()));
