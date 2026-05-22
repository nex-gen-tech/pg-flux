-- Allow any authenticated role to SELECT from the public-facing views.
GRANT SELECT ON public.employee_directory TO PUBLIC;
GRANT SELECT ON public.department_stats   TO PUBLIC;
