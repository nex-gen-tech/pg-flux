CREATE TABLE public.bookmarks (
  id            uuid                   PRIMARY KEY DEFAULT gen_random_uuid(),
  user_id       uuid                   NOT NULL REFERENCES public.users (id) ON DELETE CASCADE,
  collection_id smallint               REFERENCES public.collections (id) ON DELETE SET NULL,
  url           text                   NOT NULL,
  title         text                   NOT NULL,
  title_lower   text                   GENERATED ALWAYS AS (lower(title)) STORED,
  notes         text                   NOT NULL DEFAULT '',
  search_vector tsvector               GENERATED ALWAYS AS (to_tsvector('english', coalesce(title, '') || ' ' || coalesce(notes, ''))) STORED,
  metadata      jsonb                  NOT NULL DEFAULT '{}',
  status        public.bookmark_status NOT NULL DEFAULT 'unread',
  deleted_at    timestamptz,
  created_at    timestamptz            NOT NULL DEFAULT now(),
  updated_at    timestamptz            NOT NULL DEFAULT now(),

  CONSTRAINT bookmarks_url_format    CHECK (url LIKE 'http%'),
  CONSTRAINT bookmarks_notes_length  CHECK (length(notes) <= 5000)
);

CREATE INDEX idx_bookmarks_user_id      ON public.bookmarks (user_id);
CREATE INDEX idx_bookmarks_status       ON public.bookmarks (user_id, status) WHERE status = ANY (ARRAY['unread'::bookmark_status, 'reading'::bookmark_status]);
CREATE INDEX idx_bookmarks_active       ON public.bookmarks (user_id, created_at DESC) WHERE deleted_at IS NULL;
CREATE INDEX idx_bookmarks_title_lower   ON public.bookmarks (title_lower);
CREATE INDEX idx_bookmarks_search_vector ON public.bookmarks USING GIN (search_vector);
CREATE INDEX idx_bookmarks_metadata      ON public.bookmarks USING GIN (metadata);
