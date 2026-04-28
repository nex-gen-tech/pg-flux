-- pg-flux generated migration
-- DO NOT EDIT unless you know what you are doing.

-- [1] CREATE_TYPE: raw
DO $pgflux$ BEGIN CREATE TYPE public.post_status AS ENUM ('draft', 'published', 'archived'); EXCEPTION WHEN duplicate_object THEN NULL; END $pgflux$;

-- [2] CREATE_TABLE: public.posts
CREATE TABLE public.posts (
  id bigserial PRIMARY KEY,
  user_id pg_catalog.int8 NOT NULL,
  title text NOT NULL,
  body text,
  status public.post_status DEFAULT 'draft' NOT NULL,
  created_at timestamptz DEFAULT now() NOT NULL,
  updated_at timestamptz DEFAULT now() NOT NULL,
  CONSTRAINT posts_title_nonempty CHECK (TRIM (BOTH  FROM title) <> ''),
  CONSTRAINT posts_user_fk FOREIGN KEY (user_id) REFERENCES public.users (id) ON DELETE CASCADE
);

-- [3] CREATE_POLICY: public.posts/posts_delete
CREATE POLICY posts_delete ON public.posts FOR DELETE TO public USING (user_id = current_setting('app.user_id', true)::bigint);

-- [4] CREATE_POLICY: public.posts/posts_insert
CREATE POLICY posts_insert ON public.posts FOR INSERT TO public WITH CHECK (user_id = current_setting('app.user_id', true)::bigint);

-- [5] CREATE_POLICY: public.posts/posts_select
CREATE POLICY posts_select ON public.posts FOR SELECT TO public USING (status = 'published' OR user_id = current_setting('app.user_id', true)::bigint);

-- [6] CREATE_POLICY: public.posts/posts_update
CREATE POLICY posts_update ON public.posts FOR UPDATE TO public USING (user_id = current_setting('app.user_id', true)::bigint);

-- [7] CREATE_POLICY: public.users/users_select
CREATE POLICY users_select ON public.users FOR SELECT TO public USING (true);

-- [8] CREATE_POLICY: public.users/users_update
CREATE POLICY users_update ON public.users FOR UPDATE TO public USING (id = current_setting('app.user_id', true)::bigint);

-- [9] CREATE_INDEX: public.idx_posts_created
CREATE INDEX CONCURRENTLY idx_posts_created ON public.posts USING btree (created_at DESC);

-- [10] CREATE_INDEX: public.idx_posts_status
CREATE INDEX CONCURRENTLY idx_posts_status ON public.posts USING btree (status);

-- [11] CREATE_INDEX: public.idx_posts_user_id
CREATE INDEX CONCURRENTLY idx_posts_user_id ON public.posts USING btree (user_id);

