-- pg-flux generated migration
-- DO NOT EDIT unless you know what you are doing.
-- pg-flux-baseline-hash: e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855

BEGIN;

-- [1] CREATE_TABLE: public.users
CREATE TABLE IF NOT EXISTS public.users (
  id bigserial PRIMARY KEY,
  email text NOT NULL,
  username text NOT NULL,
  created_at timestamptz DEFAULT now() NOT NULL,
  CONSTRAINT users_email_format CHECK (email LIKE '%@%'),
  CONSTRAINT users_email_unique UNIQUE (email),
  CONSTRAINT users_username_unique UNIQUE (username)
);

COMMIT;

-- The following statements use CONCURRENTLY and run outside the transaction.
-- [2] CREATE_INDEX: public.idx_users_created_at
CREATE INDEX CONCURRENTLY IF NOT EXISTS idx_users_created_at ON public.users USING btree (created_at DESC);

