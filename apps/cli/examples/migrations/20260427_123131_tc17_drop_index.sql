-- pg-flux generated migration
-- DO NOT EDIT unless you know what you are doing.

-- [1] DROP_INDEX: public.idx_posts_status
DROP INDEX CONCURRENTLY IF EXISTS public.idx_posts_status;

