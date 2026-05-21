ALTER TABLE public.events ENABLE ROW LEVEL SECURITY;

CREATE POLICY events_workspace_isolation ON public.events
  FOR ALL
  USING (workspace_id = current_setting('app.workspace_id', true)::bigint);
