-- email_address: enforces basic RFC-5322 shape at the DB level.
-- Applications still validate more strictly; the domain is the last line of defence.
CREATE DOMAIN public.email_address AS text
  CONSTRAINT email_format CHECK (VALUE ~ '^[^@\s]+@[^@\s]+\.[^@\s]+$');

-- phone_number: E.164-compatible format (optional leading +, 7-15 digits).
CREATE DOMAIN public.phone_number AS text
  CONSTRAINT phone_format CHECK (VALUE ~ '^\+?[1-9]\d{6,14}$');
