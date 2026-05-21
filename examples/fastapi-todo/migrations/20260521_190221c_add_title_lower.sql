-- pg-flux generated migration
-- DO NOT EDIT unless you know what you are doing.
-- pg-flux-baseline-hash: 85ceba83ac6462c7da42443225ca3822b89949d4b3adb7ee108373c6a784b5db

-- [ADVISORY COLUMN_REORDER] Column order in public.todos differs from desired schema; reordering requires table recreation. Desired order (surviving cols): id, user_id, category_id, title, body, priority, done, deleted_at, created_at, updated_at
-- [ADVISORY COLUMN_REORDER] Column order in public.users differs from desired schema; reordering requires table recreation. Desired order (surviving cols): id, email, handle, email_verified, created_at

BEGIN;

-- [1] ADD_COLUMN: public.todos.title_lower
ALTER TABLE public.todos ADD COLUMN IF NOT EXISTS title_lower text GENERATED ALWAYS AS (lower(title)) STORED;

-- [HAZARD DATA_LOSS] Drops view
-- [2] DROP_VIEW: public.active_todos
DROP VIEW IF EXISTS public.active_todos CASCADE;

-- [4] CREATE_VIEW: public.active_todos
CREATE OR REPLACE VIEW public.active_todos AS SELECT t.id, t.user_id, t.title, t.priority, t.created_at FROM public.todos t WHERE t.done = false;

COMMIT;

-- The following statements use CONCURRENTLY and run outside the transaction.
-- [3] CREATE_INDEX: public.idx_todos_title_lower
CREATE INDEX CONCURRENTLY IF NOT EXISTS idx_todos_title_lower ON public.todos USING btree (title_lower);

