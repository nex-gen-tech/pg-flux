-- pg-flux generated migration
-- DO NOT EDIT unless you know what you are doing.
-- pg-flux-baseline-hash: 2453b06fa57a588ff7a59e7bf4a28e11e0b1ac1bae8ffc2f4befd78c1b938842

BEGIN;

-- [1] ADD_COLUMN: public.events.capacity
ALTER TABLE public.events ADD COLUMN IF NOT EXISTS capacity pg_catalog.int4;

-- [2] ADD_COLUMN: public.events.description
ALTER TABLE public.events ADD COLUMN IF NOT EXISTS description text NOT NULL DEFAULT '';

-- [3] ADD_COLUMN: public.events.location
ALTER TABLE public.events ADD COLUMN IF NOT EXISTS location text;

-- [4] ADD_COLUMN: public.events.status
ALTER TABLE public.events ADD COLUMN IF NOT EXISTS status public.event_status NOT NULL DEFAULT 'draft';

-- [HAZARD CONSTRAINT_SCAN] Adding constraint may scan table
-- [5] ADD_TABLE_CONSTRAINT: public.events/events_capacity_positive
ALTER TABLE public.events ADD CONSTRAINT events_capacity_positive CHECK (capacity IS NULL OR capacity > 0) NOT VALID;

COMMIT;

-- The following statements use CONCURRENTLY and run outside the transaction.
-- [6] VALIDATE_TABLE_CONSTRAINT: public.events/events_capacity_positive
ALTER TABLE public.events VALIDATE CONSTRAINT events_capacity_positive;

-- [7] CREATE_INDEX: public.idx_events_status
CREATE INDEX CONCURRENTLY IF NOT EXISTS idx_events_status ON public.events USING btree (workspace_id, status);

