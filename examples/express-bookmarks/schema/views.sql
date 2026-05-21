CREATE OR REPLACE VIEW public.unread_bookmarks AS
  SELECT b.id, b.user_id, b.title, b.url, b.status, b.created_at
  FROM public.bookmarks b
  WHERE b.status = 'unread';
