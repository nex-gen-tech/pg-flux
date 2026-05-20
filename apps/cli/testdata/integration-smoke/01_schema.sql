-- E2E smoke: table + index + function (no rename hints — post-apply diff must match).
CREATE TABLE items (
    id integer PRIMARY KEY,
    n text NOT NULL
);
CREATE INDEX idx_items_n ON public.items USING btree (n);
CREATE OR REPLACE FUNCTION public.double_it(x int) RETURNS int
    LANGUAGE sql
    IMMUTABLE
AS $$ SELECT x * 2 $$;
