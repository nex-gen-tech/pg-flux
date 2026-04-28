-- categories: optional topic tags for posts.
-- @renamed from=categories
CREATE TABLE public.tags (
  id    bigserial PRIMARY KEY,
  name  text      NOT NULL,

  CONSTRAINT tags_name_unique UNIQUE (name)
);

CREATE TABLE public.post_categories (
  post_id     bigint NOT NULL,
  category_id bigint NOT NULL,

  CONSTRAINT post_categories_pk          PRIMARY KEY (post_id, category_id),
  CONSTRAINT post_categories_post_fk     FOREIGN KEY (post_id)     REFERENCES public.posts (id) ON DELETE CASCADE,
  CONSTRAINT post_categories_category_fk FOREIGN KEY (category_id) REFERENCES public.tags (id) ON DELETE CASCADE
);
