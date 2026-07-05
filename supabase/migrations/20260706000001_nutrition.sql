create table public.foods (
  id uuid primary key default gen_random_uuid(),
  user_id uuid not null references auth.users(id) on delete cascade,
  name text not null,
  serving_label text not null default 'serving',
  calories numeric not null default 0,
  protein_g numeric not null default 0,
  carbs_g numeric not null default 0,
  fat_g numeric not null default 0,
  created_at timestamptz not null default now(),
  updated_at timestamptz not null default now()
);

create index foods_user_id_idx on public.foods (user_id);

create table public.meals (
  id uuid primary key default gen_random_uuid(),
  user_id uuid not null references auth.users(id) on delete cascade,
  name text not null,
  created_at timestamptz not null default now(),
  updated_at timestamptz not null default now()
);

create index meals_user_id_idx on public.meals (user_id);

create table public.meal_items (
  id uuid primary key default gen_random_uuid(),
  meal_id uuid not null references public.meals(id) on delete cascade,
  food_id uuid not null references public.foods(id) on delete cascade,
  servings numeric not null default 1 check (servings > 0)
);

create index meal_items_meal_id_idx on public.meal_items (meal_id);

-- Logs either a food or a meal for a given day (never both, never neither).
-- Macros are computed live from the referenced food's/meal's current values
-- at read time, not frozen when logged - editing a food later changes past
-- days' totals too. Simpler MVP option; a snapshot table would be the
-- upgrade if that ever matters.
create table public.food_log_entries (
  id uuid primary key default gen_random_uuid(),
  user_id uuid not null references auth.users(id) on delete cascade,
  log_date date not null,
  food_id uuid references public.foods(id) on delete cascade,
  meal_id uuid references public.meals(id) on delete cascade,
  servings numeric not null default 1 check (servings > 0),
  created_at timestamptz not null default now(),
  constraint food_log_entries_source_check check (
    (food_id is not null and meal_id is null) or (food_id is null and meal_id is not null)
  )
);

create index food_log_entries_user_date_idx on public.food_log_entries (user_id, log_date);

-- A single current-value row per user, not a history table. All columns
-- nullable so a user can set just a protein target and leave the rest.
create table public.nutrition_targets (
  user_id uuid primary key references auth.users(id) on delete cascade,
  calories int,
  protein_g int,
  carbs_g int,
  fat_g int,
  updated_at timestamptz not null default now()
);

alter table public.foods enable row level security;
alter table public.meals enable row level security;
alter table public.meal_items enable row level security;
alter table public.food_log_entries enable row level security;
alter table public.nutrition_targets enable row level security;

create policy "foods_owner_all" on public.foods
  for all using (auth.uid() = user_id) with check (auth.uid() = user_id);

create policy "meals_owner_all" on public.meals
  for all using (auth.uid() = user_id) with check (auth.uid() = user_id);

create policy "meal_items_owner_all" on public.meal_items
  for all using (exists (select 1 from public.meals m where m.id = meal_id and m.user_id = auth.uid()))
  with check (exists (select 1 from public.meals m where m.id = meal_id and m.user_id = auth.uid()));

create policy "food_log_entries_owner_all" on public.food_log_entries
  for all using (auth.uid() = user_id) with check (auth.uid() = user_id);

create policy "nutrition_targets_owner_all" on public.nutrition_targets
  for all using (auth.uid() = user_id) with check (auth.uid() = user_id);
