-- pg-flux generated migration
-- DO NOT EDIT unless you know what you are doing.
-- pg-flux-baseline-hash: 95207c547370e9a6867b6c449bda82f861ee0c48a4d14b1e5edd53701089b231

-- [ADVISORY COLUMN_REORDER] Column order in public.events differs from desired schema; reordering requires table recreation. Desired order (surviving cols): id, workspace_id, title, description, status, starts_at, ends_at, location, capacity, created_at
-- [ADVISORY COLUMN_REORDER] Column order in public.users differs from desired schema; reordering requires table recreation. Desired order (surviving cols): id, workspace_id, email, display_name, role, created_at

BEGIN;

-- [1] ADD_COLUMN: public.events.metadata
ALTER TABLE public.events ADD COLUMN IF NOT EXISTS metadata jsonb NOT NULL DEFAULT '{}';

-- [2] ADD_COLUMN: public.events.tags
ALTER TABLE public.events ADD COLUMN IF NOT EXISTS tags text[] NOT NULL DEFAULT '{}';

COMMIT;

-- The following statements use CONCURRENTLY and run outside the transaction.
-- [3] CREATE_INDEX: public.idx_events_metadata
CREATE INDEX CONCURRENTLY IF NOT EXISTS idx_events_metadata ON public.events USING gin (metadata);

-- [4] CREATE_INDEX: public.idx_events_tags
CREATE INDEX CONCURRENTLY IF NOT EXISTS idx_events_tags ON public.events USING gin (tags);

