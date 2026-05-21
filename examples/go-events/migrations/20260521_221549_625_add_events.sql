-- pg-flux generated migration
-- DO NOT EDIT unless you know what you are doing.
-- pg-flux-baseline-hash: 6da9f610fee0901938a69274869b6b7fffac3cd2a787e08cabc2d1c664fbf26d

BEGIN;

-- [1] CREATE_TABLE: public.events
CREATE TABLE IF NOT EXISTS public.events (
  id pg_catalog.int8 GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
  workspace_id pg_catalog.int8 NOT NULL,
  title text NOT NULL,
  starts_at timestamptz NOT NULL,
  ends_at timestamptz NOT NULL,
  created_at timestamptz DEFAULT now() NOT NULL,
  CONSTRAINT events_ends_after_starts CHECK (ends_at > starts_at),
  CONSTRAINT events_workspace_id_fkey FOREIGN KEY (workspace_id) REFERENCES public.workspaces (id) ON DELETE CASCADE
);

COMMIT;

-- The following statements use CONCURRENTLY and run outside the transaction.
-- [2] CREATE_INDEX: public.idx_events_starts_at
CREATE INDEX CONCURRENTLY IF NOT EXISTS idx_events_starts_at ON public.events USING btree (workspace_id, starts_at);

-- [3] CREATE_INDEX: public.idx_events_workspace_id
CREATE INDEX CONCURRENTLY IF NOT EXISTS idx_events_workspace_id ON public.events USING btree (workspace_id);

