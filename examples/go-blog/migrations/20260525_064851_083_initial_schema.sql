-- +migrate Up

-- pg-flux generated migration
-- DO NOT EDIT unless you know what you are doing.
-- pg-flux-baseline-hash: e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855

BEGIN;

-- [1] CREATE_TYPE: public.post_status
DO $pgflux$ BEGIN CREATE TYPE public.post_status AS ENUM ('draft', 'published', 'archived'); EXCEPTION WHEN duplicate_object THEN NULL; END $pgflux$;

-- [2] CREATE_TABLE: public.tags
CREATE TABLE IF NOT EXISTS public.tags (
  id pg_catalog.int8 GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
  name text NOT NULL
);

-- [3] CREATE_TABLE: public.users
CREATE TABLE IF NOT EXISTS public.users (
  id pg_catalog.int8 GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
  handle text NOT NULL,
  email text NOT NULL,
  bio text,
  created_at timestamptz DEFAULT now() NOT NULL,
  updated_at timestamptz DEFAULT now() NOT NULL
);

-- [4] CREATE_TABLE: public.posts
CREATE TABLE IF NOT EXISTS public.posts (
  id pg_catalog.int8 GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
  author_id pg_catalog.int8 NOT NULL,
  slug text NOT NULL,
  title text NOT NULL,
  body text DEFAULT '' NOT NULL,
  status public.post_status DEFAULT 'draft' NOT NULL,
  published_at timestamptz,
  created_at timestamptz DEFAULT now() NOT NULL,
  updated_at timestamptz DEFAULT now() NOT NULL,
  CONSTRAINT posts_author_id_fkey FOREIGN KEY (author_id) REFERENCES public.users (id) ON DELETE CASCADE
);

-- [5] CREATE_TABLE: public.post_tags
CREATE TABLE IF NOT EXISTS public.post_tags (
  post_id pg_catalog.int8 NOT NULL,
  tag_id pg_catalog.int8 NOT NULL,
  CONSTRAINT post_tags_post_id_fkey FOREIGN KEY (post_id) REFERENCES public.posts (id) ON DELETE CASCADE,
  CONSTRAINT post_tags_tag_id_fkey FOREIGN KEY (tag_id) REFERENCES public.tags (id) ON DELETE CASCADE,
  PRIMARY KEY (post_id, tag_id)
);

-- [6] CREATE_FUNCTION: public.set_updated_at()
CREATE OR REPLACE FUNCTION public.set_updated_at() RETURNS trigger LANGUAGE plpgsql AS $$
BEGIN
    NEW.updated_at = now();
    RETURN NEW;
END;
$$;

-- [7] CREATE_TRIGGER: public.posts/posts_updated_at
CREATE TRIGGER posts_updated_at BEFORE UPDATE ON public.posts FOR EACH ROW EXECUTE FUNCTION public.set_updated_at();

-- [8] CREATE_TRIGGER: public.users/users_updated_at
CREATE TRIGGER users_updated_at BEFORE UPDATE ON public.users FOR EACH ROW EXECUTE FUNCTION public.set_updated_at();

-- [12] CREATE_VIEW: public.published_posts
CREATE VIEW public.published_posts AS SELECT p.id, p.slug, p.title, p.body, p.published_at, u.handle AS author_handle FROM public.posts p JOIN public.users u ON u.id = p.author_id WHERE p.status = 'published';

COMMIT;

-- The following statements use CONCURRENTLY and run outside the transaction.
-- [9] CREATE_INDEX: public.posts_author_id_idx
CREATE INDEX CONCURRENTLY IF NOT EXISTS posts_author_id_idx ON public.posts USING btree (author_id);

-- [10] CREATE_INDEX: public.posts_slug_idx
CREATE INDEX CONCURRENTLY IF NOT EXISTS posts_slug_idx ON public.posts USING btree (slug);

-- [11] CREATE_INDEX: public.posts_status_idx
CREATE INDEX CONCURRENTLY IF NOT EXISTS posts_status_idx ON public.posts USING btree (status);



-- +migrate Down

-- pg-flux generated UNDO migration
-- Review carefully before applying. Some operations cannot be auto-reversed.

-- ============================================================
-- MANUAL UNDO REQUIRED for the following operations:
-- ============================================================
-- MANUAL: drop created trigger public.users/users_updated_at
-- MANUAL: drop created trigger public.posts/posts_updated_at

BEGIN;

DROP VIEW IF EXISTS "public"."published_posts" CASCADE;

DROP INDEX IF EXISTS "public"."posts_status_idx";

DROP INDEX IF EXISTS "public"."posts_slug_idx";

DROP INDEX IF EXISTS "public"."posts_author_id_idx";

DROP FUNCTION IF EXISTS "public"."set_updated_at"() CASCADE;

DROP TABLE IF EXISTS "public"."post_tags" CASCADE;

DROP TABLE IF EXISTS "public"."posts" CASCADE;

DROP TABLE IF EXISTS "public"."users" CASCADE;

DROP TABLE IF EXISTS "public"."tags" CASCADE;

DROP TYPE IF EXISTS "public"."post_status" CASCADE;

COMMIT;
