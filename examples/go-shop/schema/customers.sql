CREATE TABLE public.customers (
  id            bigint         GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
  email         public.email_address NOT NULL,
  -- @renamed from=display_name
  full_name     text           NOT NULL,
  tier          public.customer_tier NOT NULL DEFAULT 'standard',
  phone         text,
  shipping_addr public.address,
  metadata      jsonb          NOT NULL DEFAULT '{}',
  created_at    timestamptz    NOT NULL DEFAULT now(),
  updated_at    timestamptz    NOT NULL DEFAULT now(),
  CONSTRAINT customers_email_unique UNIQUE (email)
);

CREATE INDEX idx_customers_email    ON public.customers (lower((email)::text));
CREATE INDEX idx_customers_tier     ON public.customers (tier);
CREATE INDEX idx_customers_created  ON public.customers USING BRIN (created_at);
CREATE INDEX idx_customers_metadata ON public.customers USING GIN (metadata);
