-- Row-level security: every query is automatically scoped to the current org.
-- Set app.org_id at the start of each request: SET LOCAL app.org_id = '42'.
ALTER TABLE public.employees     ENABLE ROW LEVEL SECURITY;
ALTER TABLE public.departments   ENABLE ROW LEVEL SECURITY;
ALTER TABLE public.shifts        ENABLE ROW LEVEL SECURITY;
ALTER TABLE public.leave_requests ENABLE ROW LEVEL SECURITY;

CREATE POLICY employees_org_isolation ON public.employees
  FOR ALL
  USING (org_id = current_setting('app.org_id', true)::bigint);

CREATE POLICY departments_org_isolation ON public.departments
  FOR ALL
  USING (org_id = current_setting('app.org_id', true)::bigint);

CREATE POLICY shifts_org_isolation ON public.shifts
  FOR ALL
  USING (org_id = current_setting('app.org_id', true)::bigint);

CREATE POLICY leave_org_isolation ON public.leave_requests
  FOR ALL
  USING (org_id = current_setting('app.org_id', true)::bigint);
