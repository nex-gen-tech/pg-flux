-- pg-flux generated migration
-- DO NOT EDIT unless you know what you are doing.
-- pg-flux-baseline-hash: 7646161420265b98623222a78ca4b8018b6359375f08a63b9927163d7a2ce8a9

-- [ADVISORY COLUMN_REORDER] Column order in public.todos differs from desired schema; reordering requires table recreation. Desired order (surviving cols): id, user_id, category_id, title, body, done, created_at, updated_at
-- [ADVISORY COLUMN_REORDER] Column order in public.users differs from desired schema; reordering requires table recreation. Desired order (surviving cols): id, email, handle, email_verified, created_at

BEGIN;

-- [1] CREATE_TYPE: public.todo_priority
DO $pgflux$ BEGIN CREATE TYPE public.todo_priority AS ENUM ('low', 'normal', 'high', 'urgent'); EXCEPTION WHEN duplicate_object THEN NULL; END $pgflux$;

COMMIT;
