-- pg-flux generated migration
-- DO NOT EDIT unless you know what you are doing.

-- [1] CREATE_TABLE: public.audit_log
CREATE TABLE public.audit_log (
  id bigserial PRIMARY KEY,
  user_id pg_catalog.int8,
  action text NOT NULL,
  created_at timestamptz DEFAULT now() NOT NULL
);

-- [2] TOGGLE_RLS: public.audit_log
ALTER TABLE public.audit_log ENABLE ROW LEVEL SECURITY;

-- [3] TOGGLE_RLS_FORCE: public.audit_log
ALTER TABLE public.audit_log FORCE ROW LEVEL SECURITY;

