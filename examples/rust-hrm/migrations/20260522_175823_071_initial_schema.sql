-- pg-flux generated migration
-- DO NOT EDIT unless you know what you are doing.
-- pg-flux-baseline-hash: e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855

BEGIN;

-- [1] CREATE_EXTENSION: btree_gist
CREATE EXTENSION IF NOT EXISTS btree_gist;

-- [2] CREATE_EXTENSION: pg_trgm
CREATE EXTENSION IF NOT EXISTS pg_trgm;

-- [3] CREATE_TYPE: public.employee_status
DO $pgflux$ BEGIN CREATE TYPE public.employee_status AS ENUM ('active', 'on_leave', 'suspended', 'terminated'); EXCEPTION WHEN duplicate_object THEN NULL; END $pgflux$;

-- [4] CREATE_TYPE: public.position_level
DO $pgflux$ BEGIN CREATE TYPE public.position_level AS ENUM ('junior', 'mid', 'senior', 'lead', 'principal', 'executive'); EXCEPTION WHEN duplicate_object THEN NULL; END $pgflux$;

-- [5] CREATE_TYPE: public.leave_type
DO $pgflux$ BEGIN CREATE TYPE public.leave_type AS ENUM ('annual', 'sick', 'parental', 'bereavement', 'unpaid', 'other'); EXCEPTION WHEN duplicate_object THEN NULL; END $pgflux$;

-- [6] CREATE_TYPE: public.shift_status
DO $pgflux$ BEGIN CREATE TYPE public.shift_status AS ENUM ('scheduled', 'confirmed', 'in_progress', 'completed', 'cancelled'); EXCEPTION WHEN duplicate_object THEN NULL; END $pgflux$;

-- [7] CREATE_TYPE: public.email_address
DO $pgflux$ BEGIN CREATE DOMAIN public.email_address AS text CONSTRAINT email_format CHECK (value ~ E'^[^@\\s]+@[^@\\s]+\\.[^@\\s]+$'); EXCEPTION WHEN duplicate_object THEN NULL; END $pgflux$;

-- [8] CREATE_TYPE: public.phone_number
DO $pgflux$ BEGIN CREATE DOMAIN public.phone_number AS text CONSTRAINT phone_format CHECK (value ~ E'^\\+?[1-9]\\d{6,14}$'); EXCEPTION WHEN duplicate_object THEN NULL; END $pgflux$;

-- [9] CREATE_TYPE: public.mailing_address
DO $pgflux$ BEGIN CREATE TYPE public.mailing_address AS (line1 text, line2 text, city text, state text, zip text, country text); EXCEPTION WHEN duplicate_object THEN NULL; END $pgflux$;

-- [10] CREATE_TABLE: audit.change_log
CREATE TABLE IF NOT EXISTS audit.change_log (
  id pg_catalog.int8 GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
  table_name text NOT NULL,
  operation text NOT NULL,
  row_id text,
  old_data jsonb,
  new_data jsonb,
  changed_by text,
  changed_at timestamptz DEFAULT now() NOT NULL,
  CONSTRAINT change_log_op_check CHECK (operation IN ('INSERT', 'UPDATE', 'DELETE'))
);

-- [11] CREATE_TABLE: public.attendance
CREATE TABLE IF NOT EXISTS public.attendance (
  id pg_catalog.int8 GENERATED ALWAYS AS IDENTITY,
  org_id pg_catalog.int8 NOT NULL,
  employee_id uuid NOT NULL,
  shift_id pg_catalog.int8,
  punch_in timestamptz NOT NULL,
  punch_out timestamptz,
  notes text,
  created_at timestamptz DEFAULT now() NOT NULL,
  CONSTRAINT attendance_punch_order CHECK (punch_out IS NULL OR punch_out > punch_in),
  PRIMARY KEY (id, punch_in)
) PARTITION BY RANGE (punch_in);

-- [12] CREATE_TABLE: public.departments
CREATE TABLE IF NOT EXISTS public.departments (
  id pg_catalog.int8 GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
  org_id pg_catalog.int8 NOT NULL,
  parent_id pg_catalog.int8,
  name text NOT NULL,
  code text NOT NULL,
  description text,
  depth pg_catalog.int2 DEFAULT 0 NOT NULL,
  metadata jsonb DEFAULT '{}' NOT NULL,
  created_at timestamptz DEFAULT now() NOT NULL,
  updated_at timestamptz DEFAULT now() NOT NULL,
  CONSTRAINT departments_depth_check CHECK (depth >= 0 AND depth <= 10),
  CONSTRAINT departments_code_org_unique UNIQUE (org_id, code),
  CONSTRAINT departments_parent_id_fkey FOREIGN KEY (parent_id) REFERENCES public.departments (id) ON DELETE SET NULL
);

