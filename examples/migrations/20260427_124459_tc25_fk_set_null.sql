-- pg-flux generated migration
-- DO NOT EDIT unless you know what you are doing.

-- [1] DROP_NOT_NULL: public.posts
ALTER TABLE public.posts ALTER COLUMN user_id DROP NOT NULL;

-- [2] ADD_TABLE_CONSTRAINT: public.posts/posts_user_fk
ALTER TABLE public.posts ADD CONSTRAINT posts_user_fk FOREIGN KEY (user_id) REFERENCES public.users (id) ON DELETE SET NULL;

