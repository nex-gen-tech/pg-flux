-- pg-flux generated migration
-- DO NOT EDIT unless you know what you are doing.
-- pg-flux-baseline-hash: b87791922de435b476db3e15e9c57a9782624c936b2f3af07c064f1bb46284d2

-- [ADVISORY COLUMN_REORDER] Column order in public.todos differs from desired schema; reordering requires table recreation. Desired order (surviving cols): id, user_id, category_id, title, body, done, created_at, updated_at

BEGIN;

-- [1] CREATE_TABLE: public.todo_tags
CREATE TABLE IF NOT EXISTS public.todo_tags (
  todo_id pg_catalog.int8 NOT NULL,
  tag_id pg_catalog.int4 NOT NULL,
  CONSTRAINT todo_tags_todo_id_fkey FOREIGN KEY (todo_id) REFERENCES public.todos (id) ON DELETE CASCADE,
  CONSTRAINT todo_tags_tag_id_fkey FOREIGN KEY (tag_id) REFERENCES public.tags (id) ON DELETE CASCADE,
  PRIMARY KEY (todo_id, tag_id)
);

COMMIT;

-- The following statements use CONCURRENTLY and run outside the transaction.
-- [2] CREATE_INDEX: public.idx_todo_tags_tag
CREATE INDEX CONCURRENTLY IF NOT EXISTS idx_todo_tags_tag ON public.todo_tags USING btree (tag_id);

