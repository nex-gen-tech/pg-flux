CREATE TABLE public.bookmark_tags (
  bookmark_id uuid NOT NULL REFERENCES public.bookmarks (id) ON DELETE CASCADE,
  tag_id      int  NOT NULL REFERENCES public.tags      (id) ON DELETE CASCADE,
  PRIMARY KEY (bookmark_id, tag_id)
);

CREATE INDEX idx_bookmark_tags_tag ON public.bookmark_tags (tag_id);
