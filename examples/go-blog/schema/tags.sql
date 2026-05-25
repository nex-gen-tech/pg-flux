CREATE TABLE public.tags (
    id    bigint GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    name  text   NOT NULL UNIQUE
);

CREATE TABLE public.post_tags (
    post_id bigint NOT NULL REFERENCES public.posts(id) ON DELETE CASCADE,
    tag_id  bigint NOT NULL REFERENCES public.tags(id)  ON DELETE CASCADE,
    PRIMARY KEY (post_id, tag_id)
);
