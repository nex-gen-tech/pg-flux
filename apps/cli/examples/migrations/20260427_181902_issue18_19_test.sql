-- pg-flux generated migration
-- DO NOT EDIT unless you know what you are doing.

-- [1] RENAME_COLUMN: public.users.nickname
ALTER TABLE public.users RENAME COLUMN display_name TO nickname;

-- [2] ADD_TABLE_CONSTRAINT: public.users/users_nickname_len
ALTER TABLE public.users ADD CONSTRAINT users_nickname_len CHECK (char_length(nickname) <= 50);

-- [3] DROP_INDEX: public.idx_users_display_name
DROP INDEX CONCURRENTLY IF EXISTS public.idx_users_display_name;

-- [4] CREATE_INDEX: public.idx_users_nickname
CREATE INDEX CONCURRENTLY idx_users_nickname ON public.users USING btree (nickname) WHERE nickname IS NOT NULL;

