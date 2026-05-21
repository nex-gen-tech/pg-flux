ALTER TABLE public.todos ENABLE ROW LEVEL SECURITY;

CREATE POLICY todos_owner_only ON public.todos
  FOR ALL
  USING (user_id = current_setting('app.user_id', true)::bigint);
