-- pg-flux generated migration
-- DO NOT EDIT unless you know what you are doing.

-- [1] CREATE_INDEX: public.idx_posts_user_title
CREATE UNIQUE INDEX CONCURRENTLY idx_posts_user_title ON public.posts USING btree (user_id, title);

