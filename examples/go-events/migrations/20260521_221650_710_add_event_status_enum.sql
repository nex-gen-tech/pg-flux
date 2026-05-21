-- pg-flux generated migration
-- DO NOT EDIT unless you know what you are doing.
-- pg-flux-baseline-hash: 2453b06fa57a588ff7a59e7bf4a28e11e0b1ac1bae8ffc2f4befd78c1b938842

-- [ADVISORY COLUMN_REORDER] Column order in public.users differs from desired schema; reordering requires table recreation. Desired order (surviving cols): id, workspace_id, email, display_name, role, created_at

BEGIN;

-- [1] CREATE_TYPE: public.event_status
DO $pgflux$ BEGIN CREATE TYPE public.event_status AS ENUM ('draft', 'published', 'cancelled'); EXCEPTION WHEN duplicate_object THEN NULL; END $pgflux$;

COMMIT;
