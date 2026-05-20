-- pg-flux generated migration
-- DO NOT EDIT unless you know what you are doing.

-- [1] CREATE_INDEX: public.idx_events_kind
CREATE INDEX idx_events_kind ON public.events USING btree (kind);

-- [2] RAW_DDL: raw
CREATE TABLE IF NOT EXISTS public.events_2026 PARTITION OF public.events FOR VALUES FROM ('2026-01-01') TO ('2027-01-01');

-- [3] RAW_DDL: raw
CREATE TABLE IF NOT EXISTS public.events_2027 PARTITION OF public.events FOR VALUES FROM ('2027-01-01') TO ('2028-01-01');

