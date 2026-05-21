CREATE TABLE public.products (
  id            bigint         GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
  category_id   int            REFERENCES public.categories (id) ON DELETE SET NULL,
  sku           text           NOT NULL,
  name          text           NOT NULL,
  description   text           NOT NULL DEFAULT '',
  status        public.product_status NOT NULL DEFAULT 'draft',
  price         public.positive_amount NOT NULL,
  weight_grams  int,
  tags          text[]         NOT NULL DEFAULT '{}',
  attributes    jsonb          NOT NULL DEFAULT '{}',
  search_vector tsvector       GENERATED ALWAYS AS (
                  to_tsvector('english', coalesce(name,'') || ' ' || coalesce(description,''))
                ) STORED,
  cost          public.positive_amount,
  created_at    timestamptz    NOT NULL DEFAULT now(),
  updated_at    timestamptz    NOT NULL DEFAULT now(),
  CONSTRAINT products_sku_unique UNIQUE (sku)

);

CREATE UNIQUE INDEX idx_products_sku_nulls_nd ON public.products (sku) NULLS NOT DISTINCT;
CREATE INDEX idx_products_category     ON public.products (category_id);
CREATE INDEX idx_products_status       ON public.products (status) WHERE status = 'active'::product_status;
CREATE INDEX idx_products_sku_name     ON public.products (sku) INCLUDE (name, price);
CREATE INDEX idx_products_tags         ON public.products USING GIN (tags);
CREATE INDEX idx_products_attributes   ON public.products USING GIN (attributes);
CREATE INDEX idx_products_search       ON public.products USING GIN (search_vector);
CREATE INDEX idx_products_price        ON public.products (price) WHERE status = 'active'::product_status;
