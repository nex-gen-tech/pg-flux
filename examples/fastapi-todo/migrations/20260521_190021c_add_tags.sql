-- pg-flux generated migration
-- DO NOT EDIT unless you know what you are doing.
-- pg-flux-baseline-hash: da6037be42784dce21a7590ae94926320ac8d22365e8ccc0eed94ff0413755ad

-- [ADVISORY COLUMN_REORDER] Column order in public.todos differs from desired schema; reordering requires table recreation. Desired order (surviving cols): id, user_id, category_id, title, body, done, created_at, updated_at

BEGIN;

-- [1] CREATE_TABLE: public.tags
CREATE TABLE IF NOT EXISTS public.tags (
  id serial PRIMARY KEY,
  label text NOT NULL,
  CONSTRAINT tags_label_unique UNIQUE (label)
);

COMMIT;
