-- pg-flux generated migration
-- DO NOT EDIT unless you know what you are doing.

-- [1] DROP_TABLE_CONSTRAINT: public.posts/posts_body_len
ALTER TABLE public.posts DROP CONSTRAINT IF EXISTS posts_body_len;

