CREATE VIEW public.published_posts AS
SELECT
    p.id,
    p.slug,
    p.title,
    p.body,
    p.published_at,
    u.handle AS author_handle
FROM public.posts p
JOIN public.users u ON u.id = p.author_id
WHERE p.status = 'published';
