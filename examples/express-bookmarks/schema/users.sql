CREATE TABLE public.users (
  id          uuid         PRIMARY KEY DEFAULT gen_random_uuid(),
  email       text         NOT NULL,
  -- @renamed from=username
  handle          text         NOT NULL,
  email_verified  boolean      NOT NULL DEFAULT false,
  created_at      timestamptz  NOT NULL DEFAULT now(),

  CONSTRAINT users_email_unique    UNIQUE (email),
  CONSTRAINT users_username_unique UNIQUE (handle),
  CONSTRAINT users_email_format    CHECK  (email LIKE '%@%')
);

CREATE INDEX idx_users_created_at ON public.users (created_at DESC);
