-- pg-flux generated migration
-- DO NOT EDIT unless you know what you are doing.
-- pg-flux-baseline-hash: 0652ce10254cd9f5e07090882db8efae23c52bb6a0749b096f5b050f9e4e623d

-- [ADVISORY COLUMN_REORDER] Column order in public.todos differs from desired schema; reordering requires table recreation. Desired order (surviving cols): id, user_id, category_id, title, body, done, created_at, updated_at

BEGIN;

-- [1] ADD_COLUMN: public.users.email_verified
ALTER TABLE public.users ADD COLUMN IF NOT EXISTS email_verified pg_catalog.bool NOT NULL DEFAULT false;

COMMIT;
