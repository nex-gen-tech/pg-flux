-- users: core identity record for every account.
CREATE DOMAIN public.email_address AS text
  CHECK (VALUE LIKE '%@%' AND length(VALUE) BETWEEN 3 AND 254)
  CONSTRAINT email_no_spaces CHECK (VALUE NOT LIKE '% %');

CREATE TYPE public.user_status AS ENUM ('active', 'suspended', 'deleted', 'pending_review');

CREATE TYPE public.verification_status AS ENUM ('unverified', 'verified', 'pending');

CREATE TABLE public.users (
  id          bigserial       PRIMARY KEY,
  email       varchar(254)    NOT NULL,
  -- @renamed from=username
  handle      text,
  full_name   varchar(100),
  -- @renamed from=nickname
  screen_name text,
  -- @using CASE is_verified WHEN TRUE THEN 'verified'::public.verification_status ELSE 'unverified'::public.verification_status END
  is_verified public.verification_status NOT NULL DEFAULT 'unverified',
  phone       text               NOT NULL,
  status      public.user_status NOT NULL,
  created_at  timestamptz     NOT NULL DEFAULT now(),
  updated_at  timestamptz     NOT NULL DEFAULT now(),
  -- Issue 8 test: integer column with CHECK constraint (PG adds ::integer cast in catalog)
  test_score  integer         DEFAULT 0,
  -- Issue 5: array default and AT TIME ZONE drift test
  tags        text[]          DEFAULT ARRAY[]::text[],
  utc_signup  timestamptz     DEFAULT (now() AT TIME ZONE 'UTC'),
  -- Issue 2 (generated col): stored generated column — full_name uppercased for search
  search_name text            GENERATED ALWAYS AS (upper(coalesce(full_name, handle))) STORED,

  CONSTRAINT users_email_unique    UNIQUE (email),
  CONSTRAINT users_handle_unique   UNIQUE (handle),
  CONSTRAINT users_email_format    CHECK (email LIKE '%@%'),
  CONSTRAINT users_score_range     CHECK (test_score >= 0),
  CONSTRAINT users_nickname_len    CHECK (char_length(screen_name) <= 50),
  -- Issue 18 test: same constraint name but column was just renamed (full_name was not renamed, just existing constraint)
  CONSTRAINT users_fullname_check  CHECK (full_name IS NULL OR char_length(full_name) > 0)
);

-- Issue 1: char(n) test -- intentionally using char(n) alias
-- Issue 2: decimal test -- intentionally using decimal alias
-- Issue 3: timestamp test -- intentionally using bare timestamp
-- Issue 4: time test -- intentionally using bare time
-- Issue 5: varbit test -- intentionally using varbit alias
-- Issue 6: bare char test -- intentionally using bare char (= char(1))
-- Issue 7: int[] array test -- intentionally using int[] alias

CREATE INDEX idx_users_email        ON public.users (email);
CREATE INDEX idx_users_status       ON public.users (status);
CREATE INDEX idx_users_created      ON public.users (created_at DESC);
CREATE INDEX idx_users_nickname     ON public.users (screen_name) WHERE screen_name IS NOT NULL;
-- Issue 22 test: IF NOT EXISTS should not cause perpetual false diff
CREATE INDEX IF NOT EXISTS idx_users_phone ON public.users (phone);
