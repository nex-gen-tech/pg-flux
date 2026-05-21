CREATE OR REPLACE FUNCTION public.count_user_bookmarks(p_user_id uuid)
RETURNS bigint
LANGUAGE sql
STABLE
AS $$
  SELECT count(*) FROM public.bookmarks WHERE user_id = p_user_id;
$$;
