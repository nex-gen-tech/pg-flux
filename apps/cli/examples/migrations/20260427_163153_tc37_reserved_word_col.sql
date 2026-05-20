-- pg-flux generated migration
-- DO NOT EDIT unless you know what you are doing.

-- [1] ADD_COLUMN: public.posts.order
ALTER TABLE public.posts ADD COLUMN "order" pg_catalog.int4 NOT NULL DEFAULT 0;

