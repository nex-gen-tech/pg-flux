-- pg-flux generated migration
-- DO NOT EDIT unless you know what you are doing.
-- pg-flux-baseline-hash: a589cb93b4e61244f828e6a986331b2f8a3170cb99c3b773bae91c019c049d8b

-- [ADVISORY COLUMN_REORDER] Column order in public.users differs from desired schema; reordering requires table recreation. Desired order (surviving cols): id, workspace_id, email, display_name, role, created_at

BEGIN;

-- [1] ADD_COLUMN: public.attendees.status
ALTER TABLE public.attendees ADD COLUMN IF NOT EXISTS status public.attendee_status NOT NULL DEFAULT 'invited';

COMMIT;

-- The following statements use CONCURRENTLY and run outside the transaction.
-- [2] CREATE_INDEX: public.idx_attendees_status
CREATE INDEX CONCURRENTLY IF NOT EXISTS idx_attendees_status ON public.attendees USING btree (event_id, status);

