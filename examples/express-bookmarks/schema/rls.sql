ALTER TABLE public.bookmarks ENABLE ROW LEVEL SECURITY;

CREATE POLICY bookmarks_owner_only ON public.bookmarks
  FOR ALL
  USING (user_id = current_setting('app.user_id', true)::uuid);
