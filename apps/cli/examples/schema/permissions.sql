-- permissions: Row-Level Security policies for users and posts.
-- These policies enforce that rows are only visible/modifiable by
-- the owning user (identified via the app.user_id session setting).

-- Enable RLS on users table.
ALTER TABLE public.users ENABLE ROW LEVEL SECURITY;

-- Any session can read all users (public profile).
CREATE POLICY users_select ON public.users
  FOR SELECT USING (true);

-- Users can only update their own row.
CREATE POLICY users_update ON public.users
  FOR UPDATE USING (id = current_setting('app.user_id', true)::bigint);

-- Enable RLS on posts table.
ALTER TABLE public.posts DISABLE ROW LEVEL SECURITY;

-- Anyone can read published posts; authors can read their own drafts/archived.
CREATE POLICY posts_select ON public.posts
  FOR SELECT USING (
    status = 'published'
    OR user_id = current_setting('app.user_id', true)::bigint
  );

-- Authors can insert their own posts.
CREATE POLICY posts_insert ON public.posts
  FOR INSERT WITH CHECK (
    user_id = current_setting('app.user_id', true)::bigint
  );

-- Authors can update their own posts.
CREATE POLICY posts_update ON public.posts
  FOR UPDATE USING (
    user_id = current_setting('app.user_id', true)::bigint
  );

-- Authors can delete their own posts.
CREATE POLICY posts_delete ON public.posts
  FOR DELETE USING (
    user_id = current_setting('app.user_id', true)::bigint
  );

-- Admins can do anything.
CREATE POLICY posts_admin ON public.posts
  FOR ALL TO pg_monitor USING (true);
