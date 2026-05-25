-- +migrate Up

-- pg-flux generated migration
-- DO NOT EDIT unless you know what you are doing.
-- pg-flux-baseline-hash: 8a920d31afe9372b0b638d2787fb9a10a4562a37f4c42ba32320c201b1749c2e

BEGIN;

-- [1] CREATE_TABLE: public.comments
CREATE TABLE IF NOT EXISTS public.comments (
  id pg_catalog.int8 GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
  post_id pg_catalog.int8 NOT NULL,
  author_id pg_catalog.int8 NOT NULL,
  body text NOT NULL,
  created_at timestamptz DEFAULT now() NOT NULL,
  CONSTRAINT comments_post_id_fkey FOREIGN KEY (post_id) REFERENCES public.posts (id) ON DELETE CASCADE,
  CONSTRAINT comments_author_id_fkey FOREIGN KEY (author_id) REFERENCES public.users (id) ON DELETE CASCADE
);

COMMIT;

-- The following statements use CONCURRENTLY and run outside the transaction.
-- [2] CREATE_INDEX: public.comments_post_id_idx
CREATE INDEX CONCURRENTLY IF NOT EXISTS comments_post_id_idx ON public.comments USING btree (post_id);



-- +migrate Down

-- pg-flux generated UNDO migration
-- Review carefully before applying. Some operations cannot be auto-reversed.

BEGIN;

DROP INDEX IF EXISTS "public"."comments_post_id_idx";

DROP TABLE IF EXISTS "public"."comments" CASCADE;

COMMIT;
