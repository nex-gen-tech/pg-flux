-- pg-flux generated migration
-- DO NOT EDIT unless you know what you are doing.

-- [1] ADD_COLUMN: public.users.tags
ALTER TABLE public.users ADD COLUMN tags text[] DEFAULT ARRAY[]::text[];

-- [2] ADD_COLUMN: public.users.utc_signup
ALTER TABLE public.users ADD COLUMN utc_signup timestamptz DEFAULT pg_catalog.timezone('UTC', now());

-- [ADVISORY COLUMN_REORDER] Column order in public.posts differs from desired schema; reordering requires table recreation. Desired order (surviving cols): id, user_id, title, body, order, visibility, status, created_at, updated_at

-- [ADVISORY COLUMN_REORDER] Column order in public.users differs from desired schema; reordering requires table recreation. Desired order (surviving cols): id, email, full_name, is_verified, phone, status, created_at, updated_at, test_score, search_name

