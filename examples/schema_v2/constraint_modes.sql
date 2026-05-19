-- DEFERRABLE constraints + FK MATCH variations.
-- Useful for circular FKs (swap two values without dropping the constraint).

CREATE TABLE public.cm_parent (
  id   bigint PRIMARY KEY,
  name text NOT NULL
);

CREATE TABLE public.cm_child (
  id        bigint PRIMARY KEY,
  parent_id bigint,
  -- DEFERRABLE INITIALLY DEFERRED: check happens at COMMIT, not on each UPDATE
  CONSTRAINT cm_child_parent_fk
    FOREIGN KEY (parent_id) REFERENCES public.cm_parent (id)
    DEFERRABLE INITIALLY DEFERRED,
  -- MATCH FULL: composite NULL is ALL-or-NONE
  -- (illustrative — would need a composite FK to exercise fully)
  CONSTRAINT cm_child_name_unique UNIQUE (id) DEFERRABLE INITIALLY IMMEDIATE
);
