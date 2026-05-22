-- employees: core entity. UUID primary key (gen_random_uuid() is built-in on PG14+).
-- Exercises: UUID PK, domain columns, composite-type column, three generated
-- columns (full_name, email_lower, search_vector), text arrays, JSONB, covering
-- index with INCLUDE, NULLS NOT DISTINCT unique index, GIN trigram index.
CREATE TABLE public.employees (
  id            uuid        PRIMARY KEY DEFAULT gen_random_uuid(),
  org_id        bigint      NOT NULL,
  department_id bigint      REFERENCES public.departments (id) ON DELETE SET NULL,
  email         public.email_address NOT NULL,
  phone         public.phone_number,
  first_name    text        NOT NULL,
  last_name     text        NOT NULL,
  status        public.employee_status NOT NULL DEFAULT 'active',
  hire_date     date        NOT NULL,
  address       public.mailing_address,
  skills        text[]      NOT NULL DEFAULT '{}',
  metadata      jsonb       NOT NULL DEFAULT '{}',
  -- Generated (STORED) columns
  full_name     text        GENERATED ALWAYS AS (first_name || ' ' || last_name) STORED,
  email_lower   text        GENERATED ALWAYS AS (lower(email)) STORED,
  search_vector tsvector    GENERATED ALWAYS AS (
    setweight(to_tsvector('english', coalesce(first_name, '') || ' ' || coalesce(last_name, '')), 'A') ||
    setweight(to_tsvector('english', coalesce(email, '')), 'B')
  ) STORED,
  created_at    timestamptz NOT NULL DEFAULT now(),
  updated_at    timestamptz NOT NULL DEFAULT now(),
  deleted_at    timestamptz,
  CONSTRAINT employees_email_org_unique UNIQUE (org_id, email),
  CONSTRAINT employees_hire_date_check  CHECK (hire_date <= current_date + interval '30 days')
);

-- Standard lookup indexes
CREATE INDEX idx_employees_org    ON public.employees (org_id);
CREATE INDEX idx_employees_dept   ON public.employees (department_id);
CREATE INDEX idx_employees_status ON public.employees (org_id, status) WHERE deleted_at IS NULL;

-- Full-text search
CREATE INDEX idx_employees_search ON public.employees USING GIN (search_vector);

-- JSON and array indexes
CREATE INDEX idx_employees_skills   ON public.employees USING GIN (skills);
CREATE INDEX idx_employees_metadata ON public.employees USING GIN (metadata);

-- Covering index: look up by email_lower, include common projection columns.
-- Avoids heap fetches for the most common employee lookup query.
CREATE INDEX idx_employees_email_cover ON public.employees (email_lower)
  INCLUDE (first_name, last_name, status);

-- Nullable phone: unique per org, but NULLs are not considered duplicates.
-- Requires PG15+ NULLS NOT DISTINCT.
CREATE UNIQUE INDEX idx_employees_phone_unique ON public.employees (org_id, phone)
  NULLS NOT DISTINCT;

-- Partial index for soft-delete pattern: only active (non-deleted) rows.
CREATE INDEX idx_employees_active ON public.employees (org_id, hire_date)
  WHERE deleted_at IS NULL;

-- Trigram GIN index on full_name (needs pg_trgm).
-- Enables fast ILIKE '%john%' or similarity() queries without sequential scans.
CREATE INDEX idx_employees_name_trgm ON public.employees USING GIN (full_name gin_trgm_ops);