-- [13] CREATE_TABLE: public.employees
CREATE TABLE IF NOT EXISTS public.employees (
  id uuid DEFAULT gen_random_uuid() PRIMARY KEY,
  org_id pg_catalog.int8 NOT NULL,
  department_id pg_catalog.int8,
  email public.email_address NOT NULL,
  phone public.phone_number,
  first_name text NOT NULL,
  last_name text NOT NULL,
  status public.employee_status DEFAULT 'active' NOT NULL,
  hire_date date NOT NULL,
  address public.mailing_address,
  skills text[] DEFAULT '{}' NOT NULL,
  metadata jsonb DEFAULT '{}' NOT NULL,
  full_name text GENERATED ALWAYS AS ((first_name || ' ') || last_name) STORED,
  email_lower text GENERATED ALWAYS AS (lower(email)) STORED,
  search_vector tsvector GENERATED ALWAYS AS (setweight(to_tsvector('english', (COALESCE(first_name, '') || ' ') || COALESCE(last_name, '')), 'A') || setweight(to_tsvector('english', COALESCE(email, '')), 'B')) STORED,
  created_at timestamptz DEFAULT now() NOT NULL,
  updated_at timestamptz DEFAULT now() NOT NULL,
  deleted_at timestamptz,
  CONSTRAINT employees_hire_date_check CHECK (hire_date <= (current_date + '30 days'::interval)),
  CONSTRAINT employees_email_org_unique UNIQUE (org_id, email),
  CONSTRAINT employees_department_id_fkey FOREIGN KEY (department_id) REFERENCES public.departments (id) ON DELETE SET NULL
);

-- [14] CREATE_TABLE: public.leave_requests
CREATE TABLE IF NOT EXISTS public.leave_requests (
  id pg_catalog.int8 GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
  org_id pg_catalog.int8 NOT NULL,
  employee_id uuid NOT NULL,
  leave_type public.leave_type NOT NULL,
  during daterange NOT NULL,
  reason text,
  approved_by uuid,
  approved_at timestamptz,
  status text DEFAULT 'pending' NOT NULL,
  created_at timestamptz DEFAULT now() NOT NULL,
  updated_at timestamptz DEFAULT now() NOT NULL,
  CONSTRAINT leave_status_values CHECK (status IN ('pending', 'approved', 'rejected', 'cancelled')),
  CONSTRAINT leave_dates_nonempty CHECK (NOT isempty(during)),
  CONSTRAINT leave_requests_employee_id_fkey FOREIGN KEY (employee_id) REFERENCES public.employees (id) ON DELETE CASCADE,
  CONSTRAINT leave_requests_approved_by_fkey FOREIGN KEY (approved_by) REFERENCES public.employees (id) ON DELETE SET NULL
);

-- [15] CREATE_TABLE: public.positions
CREATE TABLE IF NOT EXISTS public.positions (
  id pg_catalog.int8 GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
  org_id pg_catalog.int8 NOT NULL,
  employee_id uuid NOT NULL,
  title text NOT NULL,
  level public.position_level NOT NULL,
  department_id pg_catalog.int8,
  salary pg_catalog.numeric(12, 2),
  valid_during daterange NOT NULL,
  created_at timestamptz DEFAULT now() NOT NULL,
  CONSTRAINT positions_salary_positive CHECK (salary IS NULL OR salary > 0),
  CONSTRAINT positions_no_overlap EXCLUDE USING gist (employee_id WITH =, valid_during WITH &&),
  CONSTRAINT positions_employee_id_fkey FOREIGN KEY (employee_id) REFERENCES public.employees (id) ON DELETE CASCADE,
  CONSTRAINT positions_department_id_fkey FOREIGN KEY (department_id) REFERENCES public.departments (id) ON DELETE SET NULL
);

