-- views: convenient pre-joined queries.

CREATE VIEW public.published_posts AS
  SELECT
    p.id,
    p.title,
    p.body,
    p.created_at,
    u.handle    AS author,
    u.email     AS author_email
  FROM public.posts p
  JOIN public.users u ON u.id = p.user_id
  WHERE p.status = 'published';
