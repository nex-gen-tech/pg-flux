-- pg-flux generated migration
-- DO NOT EDIT unless you know what you are doing.

-- [1] RENAME_COLUMN: public.users.screen_name
ALTER TABLE public.users RENAME COLUMN nickname TO screen_name;

-- [2] ADD_TABLE_CONSTRAINT: public.users/users_fullname_check
ALTER TABLE public.users ADD CONSTRAINT users_fullname_check CHECK (full_name IS NULL OR char_length(full_name) > 0);

