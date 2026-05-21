-- pg-flux generated migration
-- DO NOT EDIT unless you know what you are doing.
-- pg-flux-baseline-hash: e3d0b7875e062d458e9a6322265a19f72445579ae99d213a0e998b70b9d25508

BEGIN;

-- [1] CREATE_TYPE: public.user_role
DO $pgflux$ BEGIN CREATE TYPE public.user_role AS ENUM ('member', 'admin', 'owner'); EXCEPTION WHEN duplicate_object THEN NULL; END $pgflux$;

COMMIT;
