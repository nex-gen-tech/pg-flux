-- pg-flux generated migration
-- DO NOT EDIT unless you know what you are doing.
-- pg-flux-baseline-hash: f4320154ba4c5db4a4cad3269b46920489e9e4cf737b706578f7cf7da7fb92d7

-- [ADVISORY COLUMN_REORDER] Column order in public.todos differs from desired schema; reordering requires table recreation. Desired order (surviving cols): id, user_id, category_id, title, body, done, created_at, updated_at

BEGIN;

-- [1] RENAME_COLUMN: public.users.handle
ALTER TABLE public.users RENAME COLUMN username TO handle;

COMMIT;
