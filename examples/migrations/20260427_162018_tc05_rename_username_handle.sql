-- pg-flux generated migration
-- DO NOT EDIT unless you know what you are doing.

-- [1] RENAME_COLUMN: public.users.handle
ALTER TABLE public.users RENAME COLUMN username TO handle;

-- [2] DROP_TABLE_CONSTRAINT: public.users/users_username_unique
ALTER TABLE public.users DROP CONSTRAINT IF EXISTS users_username_unique;

-- [3] ADD_TABLE_CONSTRAINT: public.users/users_handle_unique
ALTER TABLE public.users ADD CONSTRAINT users_handle_unique UNIQUE (handle);

-- [4] DROP_VIEW: public.published_posts
DROP VIEW IF EXISTS public.published_posts CASCADE;

-- [5] CREATE_VIEW: public.published_posts
CREATE VIEW public.published_posts AS SELECT p.id, p.title, p.body, p.created_at, u.handle AS author, u.email AS author_email FROM public.posts p JOIN public.users u ON u.id = p.user_id WHERE p.status = 'published';

