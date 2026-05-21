CREATE DOMAIN public.email_address AS text
  CONSTRAINT email_format CHECK (VALUE ~ '^[^@]+@[^@]+$');

CREATE DOMAIN public.positive_amount AS numeric(12,2)
  CONSTRAINT positive CHECK (VALUE > 0);
