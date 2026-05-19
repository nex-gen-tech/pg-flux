-- Function metadata stress: every cheap ALTER FUNCTION attribute should roundtrip.

CREATE OR REPLACE FUNCTION public.normalize_email(addr text)
  RETURNS text
  LANGUAGE sql
  IMMUTABLE          -- volatility
  LEAKPROOF          -- no logging side-channel
  PARALLEL SAFE      -- safe in parallel queries
  SECURITY INVOKER   -- (default; explicit for diff visibility)
  COST 5             -- planner cost hint
  SET search_path = pg_catalog
AS $$
  SELECT lower(trim(addr));
$$;

CREATE OR REPLACE FUNCTION public.row_count_estimate(tbl regclass)
  RETURNS SETOF integer
  LANGUAGE plpgsql
  STABLE
  PARALLEL RESTRICTED
  ROWS 10
AS $$
BEGIN
  RETURN QUERY EXECUTE format('SELECT count(*) FROM %s', tbl);
END;
$$;
