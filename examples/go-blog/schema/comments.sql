CREATE TABLE public.comments (
    id         bigint      GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    post_id    bigint      NOT NULL REFERENCES public.posts(id) ON DELETE CASCADE,
    author_id  bigint      NOT NULL REFERENCES public.users(id) ON DELETE CASCADE,
    body       text        NOT NULL,
    created_at timestamptz NOT NULL DEFAULT now()
);

CREATE INDEX comments_post_id_idx ON public.comments (post_id);
