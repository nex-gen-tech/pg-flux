CREATE TABLE public.order_items (
  id          bigint  GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
  order_id    bigint  NOT NULL,
  order_date  timestamptz NOT NULL,
  product_id  bigint  NOT NULL REFERENCES public.products (id),
  qty         int     NOT NULL CHECK (qty > 0),
  unit_price  public.positive_amount NOT NULL,
  CONSTRAINT order_items_order_fkey FOREIGN KEY (order_id, order_date) REFERENCES public.orders (id, created_at)
);

CREATE INDEX idx_order_items_order   ON public.order_items (order_id);
CREATE INDEX idx_order_items_product ON public.order_items (product_id);
