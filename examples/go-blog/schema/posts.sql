CREATE TABLE public.posts (
    id           bigint          GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    author_id    bigint          NOT NULL REFERENCES public.users(id) ON DELETE CASCADE,
    slug         text            NOT NULL UNIQUE,
    title        text            NOT NULL,
    body         text            NOT NULL DEFAULT '',
    status       public.post_status NOT NULL DEFAULT 'draft',
    published_at timestamptz,
    created_at   timestamptz     NOT NULL DEFAULT now(),
    updated_at   timestamptz     NOT NULL DEFAULT now()
);

CREATE INDEX posts_author_id_idx  ON public.posts (author_id);
CREATE INDEX posts_status_idx     ON public.posts (status);
CREATE INDEX posts_slug_idx       ON public.posts (slug);
