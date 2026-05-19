-- GRANT/REVOKE stress — exercises P2.1 structured privilege diffing.
-- Note: roles must exist on the target server (or the apply will fail).
-- For the stress test, create the roles first or use the dummy fallback.

CREATE TABLE public.grants_demo (
  id    bigserial PRIMARY KEY,
  body  text      NOT NULL
);

-- Grant SELECT to PUBLIC (everyone).
GRANT SELECT ON TABLE public.grants_demo TO PUBLIC;

-- Grant INSERT, UPDATE to an application role (assumes role exists).
-- Replace with `CREATE ROLE app_writer NOLOGIN;` in your bootstrap if needed.
GRANT INSERT, UPDATE ON TABLE public.grants_demo TO app_writer;
