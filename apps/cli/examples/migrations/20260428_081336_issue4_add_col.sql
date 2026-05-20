-- pg-flux generated migration
-- DO NOT EDIT unless you know what you are doing.

-- [1] ADD_COLUMN: public.users.test_score2
ALTER TABLE public.users ADD COLUMN test_score2 pg_catalog.int4 DEFAULT 0;

-- [ADVISORY COLUMN_REORDER] Column order in public.posts differs from desired schema; reordering requires table recreation. Desired order (surviving cols): id, user_id, title, body, order, visibility, status, created_at, updated_at

-- [ADVISORY COLUMN_REORDER] Column order in public.users differs from desired schema; reordering requires table recreation. Desired order (surviving cols): id, email, full_name, is_verified, phone, status, created_at, updated_at, test_score, search_name

