-- posts: content items authored by users.
CREATE TYPE public.post_status AS ENUM ('draft', 'published', 'archived', 'deleted');
CREATE TYPE public.visibility_level AS ENUM ('public', 'private', 'friends_only');

CREATE TABLE public.posts (
  id          bigserial              PRIMARY KEY,
  user_id     bigint,
  title       text                   NOT NULL,
  body        text               NOT NULL DEFAULT '',  "order"     integer                NOT NULL DEFAULT 0,  visibility  public.visibility_level NOT NULL DEFAULT 'public',
  status      public.post_status     NOT NULL DEFAULT 'draft',
  created_at  timestamptz            NOT NULL DEFAULT now(),
  updated_at  timestamptz            NOT NULL DEFAULT now(),

  CONSTRAINT posts_title_nonempty CHECK (trim(title) <> ''),
  CONSTRAINT posts_user_title_unique UNIQUE (user_id, title),
  CONSTRAINT posts_user_fk FOREIGN KEY (user_id)
    REFERENCES public.users (id) ON DELETE SET NULL
);

CREATE INDEX idx_posts_user_id        ON public.posts (user_id);
CREATE UNIQUE INDEX idx_posts_user_title ON public.posts (user_id, title);
CREATE INDEX idx_posts_published      ON public.posts (user_id, created_at DESC) WHERE status = 'published';
CREATE INDEX idx_posts_created        ON public.posts (created_at DESC);
CREATE INDEX idx_posts_title_fts      ON public.posts (title text_pattern_ops);

-- Issue 21 test: inline (column-level) FOREIGN KEY references — must not be silently dropped.
CREATE TABLE public.comments (
  id         bigserial PRIMARY KEY,
  post_id    bigint    NOT NULL REFERENCES public.posts (id) ON DELETE CASCADE,
  user_id    bigint    REFERENCES public.users (id) ON DELETE SET NULL,
  body       text      NOT NULL
);
