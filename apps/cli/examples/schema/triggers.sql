-- triggers: auto-update updated_at on row modifications.

CREATE OR REPLACE FUNCTION public.set_updated_at()
  RETURNS trigger
  LANGUAGE plpgsql
AS $$
BEGIN
  NEW.updated_at := clock_timestamp();
  RETURN NEW;
END;
$$;

-- Issue 25 test: function using type aliases (int, bool) vs catalog canonical (int4, bool)
CREATE OR REPLACE FUNCTION public.is_valid_score(score int, threshold int)
  RETURNS bool
  LANGUAGE sql
  IMMUTABLE
AS $$
  SELECT score >= threshold AND score <= 100;
$$;
