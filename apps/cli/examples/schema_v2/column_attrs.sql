-- Column attribute stress: COLLATE / STORAGE / COMPRESSION.
-- COMPRESSION lz4 requires PG14+ (the project minimum).

CREATE TABLE public.col_attrs_demo (
  id      bigserial PRIMARY KEY,
  -- Explicit COLLATE — picked up via pg_attribute.attcollation
  name    text COLLATE "C" NOT NULL,
  -- STORAGE EXTERNAL: out-of-line TOAST without compression
  payload text   STORAGE EXTERNAL,
  -- COMPRESSION lz4: PG14+ TOAST compression algorithm choice
  notes   text   COMPRESSION lz4
);
