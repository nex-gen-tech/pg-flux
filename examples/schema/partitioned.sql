-- partitioned: partitioned table test.
-- Issue 28 (new): partitioned tables should be parsed and their partitions tracked.

CREATE TABLE public.events (
  id         bigserial     NOT NULL,
  ts         timestamptz   NOT NULL,
  kind       text          NOT NULL,
  payload    jsonb,
  PRIMARY KEY (id, ts)
) PARTITION BY RANGE (ts);

CREATE TABLE public.events_2026 PARTITION OF public.events
  FOR VALUES FROM ('2026-01-01') TO ('2027-01-01');

CREATE TABLE public.events_2027 PARTITION OF public.events
  FOR VALUES FROM ('2027-01-01') TO ('2028-01-01');

CREATE INDEX idx_events_kind ON public.events (kind);
