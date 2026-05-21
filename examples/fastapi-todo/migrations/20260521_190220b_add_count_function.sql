-- pg-flux generated migration
-- DO NOT EDIT unless you know what you are doing.
-- pg-flux-baseline-hash: 607bf64bab376e338f95f2b3f0f1f9b15b37fa05a906569f06c139f09eddb430

-- [ADVISORY COLUMN_REORDER] Column order in public.todos differs from desired schema; reordering requires table recreation. Desired order (surviving cols): id, user_id, category_id, title, body, priority, done, created_at, updated_at
-- [ADVISORY COLUMN_REORDER] Column order in public.users differs from desired schema; reordering requires table recreation. Desired order (surviving cols): id, email, handle, email_verified, created_at

BEGIN;

-- [1] CREATE_FUNCTION: public.count_user_todos(bigint)
CREATE OR REPLACE FUNCTION public.count_user_todos(p_user_id bigint) RETURNS bigint LANGUAGE sql STABLE AS $$
  SELECT count(*) FROM public.todos WHERE user_id = p_user_id;
$$;

-- [HAZARD DATA_LOSS] Drops view
-- [2] DROP_VIEW: public.active_todos
DROP VIEW IF EXISTS public.active_todos CASCADE;

-- [3] CREATE_VIEW: public.active_todos
CREATE OR REPLACE VIEW public.active_todos AS SELECT t.id, t.user_id, t.title, t.priority, t.created_at FROM public.todos t WHERE t.done = false;

COMMIT;
