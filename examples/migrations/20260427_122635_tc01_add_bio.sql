-- pg-flux generated migration
-- DO NOT EDIT unless you know what you are doing.

-- [1] ADD_COLUMN: public.users.bio
ALTER TABLE public.users ADD COLUMN bio text;

