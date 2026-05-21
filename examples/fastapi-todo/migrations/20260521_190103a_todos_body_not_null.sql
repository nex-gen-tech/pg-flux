-- pg-flux generated migration
-- DO NOT EDIT unless you know what you are doing.
-- pg-flux-baseline-hash: 4a4315156688201b4b0a6195660a28ffded0c133677ca806e20794c1877ca645

-- [ADVISORY COLUMN_REORDER] Column order in public.todos differs from desired schema; reordering requires table recreation. Desired order (surviving cols): id, user_id, category_id, title, body, done, created_at, updated_at
-- [ADVISORY COLUMN_REORDER] Column order in public.users differs from desired schema; reordering requires table recreation. Desired order (surviving cols): id, email, handle, email_verified, created_at

BEGIN;

-- [1] ALTER_DEFAULT: public.todos.body
ALTER TABLE public.todos ALTER COLUMN body SET DEFAULT '';

-- [HAZARD CONSTRAINT_SCAN] 
-- [2] SET_NOT_NULL: public.todos
ALTER TABLE public.todos ALTER COLUMN body SET NOT NULL;

COMMIT;
