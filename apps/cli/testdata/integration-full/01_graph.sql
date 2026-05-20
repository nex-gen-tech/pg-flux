-- Full graph: two tables (FK + CHECK), index, SQL + plpgsql functions, view, trigger.
-- Table names a_* / b_* so CREATE TABLE order is stable when sorted by generated DDL.
CREATE TABLE public.a_parents (
    id integer PRIMARY KEY,
    name text NOT NULL
);

CREATE TABLE public.b_children (
    id integer PRIMARY KEY,
    parent_id integer NOT NULL,
    CONSTRAINT chk_b_children_pid CHECK (parent_id > 0),
    CONSTRAINT fk_b_children_parent FOREIGN KEY (parent_id) REFERENCES public.a_parents (id)
);

CREATE INDEX idx_b_children_parent ON public.b_children USING btree (parent_id);

CREATE OR REPLACE FUNCTION public.triple(x integer) RETURNS integer
    LANGUAGE sql
    IMMUTABLE
AS $$ SELECT x * 3 $$;

CREATE OR REPLACE FUNCTION public.announce_child() RETURNS trigger
    LANGUAGE plpgsql
AS $$
BEGIN
  RETURN NEW;
END;
$$;

CREATE VIEW public.v_parents AS
    SELECT id, name FROM public.a_parents;

CREATE TRIGGER trg_b_children_ai
    AFTER INSERT ON public.b_children
    FOR EACH ROW
    EXECUTE FUNCTION public.announce_child();
