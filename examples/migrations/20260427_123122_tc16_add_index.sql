-- pg-flux generated migration
-- DO NOT EDIT unless you know what you are doing.

-- [1] CREATE_INDEX: public.idx_posts_title_fts
CREATE INDEX CONCURRENTLY idx_posts_title_fts ON public.posts USING btree (title text_pattern_ops);

