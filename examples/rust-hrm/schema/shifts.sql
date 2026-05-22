-- shifts: scheduled work periods for employees.
-- Exercises: tstzrange type, EXCLUDE constraint (no overlapping shifts per
-- employee), GIST index on the range, partial index on active statuses.
CREATE TABLE public.shifts (
  id            bigint GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
  org_id        bigint      NOT NULL,
  employee_id   uuid        NOT NULL REFERENCES public.employees (id) ON DELETE CASCADE,
  department_id bigint      REFERENCES public.departments (id) ON DELETE SET NULL,
  title         text        NOT NULL,
  status        public.shift_status NOT NULL DEFAULT 'scheduled',
  during        tstzrange   NOT NULL,
  notes         text,
  metadata      jsonb       NOT NULL DEFAULT '{}',
  created_at    timestamptz NOT NULL DEFAULT now(),
  updated_at    timestamptz NOT NULL DEFAULT now(),
  CONSTRAINT shifts_title_nonempty CHECK (length(trim(title)) > 0),
  -- An employee cannot have two overlapping shifts.
  CONSTRAINT shifts_no_overlap EXCLUDE USING gist (
    employee_id WITH =,
    during      WITH &&
  )
);

CREATE INDEX idx_shifts_employee  ON public.shifts (employee_id);
CREATE INDEX idx_shifts_during    ON public.shifts USING GIST (during);
CREATE INDEX idx_shifts_org_active ON public.shifts (org_id, status)
  WHERE status IN ('scheduled', 'confirmed', 'in_progress');
CREATE INDEX idx_shifts_metadata  ON public.shifts USING GIN (metadata);
