-- pg-flux generated migration
-- DO NOT EDIT unless you know what you are doing.

-- [1] ADD_COLUMN: public.users.display_name
ALTER TABLE public.users ADD COLUMN display_name text;

-- [2] ALTER_DEFAULT: 
ALTER TABLE public.users ALTER COLUMN status DROP DEFAULT;

-- [3] CREATE_INDEX: public.idx_users_display_name
CREATE INDEX CONCURRENTLY idx_users_display_name ON public.users USING btree (display_name) WHERE display_name IS NOT NULL;

