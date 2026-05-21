-- Materialized view: confirmed attendee counts per event.
-- Refresh with: REFRESH MATERIALIZED VIEW CONCURRENTLY public.event_stats;
CREATE MATERIALIZED VIEW public.event_stats AS
  SELECT
    e.id          AS event_id,
    e.workspace_id,
    count(a.user_id) FILTER (WHERE a.status = 'confirmed') AS confirmed_count,
    count(a.user_id) AS total_count
  FROM public.events e
  LEFT JOIN public.attendees a ON a.event_id = e.id
  GROUP BY e.id, e.workspace_id;

CREATE UNIQUE INDEX idx_event_stats_event_id ON public.event_stats (event_id);
