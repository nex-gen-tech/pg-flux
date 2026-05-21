-- pg-flux generated migration
-- DO NOT EDIT unless you know what you are doing.
-- pg-flux-baseline-hash: 0c622b82aa972216ee6fbb6566a901de8729ddd6fdb7c1cbd045a8c4b3b2fd1e

BEGIN;

-- [1] CREATE_TABLE: public.categories
CREATE TABLE IF NOT EXISTS public.categories (
  id smallserial PRIMARY KEY,
  name text NOT NULL,
  color text DEFAULT 'gray' NOT NULL,
  CONSTRAINT categories_name_unique UNIQUE (name)
);

COMMIT;
