-- COMMENT ON exercises every object kind that pg-flux tracks descriptions for.
-- Diffing a change to any IS '...' literal must produce a single COMMENT ON statement,
-- not a DROP+CREATE of the underlying object.

CREATE TABLE public.comments_demo (
  id    bigserial PRIMARY KEY,
  body  text      NOT NULL
);

COMMENT ON TABLE  public.comments_demo IS 'Demonstrates COMMENT ON diffing.';
COMMENT ON COLUMN public.comments_demo.body IS 'User-visible content.';

CREATE INDEX idx_comments_demo_body ON public.comments_demo (body);
COMMENT ON INDEX public.idx_comments_demo_body IS 'Lookup index for body text.';

CREATE SEQUENCE public.comments_demo_seq;
COMMENT ON SEQUENCE public.comments_demo_seq IS 'Counter for comments_demo events.';
