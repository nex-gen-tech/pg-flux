-- pg-flux generated migration
-- DO NOT EDIT unless you know what you are doing.
-- pg-flux-baseline-hash: b306ac9a5ef7f52f5d158e79bea8ab4009718c7c1bfe703a99024e20c29c0244

-- [ADVISORY COLUMN_REORDER] Column order in public.todos differs from desired schema; reordering requires table recreation. Desired order (surviving cols): id, user_id, category_id, title, body, priority, done, created_at, updated_at
-- [ADVISORY COLUMN_REORDER] Column order in public.users differs from desired schema; reordering requires table recreation. Desired order (surviving cols): id, email, handle, email_verified, created_at

BEGIN;

-- [1] TOGGLE_RLS: public.todos
ALTER TABLE public.todos ENABLE ROW LEVEL SECURITY;

-- [2] TOGGLE_RLS_NOFORCE: public.todos
ALTER TABLE public.todos NO FORCE ROW LEVEL SECURITY;

-- [3] CREATE_POLICY: public.todos/todos_owner_only
CREATE POLICY todos_owner_only ON public.todos TO public USING (user_id = current_setting('app.user_id', true)::bigint);

-- [HAZARD DATA_LOSS] Drops view
-- [4] DROP_VIEW: public.active_todos
DROP VIEW IF EXISTS public.active_todos CASCADE;

-- [5] CREATE_VIEW: public.active_todos
CREATE OR REPLACE VIEW public.active_todos AS SELECT t.id, t.user_id, t.title, t.priority, t.created_at FROM public.todos t WHERE t.done = false;

COMMIT;
