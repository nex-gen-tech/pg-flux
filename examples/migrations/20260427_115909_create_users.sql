-- pg-flux generated migration
-- DO NOT EDIT unless you know what you are doing.

-- [1] CREATE_TABLE: public.users
CREATE TABLE public.users (
  id bigserial PRIMARY KEY,
  email text NOT NULL,
  username text NOT NULL,
  full_name text,
  status text NOT NULL,
  created_at timestamptz NOT NULL,
  updated_at timestamptz NOT NULL,
  CONSTRAINT users_email_format CHECK (email LIKE '%@%'),
  CONSTRAINT users_status_valid CHECK (status IN ('active', 'suspended', 'deleted')),
  CONSTRAINT users_email_unique UNIQUE (email),
  CONSTRAINT users_username_unique UNIQUE (username)
);

-- [2] CREATE_INDEX: public.idx_users_created
CREATE INDEX CONCURRENTLY idx_users_created ON public.users USING btree (created_at DESC);

-- [3] CREATE_INDEX: public.idx_users_email
CREATE INDEX CONCURRENTLY idx_users_email ON public.users USING btree (email);

-- [4] CREATE_INDEX: public.idx_users_status
CREATE INDEX CONCURRENTLY idx_users_status ON public.users USING btree (status);

