-- pg-flux generated migration
-- DO NOT EDIT unless you know what you are doing.

-- [1] DROP_VIEW_EARLY: public.published_posts
DROP VIEW IF EXISTS public.published_posts CASCADE;

-- [2] ALTER_COLUMN_TYPE: 
ALTER TABLE public.users ALTER COLUMN test_score2 SET DATA TYPE pg_catalog.int8 USING test_score2::pg_catalog.int8;

-- [3] CREATE_VIEW: public.published_posts
CREATE VIEW public.published_posts AS SELECT p.id, p.title, p.body, p.created_at, u.handle AS author, u.email AS author_email FROM public.posts p JOIN public.users u ON u.id = p.user_id WHERE p.status = 'published';

-- [ADVISORY COLUMN_REORDER] Column order in public.posts differs from desired schema; reordering requires table recreation. Desired order (surviving cols): id, user_id, title, body, order, visibility, status, created_at, updated_at

-- [ADVISORY COLUMN_REORDER] Column order in public.users differs from desired schema; reordering requires table recreation. Desired order (surviving cols): id, email, full_name, is_verified, phone, status, created_at, updated_at, test_score, test_score2, search_name

