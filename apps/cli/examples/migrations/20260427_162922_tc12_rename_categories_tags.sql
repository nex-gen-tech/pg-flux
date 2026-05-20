-- pg-flux generated migration
-- DO NOT EDIT unless you know what you are doing.

-- [1] RENAME_TABLE: public.tags
ALTER TABLE public.categories RENAME TO tags;

-- [2] DROP_TABLE_CONSTRAINT: public.post_categories/post_categories_category_fk
ALTER TABLE public.post_categories DROP CONSTRAINT IF EXISTS post_categories_category_fk;

-- [3] DROP_TABLE_CONSTRAINT: public.tags/categories_name_unique
ALTER TABLE public.tags DROP CONSTRAINT IF EXISTS categories_name_unique;

-- [4] ADD_TABLE_CONSTRAINT: public.post_categories/post_categories_category_fk
ALTER TABLE public.post_categories ADD CONSTRAINT post_categories_category_fk FOREIGN KEY (category_id) REFERENCES public.tags (id) ON DELETE CASCADE;

-- [5] ADD_TABLE_CONSTRAINT: public.tags/tags_name_unique
ALTER TABLE public.tags ADD CONSTRAINT tags_name_unique UNIQUE (name);

