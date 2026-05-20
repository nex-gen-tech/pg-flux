-- pg-flux generated migration
-- DO NOT EDIT unless you know what you are doing.

-- [ADVISORY COLUMN_REORDER] Column order in public.posts differs from desired schema; reordering requires table recreation. Desired order (surviving cols): id, user_id, title, body, order, visibility, status, created_at, updated_at
-- [ADVISORY COLUMN_REORDER] Column order in public.users differs from desired schema; reordering requires table recreation. Desired order (surviving cols): id, email, full_name, is_verified, phone, status, created_at, updated_at, test_score, tags, utc_signup, search_name

BEGIN;

-- [1] CREATE_EXTENSION: pg_trgm
CREATE EXTENSION IF NOT EXISTS pg_trgm;

COMMIT;
