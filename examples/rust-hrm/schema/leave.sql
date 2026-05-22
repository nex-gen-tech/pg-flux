-- leave_requests: employee time-off records.
-- Exercises: daterange, deferrable FK (approved_by can reference a row that
-- gets inserted later in the same transaction during batch imports), and a
-- named CHECK constraint for the status column.
CREATE TABLE public.leave_requests (
  id            bigint GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
  org_id        bigint      NOT NULL,
  employee_id   uuid        NOT NULL REFERENCES public.employees (id) ON DELETE CASCADE,
  leave_type    public.leave_type NOT NULL,
  during        daterange   NOT NULL,
  reason        text,
  -- Approved by can reference the approver; deferrable so the FK check can
  -- happen after both the employee and approver rows are inserted in one txn.
  approved_by   uuid        REFERENCES public.employees (id) ON DELETE SET NULL
                              DEFERRABLE INITIALLY DEFERRED,
  approved_at   timestamptz,
  status        text        NOT NULL DEFAULT 'pending',
  created_at    timestamptz NOT NULL DEFAULT now(),
  updated_at    timestamptz NOT NULL DEFAULT now(),
  CONSTRAINT leave_status_values CHECK (
    status IN ('pending', 'approved', 'rejected', 'cancelled')
  ),
  CONSTRAINT leave_dates_nonempty CHECK (NOT isempty(during))
);

CREATE INDEX idx_leave_employee ON public.leave_requests (employee_id);
CREATE INDEX idx_leave_during   ON public.leave_requests USING GIST (during);
CREATE INDEX idx_leave_pending  ON public.leave_requests (org_id, created_at)
  WHERE status = 'pending';
