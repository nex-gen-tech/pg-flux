-- pg-flux generated migration
-- DO NOT EDIT unless you know what you are doing.

-- [1] ADD_COLUMN: public.users.phone
ALTER TABLE public.users ADD COLUMN phone text NOT NULL;

