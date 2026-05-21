CREATE TABLE public.categories (
  id    smallserial PRIMARY KEY,
  name  text NOT NULL,
  color text NOT NULL DEFAULT 'gray',
  CONSTRAINT categories_name_unique UNIQUE (name)
);