-- [16] CREATE_TABLE: public.shifts
CREATE TABLE IF NOT EXISTS public.shifts (
  id pg_catalog.int8 GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
  org_id pg_catalog.int8 NOT NULL,
  employee_id uuid NOT NULL,
  department_id pg_catalog.int8,
  title text NOT NULL,
  status public.shift_status DEFAULT 'scheduled' NOT NULL,
  during tstzrange NOT NULL,
  notes text,
  metadata jsonb DEFAULT '{}' NOT NULL,
  created_at timestamptz DEFAULT now() NOT NULL,
  updated_at timestamptz DEFAULT now() NOT NULL,
  CONSTRAINT shifts_title_nonempty CHECK (length(TRIM (BOTH  FROM title)) > 0),
  CONSTRAINT shifts_no_overlap EXCLUDE USING gist (employee_id WITH =, during WITH &&),
  CONSTRAINT shifts_employee_id_fkey FOREIGN KEY (employee_id) REFERENCES public.employees (id) ON DELETE CASCADE,
  CONSTRAINT shifts_department_id_fkey FOREIGN KEY (department_id) REFERENCES public.departments (id) ON DELETE SET NULL
);

-- [17] CREATE_FUNCTION: audit.log_change()
CREATE OR REPLACE FUNCTION audit.log_change() RETURNS trigger LANGUAGE plpgsql SECURITY DEFINER AS $$
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

-- [18] CREATE_FUNCTION: public.count_active_employees(bigint)
CREATE OR REPLACE FUNCTION public.count_active_employees(p_org_id bigint) RETURNS bigint LANGUAGE sql STABLE AS $$
  SELECT count(*)
  FROM public.employees
  WHERE org_id = p_org_id
    AND status  = 'active'
    AND deleted_at IS NULL;
$$;

-- [19] CREATE_FUNCTION: public.get_department_employees(bigint)
CREATE OR REPLACE FUNCTION public.get_department_employees(p_department_id bigint) RETURNS TABLE (employee_id uuid, full_name text, email text, status employee_status, hire_date date) LANGUAGE sql STABLE AS $$
  SELECT id, full_name, email, status, hire_date
  FROM public.employees
  WHERE department_id = p_department_id
    AND deleted_at    IS NULL
  ORDER BY last_name, first_name;
$$;

-- [20] CREATE_FUNCTION: public.onboard_employee(bigint, email_address, text, text, date, bigint, text, position_level)
CREATE OR REPLACE PROCEDURE public.onboard_employee(p_org_id bigint, p_email email_address, p_first_name text, p_last_name text, p_hire_date date, p_department_id bigint, p_title text, p_level position_level) LANGUAGE plpgsql AS $$
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

-- [21] CREATE_FUNCTION: public.set_updated_at()
CREATE OR REPLACE FUNCTION public.set_updated_at() RETURNS trigger LANGUAGE plpgsql AS $$
BEGIN
  NEW.updated_at = now();
  RETURN NEW;
END;
$$;

-- [22] TOGGLE_RLS: public.departments
ALTER TABLE public.departments ENABLE ROW LEVEL SECURITY;

-- [23] TOGGLE_RLS_NOFORCE: public.departments
ALTER TABLE public.departments NO FORCE ROW LEVEL SECURITY;

-- [24] TOGGLE_RLS: public.employees
ALTER TABLE public.employees ENABLE ROW LEVEL SECURITY;

-- [25] TOGGLE_RLS_NOFORCE: public.employees
ALTER TABLE public.employees NO FORCE ROW LEVEL SECURITY;

-- [26] TOGGLE_RLS: public.leave_requests
ALTER TABLE public.leave_requests ENABLE ROW LEVEL SECURITY;

-- [27] TOGGLE_RLS_NOFORCE: public.leave_requests
ALTER TABLE public.leave_requests NO FORCE ROW LEVEL SECURITY;

-- [28] TOGGLE_RLS: public.shifts
ALTER TABLE public.shifts ENABLE ROW LEVEL SECURITY;

-- [29] TOGGLE_RLS_NOFORCE: public.shifts
ALTER TABLE public.shifts NO FORCE ROW LEVEL SECURITY;

-- [30] CREATE_POLICY: public.departments/departments_org_isolation
CREATE POLICY departments_org_isolation ON public.departments TO public USING (org_id = current_setting('app.org_id', true)::bigint);

-- [31] CREATE_POLICY: public.employees/employees_org_isolation
CREATE POLICY employees_org_isolation ON public.employees TO public USING (org_id = current_setting('app.org_id', true)::bigint);

-- [32] CREATE_POLICY: public.leave_requests/leave_org_isolation
CREATE POLICY leave_org_isolation ON public.leave_requests TO public USING (org_id = current_setting('app.org_id', true)::bigint);

-- [33] CREATE_POLICY: public.shifts/shifts_org_isolation
CREATE POLICY shifts_org_isolation ON public.shifts TO public USING (org_id = current_setting('app.org_id', true)::bigint);

-- [34] CREATE_TRIGGER: public.departments/departments_set_updated_at
CREATE TRIGGER departments_set_updated_at BEFORE UPDATE ON public.departments FOR EACH ROW EXECUTE FUNCTION public.set_updated_at();

