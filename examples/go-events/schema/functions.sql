CREATE OR REPLACE FUNCTION public.count_confirmed_attendees(p_event_id bigint)
RETURNS bigint
LANGUAGE sql
STABLE
AS $$
  SELECT count(*) FROM public.attendees
  WHERE event_id = p_event_id AND status = 'confirmed';
$$;
