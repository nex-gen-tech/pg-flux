-- pg-flux generated migration
-- DO NOT EDIT unless you know what you are doing.

-- [ADVISORY COLUMN_REORDER] Column order in public.posts differs from desired schema; reordering requires table recreation. Desired order (surviving cols): id, user_id, title, body, order, visibility, status, created_at, updated_at
-- [ADVISORY COLUMN_REORDER] Column order in public.users differs from desired schema; reordering requires table recreation. Desired order (surviving cols): id, email, full_name, is_verified, phone, status, created_at, updated_at, test_score, tags, utc_signup, search_name

BEGIN;

-- [1] RAW_DDL: raw
ALTER SEQUENCE IF EXISTS public.counter_seq INCREMENT BY 10 MINVALUE 1 MAXVALUE 9223372036854775807 CACHE 2 NO CYCLE;

COMMIT;
