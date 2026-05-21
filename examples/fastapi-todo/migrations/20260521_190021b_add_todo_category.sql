-- pg-flux generated migration
-- DO NOT EDIT unless you know what you are doing.
-- pg-flux-baseline-hash: 91395d9055f453f1340645a4e9f0206366ed9dba9ea229381f82f200d2c1cfa8

BEGIN;

-- [1] ADD_COLUMN: public.todos.category_id
ALTER TABLE public.todos ADD COLUMN IF NOT EXISTS category_id pg_catalog.int2;

-- [HAZARD CONSTRAINT_SCAN] Adding constraint may scan table
-- [2] ADD_TABLE_CONSTRAINT: public.todos/todos_category_id_fkey
ALTER TABLE public.todos ADD CONSTRAINT todos_category_id_fkey FOREIGN KEY (category_id) REFERENCES public.categories (id) ON DELETE SET NULL NOT VALID;

COMMIT;

-- The following statements use CONCURRENTLY and run outside the transaction.
-- [3] VALIDATE_TABLE_CONSTRAINT: public.todos/todos_category_id_fkey
ALTER TABLE public.todos VALIDATE CONSTRAINT todos_category_id_fkey;

-- [4] CREATE_INDEX: public.idx_todos_category
CREATE INDEX CONCURRENTLY IF NOT EXISTS idx_todos_category ON public.todos USING btree (category_id) WHERE category_id IS NOT NULL;

