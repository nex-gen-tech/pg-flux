-- pg-flux generated migration
-- DO NOT EDIT unless you know what you are doing.
-- pg-flux-baseline-hash: e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855

BEGIN;

-- [1] CREATE_EXTENSION: btree_gist
CREATE EXTENSION IF NOT EXISTS btree_gist;

-- [2] CREATE_EXTENSION: pgcrypto
CREATE EXTENSION IF NOT EXISTS pgcrypto;

-- [3] CREATE_TYPE: public.order_status
DO $pgflux$ BEGIN CREATE TYPE public.order_status AS ENUM ('pending', 'confirmed', 'shipped', 'delivered', 'cancelled', 'refunded'); EXCEPTION WHEN duplicate_object THEN NULL; END $pgflux$;

-- [4] CREATE_TYPE: public.product_status
DO $pgflux$ BEGIN CREATE TYPE public.product_status AS ENUM ('draft', 'active', 'archived'); EXCEPTION WHEN duplicate_object THEN NULL; END $pgflux$;

-- [5] CREATE_TYPE: public.customer_tier
DO $pgflux$ BEGIN CREATE TYPE public.customer_tier AS ENUM ('standard', 'silver', 'gold', 'platinum'); EXCEPTION WHEN duplicate_object THEN NULL; END $pgflux$;

-- [6] CREATE_TYPE: public.email_address
DO $pgflux$ BEGIN CREATE DOMAIN public.email_address AS text CONSTRAINT email_format CHECK (value ~ '^[^@]+@[^@]+$'); EXCEPTION WHEN duplicate_object THEN NULL; END $pgflux$;

-- [7] CREATE_TYPE: public.positive_amount
DO $pgflux$ BEGIN CREATE DOMAIN public.positive_amount AS numeric(12, 2) CONSTRAINT positive CHECK (value > 0); EXCEPTION WHEN duplicate_object THEN NULL; END $pgflux$;

-- [8] CREATE_TYPE: public.address
DO $pgflux$ BEGIN CREATE TYPE public.address AS (line1 text, city text, state text, zip text, country text); EXCEPTION WHEN duplicate_object THEN NULL; END $pgflux$;

-- [9] CREATE_TABLE: audit.change_log
CREATE TABLE IF NOT EXISTS audit.change_log (
  id pg_catalog.int8 GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
  table_name text NOT NULL,
  record_id pg_catalog.int8 NOT NULL,
  operation text NOT NULL,
  old_data jsonb,
  new_data jsonb,
  changed_by text,
  changed_at timestamptz DEFAULT now() NOT NULL
);

-- [10] CREATE_TABLE: public.categories
CREATE TABLE IF NOT EXISTS public.categories (
  id bigserial PRIMARY KEY,
  parent_id pg_catalog.int4,
  slug text NOT NULL,
  name text NOT NULL,
  CONSTRAINT categories_slug_unique UNIQUE (slug),
  CONSTRAINT categories_parent_id_fkey FOREIGN KEY (parent_id) REFERENCES public.categories (id) ON DELETE SET NULL
);

-- [11] CREATE_TABLE: public.customers
CREATE TABLE IF NOT EXISTS public.customers (
  id pg_catalog.int8 GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
  email public.email_address NOT NULL,
  full_name text NOT NULL,
  tier public.customer_tier DEFAULT 'standard' NOT NULL,
  phone text,
  shipping_addr public.address,
  metadata jsonb DEFAULT '{}' NOT NULL,
  created_at timestamptz DEFAULT now() NOT NULL,
  updated_at timestamptz DEFAULT now() NOT NULL,
  CONSTRAINT customers_email_unique UNIQUE (email)
);

-- [12] CREATE_TABLE: public.orders
CREATE TABLE IF NOT EXISTS public.orders (
  id pg_catalog.int8 GENERATED ALWAYS AS IDENTITY,
  customer_id pg_catalog.int8 NOT NULL,
  status public.order_status DEFAULT 'pending' NOT NULL,
  total public.positive_amount,
  notes text,
  created_at timestamptz DEFAULT now() NOT NULL,
  updated_at timestamptz DEFAULT now() NOT NULL,
  CONSTRAINT orders_customer_id_fkey FOREIGN KEY (customer_id) REFERENCES public.customers (id),
  PRIMARY KEY (id, created_at)
) PARTITION BY RANGE (created_at);

