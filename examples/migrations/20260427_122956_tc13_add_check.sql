-- pg-flux generated migration
-- DO NOT EDIT unless you know what you are doing.

-- [1] ADD_TABLE_CONSTRAINT: public.posts/posts_body_len
ALTER TABLE public.posts ADD CONSTRAINT posts_body_len CHECK (length(body) < 10000);

