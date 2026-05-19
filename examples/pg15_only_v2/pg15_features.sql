-- PG15+ feature: NULLS NOT DISTINCT on UNIQUE constraints.
-- Without this clause, multiple NULL values are considered distinct and pass UNIQUE.
-- With it, only one NULL is permitted.

CREATE TABLE public.pg15_email_uniq (
  id    bigserial PRIMARY KEY,
  -- One row with NULL email is OK; a second one would violate the constraint.
  email text,
  CONSTRAINT pg15_email_uniq_email_uq UNIQUE NULLS NOT DISTINCT (email)
);
