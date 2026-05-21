-- pg-flux generated migration
-- DO NOT EDIT unless you know what you are doing.
-- pg-flux-baseline-hash: 89bdf8be4f71ab679b74e80fdae97d63e4046f2e23fcc22c826bdc40d2288dd3

-- [ADVISORY COLUMN_REORDER] Column order in public.todos differs from desired schema; reordering requires table recreation. Desired order (surviving cols): id, user_id, category_id, title, body, priority, done, created_at, updated_at
-- [ADVISORY COLUMN_REORDER] Column order in public.users differs from desired schema; reordering requires table recreation. Desired order (surviving cols): id, email, handle, email_verified, created_at

BEGIN;

-- [HAZARD DATA_LOSS] Drops column data
-- [1] DROP_COLUMN: public.tags.label
ALTER TABLE public.tags DROP COLUMN IF EXISTS label CASCADE;

-- [2] ADD_COLUMN: public.tags.name
ALTER TABLE public.tags ADD COLUMN IF NOT EXISTS name text NOT NULL;

-- [HAZARD DATA_LOSS] Drops constraint
-- [3] DROP_TABLE_CONSTRAINT: public.tags/tags_label_unique
ALTER TABLE public.tags DROP CONSTRAINT IF EXISTS tags_label_unique;

-- [HAZARD CONSTRAINT_SCAN] Adding constraint may scan table
-- [4] ADD_TABLE_CONSTRAINT: public.tags/tags_name_unique
ALTER TABLE public.tags ADD CONSTRAINT tags_name_unique UNIQUE (name);

-- [HAZARD DATA_LOSS] Drops view
-- [5] DROP_VIEW: public.active_todos
DROP VIEW IF EXISTS public.active_todos CASCADE;

-- [6] CREATE_VIEW: public.active_todos
CREATE OR REPLACE VIEW public.active_todos AS SELECT t.id, t.user_id, t.title, t.priority, t.created_at FROM public.todos t WHERE t.done = false;

COMMIT;
