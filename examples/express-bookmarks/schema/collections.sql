CREATE TABLE public.collections (
  id      smallserial PRIMARY KEY,
  user_id uuid        NOT NULL REFERENCES public.users (id) ON DELETE CASCADE,
  -- @renamed from=title
  name    text        NOT NULL,
  color   text        NOT NULL DEFAULT 'blue',
  CONSTRAINT collections_user_name_unique UNIQUE (user_id, name)
);
