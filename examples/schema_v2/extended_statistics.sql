-- CREATE STATISTICS stress — exercises P5.3.
-- Multivariate stats help the planner when columns are correlated.

CREATE TABLE public.stats_demo (
  region  text NOT NULL,
  country text NOT NULL,
  pop     bigint NOT NULL
);

CREATE STATISTICS public.stats_demo_geo
  (ndistinct, dependencies)
  ON region, country
  FROM public.stats_demo;
