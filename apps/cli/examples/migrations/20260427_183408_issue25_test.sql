-- pg-flux generated migration
-- DO NOT EDIT unless you know what you are doing.

-- [1] CREATE_FUNCTION: public.is_valid_score(integer, integer)
CREATE OR REPLACE FUNCTION public.is_valid_score(score int, threshold int) RETURNS bool LANGUAGE sql IMMUTABLE AS $$
  SELECT score >= threshold AND score <= 100;
$$;

