-- pg-flux generated migration
-- DO NOT EDIT unless you know what you are doing.
-- pg-flux-baseline-hash: 4f877e2fa9f24e08a85f6c1eee9b977d2051530595a51199c89b33a461bee57b

-- [ADVISORY COLUMN_REORDER] Column order in public.todos differs from desired schema; reordering requires table recreation. Desired order (surviving cols): id, user_id, category_id, title, body, priority, done, created_at, updated_at
-- [ADVISORY COLUMN_REORDER] Column order in public.users differs from desired schema; reordering requires table recreation. Desired order (surviving cols): id, email, handle, email_verified, created_at

BEGIN;

-- [1] ADD_COLUMN: public.todos.deleted_at
ALTER TABLE public.todos ADD COLUMN IF NOT EXISTS deleted_at timestamptz;

-- [HAZARD DATA_LOSS] Drops view
-- [2] DROP_VIEW: public.active_todos
DROP VIEW IF EXISTS public.active_todos CASCADE;

-- [4] CREATE_VIEW: public.active_todos
CREATE OR REPLACE VIEW public.active_todos AS SELECT t.id, t.user_id, t.title, t.priority, t.created_at FROM public.todos t WHERE t.done = false;

COMMIT;

-- The following statements use CONCURRENTLY and run outside the transaction.
-- [3] CREATE_INDEX: public.idx_todos_active
CREATE INDEX CONCURRENTLY IF NOT EXISTS idx_todos_active ON public.todos USING btree (user_id, created_at DESC) WHERE deleted_at IS NULL;

