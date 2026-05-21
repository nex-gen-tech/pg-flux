CREATE TABLE public.categories (
  id        bigserial PRIMARY KEY,
  parent_id int REFERENCES public.categories (id) ON DELETE SET NULL,
  slug      text NOT NULL,
  name      text NOT NULL,
  CONSTRAINT categories_slug_unique UNIQUE (slug)
);
