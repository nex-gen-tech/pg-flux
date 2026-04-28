-- pg-flux generated migration
-- DO NOT EDIT unless you know what you are doing.

-- [1] CREATE_INDEX: public.idx_posts_published
CREATE INDEX CONCURRENTLY idx_posts_published ON public.posts USING btree (user_id, created_at DESC) WHERE status = 'published';

