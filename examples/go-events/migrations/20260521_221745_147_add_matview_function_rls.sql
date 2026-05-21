-- pg-flux generated migration
-- DO NOT EDIT unless you know what you are doing.
-- pg-flux-baseline-hash: a638c2ad41ddb52357149fed8bc4ac95480a9e2c2582da11d62833ce34179459

-- [ADVISORY COLUMN_REORDER] Column order in public.attendees differs from desired schema; reordering requires table recreation. Desired order (surviving cols): event_id, user_id, status, registered_at
-- [ADVISORY COLUMN_REORDER] Column order in public.events differs from desired schema; reordering requires table recreation. Desired order (surviving cols): id, workspace_id, title, description, status, starts_at, ends_at, location, capacity, metadata, tags, created_at

BEGIN;

-- [1] CREATE_FUNCTION: public.count_confirmed_attendees(bigint)
CREATE OR REPLACE FUNCTION public.count_confirmed_attendees(p_event_id bigint) RETURNS bigint LANGUAGE sql STABLE AS $$
  SELECT count(*) FROM public.attendees
  WHERE event_id = p_event_id AND status = 'confirmed';
$$;

-- [2] TOGGLE_RLS: public.events
ALTER TABLE public.events ENABLE ROW LEVEL SECURITY;

-- [3] TOGGLE_RLS_NOFORCE: public.events
ALTER TABLE public.events NO FORCE ROW LEVEL SECURITY;

-- [4] CREATE_POLICY: public.events/events_workspace_isolation
CREATE POLICY events_workspace_isolation ON public.events TO public USING (workspace_id = current_setting('app.workspace_id', true)::bigint);

-- [5] CREATE_MATERIALIZED_VIEW: public.event_stats
CREATE MATERIALIZED VIEW public.event_stats AS SELECT e.id AS event_id, e.workspace_id, count(a.user_id) FILTER (WHERE a.status = 'confirmed') AS confirmed_count, count(a.user_id) AS total_count FROM public.events e LEFT JOIN public.attendees a ON a.event_id = e.id GROUP BY e.id, e.workspace_id;

COMMIT;

-- The following statements use CONCURRENTLY and run outside the transaction.
-- [6] CREATE_INDEX: public.idx_event_stats_event_id
CREATE UNIQUE INDEX CONCURRENTLY IF NOT EXISTS idx_event_stats_event_id ON public.event_stats USING btree (event_id);

