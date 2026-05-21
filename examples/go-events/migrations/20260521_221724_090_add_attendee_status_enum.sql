-- pg-flux generated migration
-- DO NOT EDIT unless you know what you are doing.
-- pg-flux-baseline-hash: a589cb93b4e61244f828e6a986331b2f8a3170cb99c3b773bae91c019c049d8b

-- [ADVISORY COLUMN_REORDER] Column order in public.events differs from desired schema; reordering requires table recreation. Desired order (surviving cols): id, workspace_id, title, description, status, starts_at, ends_at, location, capacity, metadata, tags, created_at

BEGIN;

-- [1] CREATE_TYPE: public.attendee_status
DO $pgflux$ BEGIN CREATE TYPE public.attendee_status AS ENUM ('invited', 'confirmed', 'declined', 'waitlisted'); EXCEPTION WHEN duplicate_object THEN NULL; END $pgflux$;

COMMIT;
