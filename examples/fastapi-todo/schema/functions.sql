CREATE OR REPLACE FUNCTION public.count_user_todos(p_user_id bigint)
RETURNS bigint
LANGUAGE sql
STABLE
AS $$
  SELECT count(*) FROM public.todos WHERE user_id = p_user_id;
$$;
