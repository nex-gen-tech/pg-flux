CREATE TABLE public.orders (
  id          bigint         GENERATED ALWAYS AS IDENTITY,
  customer_id bigint         NOT NULL REFERENCES public.customers (id),
  status      public.order_status NOT NULL DEFAULT 'pending',
  total       public.positive_amount,
  notes       text,
  created_at  timestamptz    NOT NULL DEFAULT now(),
  updated_at  timestamptz    NOT NULL DEFAULT now(),
  PRIMARY KEY (id, created_at)
) PARTITION BY RANGE (created_at);

CREATE TABLE public.orders_2024 PARTITION OF public.orders
  FOR VALUES FROM ('2024-01-01') TO ('2025-01-01');
CREATE TABLE public.orders_2025 PARTITION OF public.orders
  FOR VALUES FROM ('2025-01-01') TO ('2026-01-01');
CREATE TABLE public.orders_2026 PARTITION OF public.orders
  FOR VALUES FROM ('2026-01-01') TO ('2027-01-01');

CREATE INDEX idx_orders_customer     ON public.orders (customer_id);
CREATE INDEX idx_orders_status       ON public.orders (status, created_at DESC);
CREATE INDEX idx_orders_created_brin ON public.orders USING BRIN (created_at);
