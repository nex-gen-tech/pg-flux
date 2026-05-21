-- pg-flux generated migration
-- DO NOT EDIT unless you know what you are doing.
-- pg-flux-baseline-hash: ad803dc96439c83560e655a6e0e7753e6f40d8ef5c65c5a3d823e73b0aa57c8d

-- [ADVISORY COLUMN_REORDER] Column order in public.todos differs from desired schema; reordering requires table recreation. Desired order (surviving cols): id, user_id, category_id, title, body, done, created_at, updated_at
-- [ADVISORY COLUMN_REORDER] Column order in public.users differs from desired schema; reordering requires table recreation. Desired order (surviving cols): id, email, handle, email_verified, created_at

BEGIN;

-- [HAZARD CONSTRAINT_SCAN] Adding constraint may scan table
-- [1] ADD_TABLE_CONSTRAINT: public.todos/todos_body_length
ALTER TABLE public.todos ADD CONSTRAINT todos_body_length CHECK (length(body) <= 4000) NOT VALID;

COMMIT;

-- The following statements use CONCURRENTLY and run outside the transaction.
-- [2] VALIDATE_TABLE_CONSTRAINT: public.todos/todos_body_length
ALTER TABLE public.todos VALIDATE CONSTRAINT todos_body_length;