-- [35] CREATE_TRIGGER: public.employees/employees_audit
CREATE TRIGGER employees_audit AFTER INSERT OR DELETE OR UPDATE ON public.employees FOR EACH ROW EXECUTE FUNCTION audit.log_change();

-- [36] CREATE_TRIGGER: public.employees/employees_set_updated_at
CREATE TRIGGER employees_set_updated_at BEFORE UPDATE ON public.employees FOR EACH ROW EXECUTE FUNCTION public.set_updated_at();

-- [37] CREATE_TRIGGER: public.leave_requests/leave_requests_set_updated_at
CREATE TRIGGER leave_requests_set_updated_at BEFORE UPDATE ON public.leave_requests FOR EACH ROW EXECUTE FUNCTION public.set_updated_at();

-- [38] CREATE_TRIGGER: public.shifts/shifts_set_updated_at
CREATE TRIGGER shifts_set_updated_at BEFORE UPDATE ON public.shifts FOR EACH ROW EXECUTE FUNCTION public.set_updated_at();

-- [41] CREATE_INDEX: public.idx_attendance_brin
CREATE INDEX IF NOT EXISTS idx_attendance_brin ON public.attendance USING brin (punch_in);

-- [42] CREATE_INDEX: public.idx_attendance_employee
CREATE INDEX IF NOT EXISTS idx_attendance_employee ON public.attendance USING btree (employee_id, punch_in);

-- [67] CREATE_MATERIALIZED_VIEW: public.department_stats
CREATE MATERIALIZED VIEW public.department_stats AS SELECT department_id, department_name, org_id, employee_count, active_count, rank() OVER (PARTITION BY org_id ORDER BY employee_count DESC) AS size_rank FROM (SELECT d.id AS department_id, d.name AS department_name, d.org_id, count(e.id) AS employee_count, count(e.id) FILTER (WHERE e.status = 'active') AS active_count FROM public.departments d LEFT JOIN public.employees e ON e.department_id = d.id AND e.deleted_at IS NULL GROUP BY d.id, d.name, d.org_id) sub;

-- [69] CREATE_VIEW: public.employee_directory
CREATE OR REPLACE VIEW public.employee_directory WITH (security_invoker=true) AS SELECT e.id, e.org_id, e.first_name, e.last_name, e.full_name, e.email, e.phone, e.status, e.hire_date, e.skills, d.name AS department_name, d.code AS department_code FROM public.employees e LEFT JOIN public.departments d ON d.id = e.department_id WHERE e.deleted_at IS NULL;

-- [70] RAW_DDL: raw
CREATE TABLE IF NOT EXISTS public.attendance_2025 PARTITION OF public.attendance FOR VALUES FROM ('2025-01-01') TO ('2026-01-01');

-- [71] RAW_DDL: raw
CREATE TABLE IF NOT EXISTS public.attendance_2026 PARTITION OF public.attendance FOR VALUES FROM ('2026-01-01') TO ('2027-01-01');

-- [72] RAW_DDL: raw
GRANT SELECT ON TABLE public.department_stats TO PUBLIC;

-- [73] RAW_DDL: raw
GRANT SELECT ON TABLE public.employee_directory TO PUBLIC;

COMMIT;

-- The following statements use CONCURRENTLY and run outside the transaction.
-- [39] CREATE_INDEX: audit.idx_change_log_brin
CREATE INDEX CONCURRENTLY IF NOT EXISTS idx_change_log_brin ON audit.change_log USING brin (changed_at);

-- [40] CREATE_INDEX: audit.idx_change_log_table
CREATE INDEX CONCURRENTLY IF NOT EXISTS idx_change_log_table ON audit.change_log USING btree (table_name, changed_at DESC);

-- [43] CREATE_INDEX: public.idx_departments_metadata
CREATE INDEX CONCURRENTLY IF NOT EXISTS idx_departments_metadata ON public.departments USING gin (metadata);

-- [44] CREATE_INDEX: public.idx_departments_org
CREATE INDEX CONCURRENTLY IF NOT EXISTS idx_departments_org ON public.departments USING btree (org_id);

-- [45] CREATE_INDEX: public.idx_departments_parent
CREATE INDEX CONCURRENTLY IF NOT EXISTS idx_departments_parent ON public.departments USING btree (parent_id) WHERE parent_id IS NOT NULL;

-- [46] CREATE_INDEX: public.idx_employees_active
CREATE INDEX CONCURRENTLY IF NOT EXISTS idx_employees_active ON public.employees USING btree (org_id, hire_date) WHERE deleted_at IS NULL;

