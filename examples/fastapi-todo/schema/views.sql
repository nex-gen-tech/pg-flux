CREATE OR REPLACE VIEW public.active_todos AS
  SELECT t.id, t.user_id, t.title, t.priority, t.created_at
  FROM public.todos t
  WHERE t.done = false;
