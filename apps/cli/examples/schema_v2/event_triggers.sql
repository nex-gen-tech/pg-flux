-- EVENT TRIGGER stress — exercises P5.1.
-- Event triggers are SUPERUSER-only on most installs; the stress test skips
-- applying this file if the connected user lacks the privilege.

CREATE OR REPLACE FUNCTION public.audit_ddl()
  RETURNS event_trigger
  LANGUAGE plpgsql
AS $$
BEGIN
  RAISE NOTICE 'DDL: %', tg_event;
END;
$$;

CREATE EVENT TRIGGER pgflux_audit_ddl
  ON ddl_command_end
  WHEN TAG IN ('CREATE TABLE', 'ALTER TABLE', 'DROP TABLE')
  EXECUTE FUNCTION public.audit_ddl();
