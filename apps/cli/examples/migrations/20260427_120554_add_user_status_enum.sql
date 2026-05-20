-- pg-flux generated migration
-- DO NOT EDIT unless you know what you are doing.

-- [1] CREATE_TYPE: raw
DO $pgflux$ BEGIN CREATE TYPE public.user_status AS ENUM ('active', 'suspended', 'deleted'); EXCEPTION WHEN duplicate_object THEN NULL; END $pgflux$;

-- [2] DROP_TABLE_CONSTRAINT: public.users/users_status_valid
ALTER TABLE public.users DROP CONSTRAINT IF EXISTS users_status_valid;

-- [3] ALTER_COLUMN_TYPE:
ALTER TABLE public.users ALTER COLUMN status SET DATA TYPE public.user_status USING status::public.user_status;
