-- s02: add a column with a DEFAULT.
CREATE TABLE public.users (
  id    bigserial PRIMARY KEY,
  email text NOT NULL,
  name  text DEFAULT '',
  CONSTRAINT users_email_unique UNIQUE (email)
);
CREATE TABLE public.posts (
  id         bigserial PRIMARY KEY,
  user_id    bigint NOT NULL,
  title      text NOT NULL,
  body       text NOT NULL DEFAULT '',
  status     text NOT NULL DEFAULT 'draft',
  updated_at timestamptz NOT NULL DEFAULT now(),
  CONSTRAINT posts_user_fk FOREIGN KEY (user_id) REFERENCES public.users (id)
);
CREATE INDEX idx_posts_user ON public.posts (user_id);
CREATE OR REPLACE FUNCTION public.set_updated_at() RETURNS trigger LANGUAGE plpgsql AS $$
BEGIN NEW.updated_at := clock_timestamp(); RETURN NEW; END;
$$;
CREATE TRIGGER posts_set_updated_at BEFORE UPDATE ON public.posts FOR EACH ROW EXECUTE FUNCTION public.set_updated_at();
ALTER TABLE public.users ENABLE ROW LEVEL SECURITY;
CREATE POLICY users_select ON public.users FOR SELECT USING (true);
CREATE VIEW public.published_posts AS SELECT posts.id, posts.title, posts.body FROM public.posts WHERE posts.status = 'published';
CREATE SEQUENCE public.demo_seq START 100 INCREMENT 5 CACHE 1;
CREATE TYPE public.user_role AS ENUM ('admin', 'user');
CREATE DOMAIN public.short_text AS text CHECK (length(VALUE) <= 100);
