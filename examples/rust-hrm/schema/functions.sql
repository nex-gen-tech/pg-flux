-- ── Trigger helper ───────────────────────────────────────────────────────────

CREATE OR REPLACE FUNCTION public.set_updated_at()
RETURNS trigger
LANGUAGE plpgsql
AS $$
BEGIN
  NEW.updated_at = now();
  RETURN NEW;
END;
$$;

-- ── Audit log ────────────────────────────────────────────────────────────────

-- SECURITY DEFINER: runs as the function owner, not the caller, so it can
-- always INSERT into audit.change_log regardless of caller's privileges.
CREATE OR REPLACE FUNCTION audit.log_change()
RETURNS trigger
LANGUAGE plpgsql
SECURITY DEFINER
AS $$
DECLARE
  v_old  jsonb;
  v_new  jsonb;
  v_id   text;
BEGIN
  v_old := CASE WHEN TG_OP IN ('UPDATE', 'DELETE') THEN row_to_json(OLD)::jsonb ELSE NULL END;
  v_new := CASE WHEN TG_OP IN ('UPDATE', 'INSERT') THEN row_to_json(NEW)::jsonb ELSE NULL END;
  v_id  := CASE
             WHEN TG_OP = 'DELETE' THEN (row_to_json(OLD)::jsonb ->> 'id')
             ELSE (row_to_json(NEW)::jsonb ->> 'id')
           END;

  INSERT INTO audit.change_log (table_name, operation, row_id, old_data, new_data, changed_by)
  VALUES (TG_TABLE_NAME, TG_OP, v_id, v_old, v_new,
          current_setting('app.user_id', true));

  RETURN CASE WHEN TG_OP = 'DELETE' THEN OLD ELSE NEW END;
END;
$$;

-- ── Application functions ─────────────────────────────────────────────────────

-- Scalar STABLE function: count active employees for an org.
-- Codegen emits CountActiveEmployeesParams + CountActiveEmployeesRow.
CREATE OR REPLACE FUNCTION public.count_active_employees(p_org_id bigint)
RETURNS bigint
LANGUAGE sql
STABLE
AS $$
  SELECT count(*)
  FROM public.employees
  WHERE org_id = p_org_id
    AND status  = 'active'
    AND deleted_at IS NULL;
$$;

-- RETURNS TABLE function: fetch all employees for a department.
-- Demonstrates multi-column return shape; codegen emits GetDepartmentEmployeesResult.
-- Note: PG drops schema-qualification on user-defined types in RETURNS TABLE,
-- so we omit it here to avoid perpetual drift.
CREATE OR REPLACE FUNCTION public.get_department_employees(p_department_id bigint)
RETURNS TABLE (
  employee_id  uuid,
  full_name    text,
  email        text,
  status       employee_status,
  hire_date    date
)
LANGUAGE sql
STABLE
AS $$
  SELECT id, full_name, email, status, hire_date
  FROM public.employees
  WHERE department_id = p_department_id
    AND deleted_at    IS NULL
  ORDER BY last_name, first_name;
$$;

-- ── Stored procedure ──────────────────────────────────────────────────────────

-- onboard_employee: atomically inserts an employee and their first position.
-- A PROCEDURE (not a FUNCTION) because it may COMMIT internally; callers
-- use CALL, not SELECT. Demonstrates the fourth DDL object type that codegen
-- supports (alongside tables, views, and functions).
-- Note: PG drops schema-qualification on domain/enum types in procedure params
-- (pg_get_functiondef strips the prefix). Omit public. to keep drift clean.
CREATE OR REPLACE PROCEDURE public.onboard_employee(
  p_org_id        bigint,
  p_email         email_address,
  p_first_name    text,
  p_last_name     text,
  p_hire_date     date,
  p_department_id bigint,
  p_title         text,
  p_level         position_level
)
LANGUAGE plpgsql
AS $$
DECLARE
  v_employee_id uuid;
BEGIN
  INSERT INTO public.employees (
    org_id, email, first_name, last_name, hire_date, department_id
  )
  VALUES (
    p_org_id, p_email, p_first_name, p_last_name, p_hire_date, p_department_id
  )
  RETURNING id INTO v_employee_id;

  INSERT INTO public.positions (
    org_id, employee_id, title, level, department_id, valid_during
  )
  VALUES (
    p_org_id, v_employee_id, p_title, p_level, p_department_id,
    daterange(p_hire_date, NULL)
  );
END;
$$;
