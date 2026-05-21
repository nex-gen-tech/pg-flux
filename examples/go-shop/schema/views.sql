CREATE OR REPLACE VIEW public.active_products
  WITH (security_invoker = true)
AS
  SELECT p.id, p.sku, p.name, p.price, p.tags, p.search_vector
  FROM public.products p
  WHERE p.status = 'active';

CREATE MATERIALIZED VIEW public.product_catalog AS
  SELECT p.id, p.sku, p.name, p.price, p.status,
         c.name AS category_name
  FROM public.products p
  LEFT JOIN public.categories c ON c.id = p.category_id
  WHERE p.status = 'active';

CREATE UNIQUE INDEX idx_product_catalog_id ON public.product_catalog (id);
