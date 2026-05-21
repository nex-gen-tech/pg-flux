-- pg-flux generated migration
-- DO NOT EDIT unless you know what you are doing.
-- pg-flux-baseline-hash: 7bbaf7659deae52ebbc209800b3a148eaf03dd36be370393a1faed460c984bf9

-- [ADVISORY COLUMN_REORDER] Column order in public.attendees differs from desired schema; reordering requires table recreation. Desired order (surviving cols): event_id, user_id, status, registered_at
-- [ADVISORY COLUMN_REORDER] Column order in public.events differs from desired schema; reordering requires table recreation. Desired order (surviving cols): id, workspace_id, title, description, status, starts_at, ends_at, location, capacity, metadata, tags, deleted_at, created_at, updated_at

BEGIN;

-- [1] ADD_COLUMN: public.events.title_lower
ALTER TABLE public.events ADD COLUMN IF NOT EXISTS title_lower text GENERATED ALWAYS AS (lower(title)) STORED;

COMMIT;

-- The following statements use CONCURRENTLY and run outside the transaction.
-- [2] CREATE_INDEX: public.idx_events_title_lower
CREATE INDEX CONCURRENTLY IF NOT EXISTS idx_events_title_lower ON public.events USING btree (title_lower);

