-- pg-flux generated migration
-- DO NOT EDIT unless you know what you are doing.
-- pg-flux-baseline-hash: 4828de5c1fa97fdf5157c97e0443ec27aed93d34eec7ed2f9fa295b84d19937b

BEGIN;

-- [1] RENAME_COLUMN: public.users.display_name
ALTER TABLE public.users RENAME COLUMN username TO display_name;

COMMIT;
