-- employee_directory: read-only view with security_invoker (PG15+).
-- WITH (security_invoker = true) means the view runs as the calling user, not
-- the view owner — enforces RLS policies on the underlying tables.
CREATE OR REPLACE VIEW public.employee_directory
  WITH (security_invoker = true)
AS
  SELECT
    e.id,
    e.org_id,
    e.first_name,
    e.last_name,
    e.full_name,
    e.email,
    e.phone,
    e.status,
    e.hire_date,
    e.skills,
    d.name  AS department_name,
    d.code  AS department_code
  FROM public.employees e
  LEFT JOIN public.departments d ON d.id = e.department_id
  WHERE e.deleted_at IS NULL;

-- department_stats: materialized view that uses a window function.
-- rank() OVER (...) partitions by org_id so each org gets its own ranking.
-- Demonstrates: FILTER clause, window function, subquery wrapping.
CREATE MATERIALIZED VIEW public.department_stats AS
  SELECT
    department_id,
    department_name,
    org_id,
    employee_count,
    active_count,
    rank() OVER (
      PARTITION BY org_id
      ORDER BY employee_count DESC
    ) AS size_rank
  FROM (
    SELECT
      d.id                                                   AS department_id,
      d.name                                                 AS department_name,
      d.org_id,
      count(e.id)                                            AS employee_count,
      count(e.id) FILTER (WHERE e.status = 'active')        AS active_count
    FROM public.departments d
    LEFT JOIN public.employees e
           ON e.department_id = d.id
          AND e.deleted_at IS NULL
    GROUP BY d.id, d.name, d.org_id
  ) sub;

-- Unique index required so REFRESH MATERIALIZED VIEW CONCURRENTLY works.
CREATE UNIQUE INDEX idx_department_stats_id ON public.department_stats (department_id);