-- [47] CREATE_INDEX: public.idx_employees_dept
CREATE INDEX CONCURRENTLY IF NOT EXISTS idx_employees_dept ON public.employees USING btree (department_id);

-- [48] CREATE_INDEX: public.idx_employees_email_cover
CREATE INDEX CONCURRENTLY IF NOT EXISTS idx_employees_email_cover ON public.employees USING btree (email_lower) INCLUDE (first_name, last_name, status);

-- [49] CREATE_INDEX: public.idx_employees_metadata
CREATE INDEX CONCURRENTLY IF NOT EXISTS idx_employees_metadata ON public.employees USING gin (metadata);

-- [50] CREATE_INDEX: public.idx_employees_name_trgm
CREATE INDEX CONCURRENTLY IF NOT EXISTS idx_employees_name_trgm ON public.employees USING gin (full_name gin_trgm_ops);

-- [51] CREATE_INDEX: public.idx_employees_org
CREATE INDEX CONCURRENTLY IF NOT EXISTS idx_employees_org ON public.employees USING btree (org_id);

-- [52] CREATE_INDEX: public.idx_employees_phone_unique
CREATE UNIQUE INDEX CONCURRENTLY IF NOT EXISTS idx_employees_phone_unique ON public.employees USING btree (org_id, phone) NULLS NOT DISTINCT;

-- [53] CREATE_INDEX: public.idx_employees_search
CREATE INDEX CONCURRENTLY IF NOT EXISTS idx_employees_search ON public.employees USING gin (search_vector);

-- [54] CREATE_INDEX: public.idx_employees_skills
CREATE INDEX CONCURRENTLY IF NOT EXISTS idx_employees_skills ON public.employees USING gin (skills);

-- [55] CREATE_INDEX: public.idx_employees_status
CREATE INDEX CONCURRENTLY IF NOT EXISTS idx_employees_status ON public.employees USING btree (org_id, status) WHERE deleted_at IS NULL;

-- [56] CREATE_INDEX: public.idx_leave_during
CREATE INDEX CONCURRENTLY IF NOT EXISTS idx_leave_during ON public.leave_requests USING gist (during);

-- [57] CREATE_INDEX: public.idx_leave_employee
CREATE INDEX CONCURRENTLY IF NOT EXISTS idx_leave_employee ON public.leave_requests USING btree (employee_id);

-- [58] CREATE_INDEX: public.idx_leave_pending
CREATE INDEX CONCURRENTLY IF NOT EXISTS idx_leave_pending ON public.leave_requests USING btree (org_id, created_at) WHERE status = 'pending';

-- [59] CREATE_INDEX: public.idx_positions_created_brin
CREATE INDEX CONCURRENTLY IF NOT EXISTS idx_positions_created_brin ON public.positions USING brin (created_at);

-- [60] CREATE_INDEX: public.idx_positions_department
CREATE INDEX CONCURRENTLY IF NOT EXISTS idx_positions_department ON public.positions USING btree (department_id);

-- [61] CREATE_INDEX: public.idx_positions_employee
CREATE INDEX CONCURRENTLY IF NOT EXISTS idx_positions_employee ON public.positions USING btree (employee_id);

-- [62] CREATE_INDEX: public.idx_positions_valid_during
CREATE INDEX CONCURRENTLY IF NOT EXISTS idx_positions_valid_during ON public.positions USING gist (valid_during);

-- [63] CREATE_INDEX: public.idx_shifts_during
CREATE INDEX CONCURRENTLY IF NOT EXISTS idx_shifts_during ON public.shifts USING gist (during);

-- [64] CREATE_INDEX: public.idx_shifts_employee
CREATE INDEX CONCURRENTLY IF NOT EXISTS idx_shifts_employee ON public.shifts USING btree (employee_id);

-- [65] CREATE_INDEX: public.idx_shifts_metadata
CREATE INDEX CONCURRENTLY IF NOT EXISTS idx_shifts_metadata ON public.shifts USING gin (metadata);

-- [66] CREATE_INDEX: public.idx_shifts_org_active
CREATE INDEX CONCURRENTLY IF NOT EXISTS idx_shifts_org_active ON public.shifts USING btree (org_id, status) WHERE status IN ('scheduled', 'confirmed', 'in_progress');

-- [68] CREATE_INDEX: public.idx_department_stats_id
CREATE UNIQUE INDEX CONCURRENTLY IF NOT EXISTS idx_department_stats_id ON public.department_stats USING btree (department_id);

