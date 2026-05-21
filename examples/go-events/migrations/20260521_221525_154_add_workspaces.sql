-- pg-flux generated migration
-- DO NOT EDIT unless you know what you are doing.
-- pg-flux-baseline-hash: e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855

BEGIN;

-- [1] CREATE_TABLE: public.workspaces
CREATE TABLE IF NOT EXISTS public.workspaces (
  id pg_catalog.int8 GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
  slug text NOT NULL,
  name text NOT NULL,
  plan text DEFAULT 'free' NOT NULL,
  CONSTRAINT workspaces_slug_format CHECK (slug ~ '^[a-z0-9-]+$'),
  CONSTRAINT workspaces_slug_unique UNIQUE (slug)
);

COMMIT;

-- The following statements use CONCURRENTLY and run outside the transaction.
-- [2] CREATE_INDEX: public.idx_workspaces_slug
CREATE INDEX CONCURRENTLY IF NOT EXISTS idx_workspaces_slug ON public.workspaces USING btree (slug);

