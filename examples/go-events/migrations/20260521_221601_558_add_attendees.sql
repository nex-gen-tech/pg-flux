-- pg-flux generated migration
-- DO NOT EDIT unless you know what you are doing.
-- pg-flux-baseline-hash: 41118f64178cb5e999f6d83c7cc514aaa7ef0943b090510d1e815c568da468d0

BEGIN;

-- [1] CREATE_TABLE: public.attendees
CREATE TABLE IF NOT EXISTS public.attendees (
  event_id pg_catalog.int8 NOT NULL,
  user_id pg_catalog.int8 NOT NULL,
  registered_at timestamptz DEFAULT now() NOT NULL,
  CONSTRAINT attendees_event_id_fkey FOREIGN KEY (event_id) REFERENCES public.events (id) ON DELETE CASCADE,
  CONSTRAINT attendees_user_id_fkey FOREIGN KEY (user_id) REFERENCES public.users (id) ON DELETE CASCADE DEFERRABLE INITIALLY DEFERRED,
  PRIMARY KEY (event_id, user_id)
);

COMMIT;

-- The following statements use CONCURRENTLY and run outside the transaction.
-- [2] CREATE_INDEX: public.idx_attendees_event_id
CREATE INDEX CONCURRENTLY IF NOT EXISTS idx_attendees_event_id ON public.attendees USING btree (event_id);

-- [3] CREATE_INDEX: public.idx_attendees_user_id
CREATE INDEX CONCURRENTLY IF NOT EXISTS idx_attendees_user_id ON public.attendees USING btree (user_id);

