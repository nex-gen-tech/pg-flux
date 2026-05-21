-- pg-flux generated migration
-- DO NOT EDIT unless you know what you are doing.
-- pg-flux-baseline-hash: 7646161420265b98623222a78ca4b8018b6359375f08a63b9927163d7a2ce8a9

-- [ADVISORY COLUMN_REORDER] Column order in public.todos differs from desired schema; reordering requires table recreation. Desired order (surviving cols): id, user_id, category_id, title, body, done, created_at, updated_at
-- [ADVISORY COLUMN_REORDER] Column order in public.users differs from desired schema; reordering requires table recreation. Desired order (surviving cols): id, email, handle, email_verified, created_at

BEGIN;

-- [1] ADD_COLUMN: public.todos.priority
ALTER TABLE public.todos ADD COLUMN IF NOT EXISTS priority public.todo_priority NOT NULL DEFAULT 'normal';

COMMIT;

-- The following statements use CONCURRENTLY and run outside the transaction.
-- [2] CREATE_INDEX: public.idx_todos_priority
CREATE INDEX CONCURRENTLY IF NOT EXISTS idx_todos_priority ON public.todos USING btree (priority) WHERE priority IN ('high', 'urgent');

