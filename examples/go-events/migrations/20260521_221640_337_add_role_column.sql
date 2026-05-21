-- pg-flux generated migration
-- DO NOT EDIT unless you know what you are doing.
-- pg-flux-baseline-hash: e3d0b7875e062d458e9a6322265a19f72445579ae99d213a0e998b70b9d25508

BEGIN;

-- [1] ADD_COLUMN: public.users.role
ALTER TABLE public.users ADD COLUMN IF NOT EXISTS role public.user_role NOT NULL DEFAULT 'member';

COMMIT;