-- [13] CREATE_TABLE: public.products
CREATE TABLE IF NOT EXISTS public.products (
  id pg_catalog.int8 GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
  category_id pg_catalog.int4,
  sku text NOT NULL,
  name text NOT NULL,
  description text DEFAULT '' NOT NULL,
  status public.product_status DEFAULT 'draft' NOT NULL,
  price public.positive_amount NOT NULL,
  weight_grams pg_catalog.int4,
  tags text[] DEFAULT '{}' NOT NULL,
  attributes jsonb DEFAULT '{}' NOT NULL,
  search_vector tsvector GENERATED ALWAYS AS (to_tsvector('english', (COALESCE(name, '') || ' ') || COALESCE(description, ''))) STORED,
  cost public.positive_amount,
  created_at timestamptz DEFAULT now() NOT NULL,
  updated_at timestamptz DEFAULT now() NOT NULL,
  CONSTRAINT products_sku_unique UNIQUE (sku),
  CONSTRAINT products_category_id_fkey FOREIGN KEY (category_id) REFERENCES public.categories (id) ON DELETE SET NULL
);

-- [14] CREATE_TABLE: public.order_items
CREATE TABLE IF NOT EXISTS public.order_items (
  id pg_catalog.int8 GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
  order_id pg_catalog.int8 NOT NULL,
  order_date timestamptz NOT NULL,
  product_id pg_catalog.int8 NOT NULL,
  qty pg_catalog.int4 NOT NULL,
  unit_price public.positive_amount NOT NULL,
  CONSTRAINT order_items_product_id_fkey FOREIGN KEY (product_id) REFERENCES public.products (id),
  CONSTRAINT order_items_order_fkey FOREIGN KEY (order_id, order_date) REFERENCES public.orders (id, created_at)
);

-- [15] CREATE_TABLE: public.price_rules
CREATE TABLE IF NOT EXISTS public.price_rules (
  id pg_catalog.int8 GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
  product_id pg_catalog.int8 NOT NULL,
  price public.positive_amount NOT NULL,
  valid_during tstzrange DEFAULT tstzrange(now(), 'infinity') NOT NULL,
  label text,
  CONSTRAINT price_rules_no_overlap EXCLUDE USING gist (product_id WITH =, valid_during WITH &&),
  CONSTRAINT price_rules_product_id_fkey FOREIGN KEY (product_id) REFERENCES public.products (id) ON DELETE CASCADE
);

-- [16] CREATE_FUNCTION: audit.log_change()
CREATE OR REPLACE FUNCTION audit.log_change() RETURNS trigger LANGUAGE plpgsql SECURITY DEFINER AS $$
BEGIN
  INSERT INTO audit.change_log (table_name, record_id, operation, old_data, new_data)
  VALUES (
    TG_TABLE_NAME,
    CASE TG_OP WHEN 'DELETE' THEN OLD.id ELSE NEW.id END,
    TG_OP,
    CASE TG_OP WHEN 'INSERT' THEN NULL ELSE row_to_json(OLD)::jsonb END,
    CASE TG_OP WHEN 'DELETE' THEN NULL ELSE row_to_json(NEW)::jsonb END
  );
  RETURN NEW;
END;
$$;

-- [17] CREATE_FUNCTION: public.calculate_order_total(bigint)
CREATE OR REPLACE FUNCTION public.calculate_order_total(p_order_id bigint) RETURNS numeric LANGUAGE sql STABLE SECURITY DEFINER AS $$
  SELECT COALESCE(SUM(qty * unit_price), 0)
  FROM public.order_items
  WHERE order_id = p_order_id;
$$;

-- [18] CREATE_FUNCTION: public.process_order(bigint)
CREATE OR REPLACE PROCEDURE public.process_order(p_order_id bigint) LANGUAGE plpgsql AS $$
BEGIN
  UPDATE public.orders
  SET status = 'confirmed',
      total  = public.calculate_order_total(p_order_id)
  WHERE id = p_order_id AND status = 'pending';
