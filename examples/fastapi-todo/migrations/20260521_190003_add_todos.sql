-- pg-flux generated migration
-- DO NOT EDIT unless you know what you are doing.
-- pg-flux-baseline-hash: cc0acab2c1833730289aad720eec3cca550e6148f2e11d5f0608cc4434acd759

BEGIN;

-- [1] CREATE_TABLE: public.todos
CREATE TABLE IF NOT EXISTS public.todos (
  id bigserial PRIMARY KEY,
  user_id pg_catalog.int8 NOT NULL,
  title text NOT NULL,
  body text,
  done pg_catalog.bool DEFAULT false NOT NULL,
  created_at timestamptz DEFAULT now() NOT NULL,
  updated_at timestamptz DEFAULT now() NOT NULL,
  CONSTRAINT todos_title_not_blank CHECK (length(TRIM (BOTH  FROM title)) > 0),
  CONSTRAINT todos_user_id_fkey FOREIGN KEY (user_id) REFERENCES public.users (id) ON DELETE CASCADE
);

COMMIT;

-- The following statements use CONCURRENTLY and run outside the transaction.
-- [2] CREATE_INDEX: public.idx_todos_user_done
CREATE INDEX CONCURRENTLY IF NOT EXISTS idx_todos_user_done ON public.todos USING btree (user_id, done) WHERE done = false;

-- [3] CREATE_INDEX: public.idx_todos_user_id
CREATE INDEX CONCURRENTLY IF NOT EXISTS idx_todos_user_id ON public.todos USING btree (user_id);

