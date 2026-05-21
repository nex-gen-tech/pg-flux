-- pg-flux generated migration
-- DO NOT EDIT unless you know what you are doing.
-- pg-flux-baseline-hash: e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855

BEGIN;

-- [1] CREATE_TYPE: public.bookmark_status
DO $pgflux$ BEGIN CREATE TYPE public.bookmark_status AS ENUM ('unread', 'reading', 'read', 'archived'); EXCEPTION WHEN duplicate_object THEN NULL; END $pgflux$;

-- [2] CREATE_TABLE: public.tags
CREATE TABLE IF NOT EXISTS public.tags (
  id serial PRIMARY KEY,
  name text NOT NULL,
  CONSTRAINT tags_name_unique UNIQUE (name)
);

-- [3] CREATE_TABLE: public.users
CREATE TABLE IF NOT EXISTS public.users (
  id uuid DEFAULT gen_random_uuid() PRIMARY KEY,
  email text NOT NULL,
  handle text NOT NULL,
  email_verified pg_catalog.bool DEFAULT false NOT NULL,
  created_at timestamptz DEFAULT now() NOT NULL,
  CONSTRAINT users_email_format CHECK (email LIKE '%@%'),
  CONSTRAINT users_email_unique UNIQUE (email),
  CONSTRAINT users_username_unique UNIQUE (handle)
);

-- [4] CREATE_TABLE: public.collections
CREATE TABLE IF NOT EXISTS public.collections (
  id smallserial PRIMARY KEY,
  user_id uuid NOT NULL,
  name text NOT NULL,
  color text DEFAULT 'blue' NOT NULL,
  CONSTRAINT collections_user_name_unique UNIQUE (user_id, name),
  CONSTRAINT collections_user_id_fkey FOREIGN KEY (user_id) REFERENCES public.users (id) ON DELETE CASCADE
);

-- [5] CREATE_TABLE: public.bookmarks
CREATE TABLE IF NOT EXISTS public.bookmarks (
  id uuid DEFAULT gen_random_uuid() PRIMARY KEY,
  user_id uuid NOT NULL,
  collection_id pg_catalog.int2,
  url text NOT NULL,
  title text NOT NULL,
  title_lower text GENERATED ALWAYS AS (lower(title)) STORED,
  notes text DEFAULT '' NOT NULL,
  search_vector tsvector GENERATED ALWAYS AS (to_tsvector('english', (COALESCE(title, '') || ' ') || COALESCE(notes, ''))) STORED,
  metadata jsonb DEFAULT '{}' NOT NULL,
  status public.bookmark_status DEFAULT 'unread' NOT NULL,
  deleted_at timestamptz,
  created_at timestamptz DEFAULT now() NOT NULL,
  updated_at timestamptz DEFAULT now() NOT NULL,
  CONSTRAINT bookmarks_url_format CHECK (url LIKE 'http%'),
  CONSTRAINT bookmarks_notes_length CHECK (length(notes) <= 5000),
  CONSTRAINT bookmarks_user_id_fkey FOREIGN KEY (user_id) REFERENCES public.users (id) ON DELETE CASCADE,
  CONSTRAINT bookmarks_collection_id_fkey FOREIGN KEY (collection_id) REFERENCES public.collections (id) ON DELETE SET NULL
);

-- [6] CREATE_TABLE: public.bookmark_tags
CREATE TABLE IF NOT EXISTS public.bookmark_tags (
  bookmark_id uuid NOT NULL,
  tag_id pg_catalog.int4 NOT NULL,
  CONSTRAINT bookmark_tags_bookmark_id_fkey FOREIGN KEY (bookmark_id) REFERENCES public.bookmarks (id) ON DELETE CASCADE,
  CONSTRAINT bookmark_tags_tag_id_fkey FOREIGN KEY (tag_id) REFERENCES public.tags (id) ON DELETE CASCADE,
  PRIMARY KEY (bookmark_id, tag_id)
);

-- [7] CREATE_FUNCTION: public.count_user_bookmarks(uuid)
CREATE OR REPLACE FUNCTION public.count_user_bookmarks(p_user_id uuid) RETURNS bigint LANGUAGE sql STABLE AS $$
  SELECT count(*) FROM public.bookmarks WHERE user_id = p_user_id;
$$;

-- [8] CREATE_FUNCTION: public.set_updated_at()
CREATE OR REPLACE FUNCTION public.set_updated_at() RETURNS trigger LANGUAGE plpgsql AS $$
BEGIN
  NEW.updated_at = now();
  RETURN NEW;
END;
$$;

-- [9] TOGGLE_RLS: public.bookmarks
ALTER TABLE public.bookmarks ENABLE ROW LEVEL SECURITY;

-- [10] TOGGLE_RLS_NOFORCE: public.bookmarks
ALTER TABLE public.bookmarks NO FORCE ROW LEVEL SECURITY;

-- [11] CREATE_POLICY: public.bookmarks/bookmarks_owner_only
CREATE POLICY bookmarks_owner_only ON public.bookmarks TO public USING (user_id = current_setting('app.user_id', true)::uuid);

-- [12] CREATE_TRIGGER: public.bookmarks/bookmarks_set_updated_at
CREATE TRIGGER bookmarks_set_updated_at BEFORE UPDATE ON public.bookmarks FOR EACH ROW EXECUTE FUNCTION public.set_updated_at();

-- [21] CREATE_VIEW: public.unread_bookmarks
CREATE OR REPLACE VIEW public.unread_bookmarks AS SELECT b.id, b.user_id, b.title, b.url, b.status, b.created_at FROM public.bookmarks b WHERE b.status = 'unread';

COMMIT;

-- The following statements use CONCURRENTLY and run outside the transaction.
-- [13] CREATE_INDEX: public.idx_bookmark_tags_tag
CREATE INDEX CONCURRENTLY IF NOT EXISTS idx_bookmark_tags_tag ON public.bookmark_tags USING btree (tag_id);

-- [14] CREATE_INDEX: public.idx_bookmarks_active
CREATE INDEX CONCURRENTLY IF NOT EXISTS idx_bookmarks_active ON public.bookmarks USING btree (user_id, created_at DESC) WHERE deleted_at IS NULL;

-- [15] CREATE_INDEX: public.idx_bookmarks_metadata
CREATE INDEX CONCURRENTLY IF NOT EXISTS idx_bookmarks_metadata ON public.bookmarks USING gin (metadata);

-- [16] CREATE_INDEX: public.idx_bookmarks_search_vector
CREATE INDEX CONCURRENTLY IF NOT EXISTS idx_bookmarks_search_vector ON public.bookmarks USING gin (search_vector);

-- [17] CREATE_INDEX: public.idx_bookmarks_status
CREATE INDEX CONCURRENTLY IF NOT EXISTS idx_bookmarks_status ON public.bookmarks USING btree (user_id, status) WHERE status = ANY(ARRAY['unread'::bookmark_status, 'reading'::bookmark_status]);

-- [18] CREATE_INDEX: public.idx_bookmarks_title_lower
CREATE INDEX CONCURRENTLY IF NOT EXISTS idx_bookmarks_title_lower ON public.bookmarks USING btree (title_lower);

-- [19] CREATE_INDEX: public.idx_bookmarks_user_id
CREATE INDEX CONCURRENTLY IF NOT EXISTS idx_bookmarks_user_id ON public.bookmarks USING btree (user_id);

-- [20] CREATE_INDEX: public.idx_users_created_at
CREATE INDEX CONCURRENTLY IF NOT EXISTS idx_users_created_at ON public.users USING btree (created_at DESC);

