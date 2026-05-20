-- pg-flux generated migration
-- DO NOT EDIT unless you know what you are doing.

-- [1] CREATE_POLICY: public.posts/posts_admin
CREATE POLICY posts_admin ON public.posts TO pg_monitor USING (true);