END;
$$;

-- [19] CREATE_FUNCTION: public.set_updated_at()
CREATE OR REPLACE FUNCTION public.set_updated_at() RETURNS trigger LANGUAGE plpgsql AS $$
BEGIN NEW.updated_at = now(); RETURN NEW; END;
$$;

-- [20] TOGGLE_RLS: public.orders
ALTER TABLE public.orders ENABLE ROW LEVEL SECURITY;

-- [21] TOGGLE_RLS_NOFORCE: public.orders
ALTER TABLE public.orders NO FORCE ROW LEVEL SECURITY;

-- [22] TOGGLE_RLS: public.products
ALTER TABLE public.products ENABLE ROW LEVEL SECURITY;

-- [23] TOGGLE_RLS_NOFORCE: public.products
ALTER TABLE public.products NO FORCE ROW LEVEL SECURITY;

-- [24] CREATE_POLICY: public.orders/orders_owner_only
CREATE POLICY orders_owner_only ON public.orders TO public USING (customer_id = current_setting('app.customer_id', true)::bigint);

-- [25] CREATE_POLICY: public.products/products_public_read
CREATE POLICY products_public_read ON public.products FOR SELECT TO public USING (status = 'active');

-- [26] CREATE_TRIGGER: public.customers/customers_set_updated_at
CREATE TRIGGER customers_set_updated_at BEFORE UPDATE ON public.customers FOR EACH ROW EXECUTE FUNCTION public.set_updated_at();

-- [27] CREATE_TRIGGER: public.products/products_audit
CREATE TRIGGER products_audit AFTER INSERT OR DELETE OR UPDATE ON public.products FOR EACH ROW EXECUTE FUNCTION audit.log_change();

-- [28] CREATE_TRIGGER: public.products/products_set_updated_at
CREATE TRIGGER products_set_updated_at BEFORE UPDATE ON public.products FOR EACH ROW EXECUTE FUNCTION public.set_updated_at();

-- [37] CREATE_INDEX: public.idx_orders_created_brin
CREATE INDEX IF NOT EXISTS idx_orders_created_brin ON public.orders USING brin (created_at);

-- [38] CREATE_INDEX: public.idx_orders_customer
CREATE INDEX IF NOT EXISTS idx_orders_customer ON public.orders USING btree (customer_id);

-- [39] CREATE_INDEX: public.idx_orders_status
CREATE INDEX IF NOT EXISTS idx_orders_status ON public.orders USING btree (status, created_at DESC);

-- [50] CREATE_MATERIALIZED_VIEW: public.product_catalog
CREATE MATERIALIZED VIEW public.product_catalog AS SELECT p.id, p.sku, p.name, p.price, p.status, c.name AS category_name FROM public.products p LEFT JOIN public.categories c ON c.id = p.category_id WHERE p.status = 'active';

-- [52] CREATE_VIEW: public.active_products
CREATE OR REPLACE VIEW public.active_products WITH (security_invoker=true) AS SELECT p.id, p.sku, p.name, p.price, p.tags, p.search_vector FROM public.products p WHERE p.status = 'active';

-- [53] RAW_DDL: raw
CREATE TABLE IF NOT EXISTS public.orders_2024 PARTITION OF public.orders FOR VALUES FROM ('2024-01-01') TO ('2025-01-01');

-- [54] RAW_DDL: raw
CREATE TABLE IF NOT EXISTS public.orders_2025 PARTITION OF public.orders FOR VALUES FROM ('2025-01-01') TO ('2026-01-01');

-- [55] RAW_DDL: raw
CREATE TABLE IF NOT EXISTS public.orders_2026 PARTITION OF public.orders FOR VALUES FROM ('2026-01-01') TO ('2027-01-01');

-- [56] RAW_DDL: raw
GRANT SELECT ON TABLE public.active_products TO PUBLIC;

-- [57] RAW_DDL: raw
GRANT SELECT ON TABLE public.product_catalog TO PUBLIC;

