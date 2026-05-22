-- ── Enums ────────────────────────────────────────────────────────────────────

CREATE TYPE public.employee_status AS ENUM (
  'active',
  'on_leave',
  'suspended',
  'terminated'
);

CREATE TYPE public.position_level AS ENUM (
  'junior',
  'mid',
  'senior',
  'lead',
  'principal',
  'executive'
);

CREATE TYPE public.leave_type AS ENUM (
  'annual',
  'sick',
  'parental',
  'bereavement',
  'unpaid',
  'other'
);

CREATE TYPE public.shift_status AS ENUM (
  'scheduled',
  'confirmed',
  'in_progress',
  'completed',
  'cancelled'
);

-- ── Composite types ───────────────────────────────────────────────────────────

-- mailing_address is stored as a single composite column on employees.
-- Codegen emits a MailingAddress struct; queries cast via row_to_json / jsonb.
CREATE TYPE public.mailing_address AS (
  line1   text,
  line2   text,
  city    text,
  state   text,
  zip     text,
  country text
);
