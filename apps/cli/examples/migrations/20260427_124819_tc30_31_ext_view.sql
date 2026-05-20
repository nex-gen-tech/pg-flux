-- pg-flux generated migration
-- DO NOT EDIT unless you know what you are doing.

-- [1] CREATE_EXTENSION: pg_trgm
CREATE EXTENSION IF NOT EXISTS pg_trgm;

-- [2] CREATE_VIEW: public.published_posts
CREATE VIEW public.published_posts AS SELECT p.id, p.title, p.body, p.created_at, u.username AS author FROM public.posts p JOIN public.users u ON u.id = p.user_id WHERE p.status = 'published';

