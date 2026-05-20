-- pg-flux generated migration
-- DO NOT EDIT unless you know what you are doing.

-- [1] DROP_POLICY: public.posts/posts_visible
DROP POLICY IF EXISTS posts_visible ON public.posts;