-- [58] RAW_DDL: raw
GRANT SELECT ON TABLE public.products TO PUBLIC;

COMMIT;

-- The following statements use CONCURRENTLY and run outside the transaction.
-- [29] CREATE_INDEX: audit.idx_change_log_date
CREATE INDEX CONCURRENTLY IF NOT EXISTS idx_change_log_date ON audit.change_log USING brin (changed_at);

-- [30] CREATE_INDEX: audit.idx_change_log_table
CREATE INDEX CONCURRENTLY IF NOT EXISTS idx_change_log_table ON audit.change_log USING btree (table_name, record_id);

-- [31] CREATE_INDEX: public.idx_customers_created
CREATE INDEX CONCURRENTLY IF NOT EXISTS idx_customers_created ON public.customers USING brin (created_at);

-- [32] CREATE_INDEX: public.idx_customers_email
CREATE INDEX CONCURRENTLY IF NOT EXISTS idx_customers_email ON public.customers USING btree (lower(email::text));

-- [33] CREATE_INDEX: public.idx_customers_metadata
CREATE INDEX CONCURRENTLY IF NOT EXISTS idx_customers_metadata ON public.customers USING gin (metadata);

-- [34] CREATE_INDEX: public.idx_customers_tier
CREATE INDEX CONCURRENTLY IF NOT EXISTS idx_customers_tier ON public.customers USING btree (tier);

-- [35] CREATE_INDEX: public.idx_order_items_order
CREATE INDEX CONCURRENTLY IF NOT EXISTS idx_order_items_order ON public.order_items USING btree (order_id);

-- [36] CREATE_INDEX: public.idx_order_items_product
CREATE INDEX CONCURRENTLY IF NOT EXISTS idx_order_items_product ON public.order_items USING btree (product_id);

-- [40] CREATE_INDEX: public.idx_price_rules_during
CREATE INDEX CONCURRENTLY IF NOT EXISTS idx_price_rules_during ON public.price_rules USING gist (valid_during);

-- [41] CREATE_INDEX: public.idx_price_rules_product
CREATE INDEX CONCURRENTLY IF NOT EXISTS idx_price_rules_product ON public.price_rules USING btree (product_id);

-- [42] CREATE_INDEX: public.idx_products_attributes
CREATE INDEX CONCURRENTLY IF NOT EXISTS idx_products_attributes ON public.products USING gin (attributes);

-- [43] CREATE_INDEX: public.idx_products_category
CREATE INDEX CONCURRENTLY IF NOT EXISTS idx_products_category ON public.products USING btree (category_id);

-- [44] CREATE_INDEX: public.idx_products_price
CREATE INDEX CONCURRENTLY IF NOT EXISTS idx_products_price ON public.products USING btree (price) WHERE status = 'active'::product_status;

-- [45] CREATE_INDEX: public.idx_products_search
CREATE INDEX CONCURRENTLY IF NOT EXISTS idx_products_search ON public.products USING gin (search_vector);

-- [46] CREATE_INDEX: public.idx_products_sku_name
CREATE INDEX CONCURRENTLY IF NOT EXISTS idx_products_sku_name ON public.products USING btree (sku) INCLUDE (name, price);

-- [47] CREATE_INDEX: public.idx_products_sku_nulls_nd
CREATE UNIQUE INDEX CONCURRENTLY IF NOT EXISTS idx_products_sku_nulls_nd ON public.products USING btree (sku) NULLS NOT DISTINCT;

-- [48] CREATE_INDEX: public.idx_products_status
CREATE INDEX CONCURRENTLY IF NOT EXISTS idx_products_status ON public.products USING btree (status) WHERE status = 'active'::product_status;

-- [49] CREATE_INDEX: public.idx_products_tags
CREATE INDEX CONCURRENTLY IF NOT EXISTS idx_products_tags ON public.products USING gin (tags);

-- [51] CREATE_INDEX: public.idx_product_catalog_id
CREATE UNIQUE INDEX CONCURRENTLY IF NOT EXISTS idx_product_catalog_id ON public.product_catalog USING btree (id);

