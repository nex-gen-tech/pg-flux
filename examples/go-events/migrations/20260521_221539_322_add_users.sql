-- pg-flux generated migration
-- DO NOT EDIT unless you know what you are doing.
-- pg-flux-baseline-hash: 52294552873d9c9984d9180da121410e2d9ec82c659acfdf324bc46fe985368f

BEGIN;

-- [1] CREATE_TABLE: public.users
CREATE TABLE IF NOT EXISTS public.users (
  id pg_catalog.int8 GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
  workspace_id pg_catalog.int8 NOT NULL,
  email text NOT NULL,
  username text NOT NULL,
  created_at timestamptz DEFAULT now() NOT NULL,
  CONSTRAINT users_email_format CHECK (email LIKE '%@%'),
  CONSTRAINT users_workspace_email_unique UNIQUE (workspace_id, email),
  CONSTRAINT users_workspace_id_fkey FOREIGN KEY (workspace_id) REFERENCES public.workspaces (id) ON DELETE CASCADE
);

COMMIT;

-- The following statements use CONCURRENTLY and run outside the transaction.
-- [2] CREATE_INDEX: public.idx_users_created_at
CREATE INDEX CONCURRENTLY IF NOT EXISTS idx_users_created_at ON public.users USING btree (created_at DESC);

-- [3] CREATE_INDEX: public.idx_users_workspace_id
CREATE INDEX CONCURRENTLY IF NOT EXISTS idx_users_workspace_id ON public.users USING btree (workspace_id);

