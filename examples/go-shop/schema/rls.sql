ALTER TABLE public.products ENABLE ROW LEVEL SECURITY;
CREATE POLICY products_public_read ON public.products
  FOR SELECT USING (status = 'active');

ALTER TABLE public.orders ENABLE ROW LEVEL SECURITY;
CREATE POLICY orders_owner_only ON public.orders
  FOR ALL USING (customer_id = current_setting('app.customer_id', true)::bigint);
