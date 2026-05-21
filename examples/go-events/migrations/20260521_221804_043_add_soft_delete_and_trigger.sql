-- pg-flux generated migration
-- DO NOT EDIT unless you know what you are doing.
-- pg-flux-baseline-hash: 27823d1cc82fe944ef4aaeea9eb79acd11e409ae4991756e9a6409f6ce6b81fb

-- [ADVISORY COLUMN_REORDER] Column order in public.users differs from desired schema; reordering requires table recreation. Desired order (surviving cols): id, workspace_id, email, display_name, role, created_at

BEGIN;

-- [1] ADD_COLUMN: public.events.deleted_at
ALTER TABLE public.events ADD COLUMN IF NOT EXISTS deleted_at timestamptz;

-- [2] ADD_COLUMN: public.events.updated_at
ALTER TABLE public.events ADD COLUMN IF NOT EXISTS updated_at timestamptz NOT NULL DEFAULT now();

-- [3] CREATE_FUNCTION: public.set_updated_at()
CREATE OR REPLACE FUNCTION public.set_updated_at() RETURNS trigger LANGUAGE plpgsql AS $$
BEGIN
  NEW.updated_at = now();
  RETURN NEW;
END;
$$;

-- [4] CREATE_TRIGGER: public.events/events_set_updated_at
CREATE TRIGGER events_set_updated_at BEFORE UPDATE ON public.events FOR EACH ROW EXECUTE FUNCTION public.set_updated_at();

COMMIT;

-- The following statements use CONCURRENTLY and run outside the transaction.
-- [5] DROP_INDEX: public.idx_events_starts_at
DROP INDEX CONCURRENTLY IF EXISTS public.idx_events_starts_at;

-- [6] DROP_INDEX: public.idx_events_status
DROP INDEX CONCURRENTLY IF EXISTS public.idx_events_status;

-- [7] CREATE_INDEX: public.idx_events_active
CREATE INDEX CONCURRENTLY IF NOT EXISTS idx_events_active ON public.events USING btree (workspace_id, created_at DESC) WHERE deleted_at IS NULL;

-- [8] CREATE_INDEX: public.idx_events_starts_at
CREATE INDEX CONCURRENTLY IF NOT EXISTS idx_events_starts_at ON public.events USING btree (workspace_id, starts_at) WHERE deleted_at IS NULL;

-- [9] CREATE_INDEX: public.idx_events_status
CREATE INDEX CONCURRENTLY IF NOT EXISTS idx_events_status ON public.events USING btree (workspace_id, status) WHERE deleted_at IS NULL;

-- [10] CREATE_INDEX: public.idx_events_workspace_title_active
CREATE UNIQUE INDEX CONCURRENTLY IF NOT EXISTS idx_events_workspace_title_active ON public.events USING btree (workspace_id, title) WHERE deleted_at IS NULL;

