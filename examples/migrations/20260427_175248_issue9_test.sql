-- pg-flux generated migration
-- DO NOT EDIT unless you know what you are doing.

-- [1] CREATE_POLICY: public.posts/posts_visible
CREATE POLICY posts_visible ON public.posts FOR SELECT TO public USING ((user_id > 0 AND status = 'published') OR user_id = current_setting('app.user_id', true)::bigint);

