CREATE TABLE public.tags (
  id    serial PRIMARY KEY,
  name  text NOT NULL,
  CONSTRAINT tags_name_unique UNIQUE (name)
);
