-- s15: view body change (WHERE clause).
CREATE TABLE public.users (
  id    bigserial PRIMARY KEY,
  email text NOT NULL,
  CONSTRAINT users_email_unique UNIQUE (email),
  CONSTRAINT users_email_at CHECK (email LIKE '%@%')
);
GRANT SELECT ON TABLE public.users TO app_reader;
GRANT SELECT, INSERT, UPDATE ON TABLE public.users TO app_writer;
COMMENT ON TABLE public.users IS 'Identity record for every account.';
COMMENT ON COLUMN public.users.email IS 'Login identifier.';
CREATE TABLE public.posts (
  id         bigserial PRIMARY KEY,
  user_id    bigint NOT NULL,
  title      text NOT NULL,
  body       text NOT NULL DEFAULT '',
  status     text NOT NULL DEFAULT 'draft',
  updated_at timestamptz NOT NULL DEFAULT now(),
  CONSTRAINT posts_user_fk FOREIGN KEY (user_id) REFERENCES public.users (id)
);
GRANT SELECT ON TABLE public.posts TO PUBLIC;
COMMENT ON TABLE public.posts IS 'User-authored content.';
CREATE INDEX idx_posts_user ON public.posts (user_id);
CREATE INDEX idx_posts_published ON public.posts (updated_at) INCLUDE (title) WHERE status = 'published';
COMMENT ON INDEX public.idx_posts_published IS 'Hot-path index for the public feed.';
CREATE OR REPLACE FUNCTION public.set_updated_at() RETURNS trigger
  LANGUAGE plpgsql STABLE PARALLEL SAFE COST 8
AS $$
BEGIN NEW.updated_at := clock_timestamp(); RETURN NEW; END;
$$;
CREATE TRIGGER posts_set_updated_at BEFORE UPDATE ON public.posts FOR EACH ROW EXECUTE FUNCTION public.set_updated_at();
ALTER TABLE public.users ENABLE ROW LEVEL SECURITY;
CREATE POLICY users_select ON public.users FOR SELECT USING (id > 0);
-- changed WHERE clause: 'published' → IN ('published', 'featured')
CREATE VIEW public.published_posts AS SELECT posts.id, posts.title, posts.body FROM public.posts WHERE posts.status IN ('published', 'featured');
CREATE SEQUENCE public.demo_seq START 500 INCREMENT 10 CACHE 1;
CREATE TYPE public.user_role AS ENUM ('admin', 'user', 'moderator');
CREATE DOMAIN public.short_text AS text CHECK (length(VALUE) <= 100);
