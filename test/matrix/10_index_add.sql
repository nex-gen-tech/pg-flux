-- s10: add a new index with INCLUDE column + partial predicate.
CREATE TABLE public.users (
  id    bigserial PRIMARY KEY,
  email text NOT NULL,
  -- @renamed from=name
  full_name varchar(200) DEFAULT '',
  CONSTRAINT users_email_unique UNIQUE (email),
  CONSTRAINT users_email_at CHECK (email LIKE '%@%')
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
CREATE INDEX idx_posts_published ON public.posts (updated_at) INCLUDE (title) WHERE status = 'published';
CREATE OR REPLACE FUNCTION public.set_updated_at() RETURNS trigger
  LANGUAGE plpgsql STABLE PARALLEL SAFE COST 8
AS $$
BEGIN NEW.updated_at := clock_timestamp(); RETURN NEW; END;
$$;
CREATE TRIGGER posts_set_updated_at BEFORE UPDATE ON public.posts FOR EACH ROW EXECUTE FUNCTION public.set_updated_at();
ALTER TABLE public.users ENABLE ROW LEVEL SECURITY;
CREATE POLICY users_select ON public.users FOR SELECT USING (id > 0);
CREATE VIEW public.published_posts AS SELECT posts.id, posts.title, posts.body FROM public.posts WHERE posts.status = 'published';
CREATE SEQUENCE public.demo_seq START 500 INCREMENT 10 CACHE 1;
CREATE TYPE public.user_role AS ENUM ('admin', 'user', 'moderator');
CREATE DOMAIN public.short_text AS text CHECK (length(VALUE) <= 100);
