-- 'weekdays' was a redundant recurrence_type: it's just 'weekly' with
-- recurrence_days = {1,2,3,4,5}. The frontend offers it as a preset button
-- that fills in those days rather than the backend needing a separate case.
alter table public.tasks drop constraint recurrence_type_check;
alter table public.tasks add constraint recurrence_type_check
  check (recurrence_type in ('none', 'daily', 'weekly'));
