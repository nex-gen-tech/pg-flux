-- UNLOGGED table + WITH reloptions.
-- UNLOGGED skips WAL — faster writes, contents lost on crash. Common for caches.

CREATE UNLOGGED TABLE public.cache_entries (
  key   text   PRIMARY KEY,
  value bytea  NOT NULL,
  ts    timestamptz DEFAULT now()
) WITH (
  fillfactor = 70,
  autovacuum_vacuum_scale_factor = 0.05
);

CREATE TABLE public.hot_writes (
  id   bigint PRIMARY KEY,
  data text   NOT NULL
) WITH (fillfactor = 90);
