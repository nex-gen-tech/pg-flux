-- pg-flux generated migration
-- DO NOT EDIT unless you know what you are doing.

-- [1] ADD_COLUMN: public.users.is_verified
ALTER TABLE public.users ADD COLUMN is_verified pg_catalog.bool NOT NULL DEFAULT false;

