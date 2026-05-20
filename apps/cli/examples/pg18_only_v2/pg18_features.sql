-- PG18+ features.
-- Below PG18 the differ fails loud naming the feature and required version.

CREATE TABLE public.pg18_demo (
  id    bigserial PRIMARY KEY,
  first text NOT NULL,
  last  text NOT NULL,
  -- VIRTUAL generated column: computed at read time, not stored.
  full_name text GENERATED ALWAYS AS (first || ' ' || last) VIRTUAL
);

-- NOT ENFORCED: the constraint is recorded but PostgreSQL does not validate
-- inserts/updates against it. Useful for staging environments mirroring
-- production constraints without paying the runtime cost.
CREATE TABLE public.pg18_notenforced_demo (
  id     bigserial PRIMARY KEY,
  score  integer NOT NULL,
  CONSTRAINT pg18_score_range
    CHECK (score >= 0 AND score <= 100) NOT ENFORCED
);
