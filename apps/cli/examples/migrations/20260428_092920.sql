-- pg-flux generated migration
-- DO NOT EDIT unless you know what you are doing.

-- [ADVISORY COLUMN_REORDER] Column order in public.posts differs from desired schema; reordering requires table recreation. Desired order (surviving cols): id, user_id, title, body, order, visibility, status, created_at, updated_at
-- [ADVISORY COLUMN_REORDER] Column order in public.users differs from desired schema; reordering requires table recreation. Desired order (surviving cols): id, email, full_name, is_verified, phone, status, created_at, updated_at, test_score, tags, utc_signup, search_name

BEGIN;

-- [1] CREATE_TYPE: public.verification_status
DO $pgflux$ BEGIN CREATE TYPE public.verification_status AS ENUM ('unverified', 'verified', 'pending'); EXCEPTION WHEN duplicate_object THEN NULL; END $pgflux$;

-- [HAZARD DATA_LOSS] Drops column data
-- [2] DROP_COLUMN: public.users.issue9_test
ALTER TABLE public.users DROP COLUMN IF EXISTS issue9_test CASCADE;

-- [HAZARD DATA_LOSS] Drops view
-- [3] DROP_VIEW_EARLY: public.published_posts
DROP VIEW IF EXISTS public.published_posts CASCADE;

-- [4] ALTER_DEFAULT: public.users.is_verified
ALTER TABLE public.users ALTER COLUMN is_verified DROP DEFAULT;

-- [HAZARD COLUMN_TYPE_CHANGE] Column type change may rewrite table
-- [5] ALTER_COLUMN_TYPE: public.users.is_verified
ALTER TABLE public.users ALTER COLUMN is_verified SET DATA TYPE public.verification_status USING CASE is_verified WHEN TRUE THEN 'verified'::public.verification_status ELSE 'unverified'::public.verification_status END;

-- [6] ALTER_DEFAULT: public.users.is_verified
ALTER TABLE public.users ALTER COLUMN is_verified SET DEFAULT 'unverified';

-- [7] CREATE_VIEW: public.published_posts
CREATE VIEW public.published_posts AS SELECT p.id, p.title, p.body, p.created_at, u.handle AS author, u.email AS author_email FROM public.posts p JOIN public.users u ON u.id = p.user_id WHERE p.status = 'published';

-- [HAZARD DATA_LOSS] Drops extension and dependent objects
-- [8] DROP_EXTENSION: pg_trgm
DROP EXTENSION IF EXISTS pg_trgm CASCADE;

COMMIT;
