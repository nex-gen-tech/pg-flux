CREATE TABLE public.attendees (
  event_id      bigint                 NOT NULL REFERENCES public.events (id) ON DELETE CASCADE,
  user_id       bigint                 NOT NULL,
  status        public.attendee_status NOT NULL DEFAULT 'invited',
  registered_at timestamptz            NOT NULL DEFAULT now(),

  CONSTRAINT attendees_pkey PRIMARY KEY (event_id, user_id),
  CONSTRAINT attendees_user_id_fkey FOREIGN KEY (user_id) REFERENCES public.users (id) ON DELETE CASCADE DEFERRABLE INITIALLY DEFERRED
);

CREATE INDEX idx_attendees_event_id ON public.attendees (event_id);
CREATE INDEX idx_attendees_user_id  ON public.attendees (user_id);
CREATE INDEX idx_attendees_status   ON public.attendees (event_id, status);
