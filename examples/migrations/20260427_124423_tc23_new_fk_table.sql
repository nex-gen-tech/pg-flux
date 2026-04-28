-- pg-flux generated migration
-- DO NOT EDIT unless you know what you are doing.

-- [1] CREATE_TABLE: public.categories
CREATE TABLE public.categories (
  id bigserial PRIMARY KEY,
  name text NOT NULL,
  CONSTRAINT categories_name_unique UNIQUE (name)
);

-- [2] CREATE_TABLE: public.post_categories
CREATE TABLE public.post_categories (
  post_id pg_catalog.int8 NOT NULL,
  category_id pg_catalog.int8 NOT NULL,
  CONSTRAINT post_categories_post_fk FOREIGN KEY (post_id) REFERENCES public.posts (id) ON DELETE CASCADE,
  CONSTRAINT post_categories_category_fk FOREIGN KEY (category_id) REFERENCES public.categories (id) ON DELETE CASCADE
);

