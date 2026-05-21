-- pg-flux generated migration
-- DO NOT EDIT unless you know what you are doing.
-- pg-flux-baseline-hash: 4f54ca17b01f9b0dfc497d35d17916dc22f59fe1d30374af4f32b37d61fc8af5

BEGIN;

-- [1] RENAME_COLUMN: public.customers.full_name
ALTER TABLE public.customers RENAME COLUMN display_name TO full_name;

-- [2] ADD_COLUMN: public.customers.phone
ALTER TABLE public.customers ADD COLUMN IF NOT EXISTS phone text;

-- [3] ADD_COLUMN: public.products.cost
ALTER TABLE public.products ADD COLUMN IF NOT EXISTS cost public.positive_amount;

-- WORKAROUND B1: same partition-FK ghost issue — removed three DROP CONSTRAINT statements.
-- WORKAROUND B2: process_order re-emitted on every migrate generate — removed (already applied).
-- [7] CREATE_FUNCTION: public.process_order(bigint)
CREATE OR REPLACE PROCEDURE public.process_order(p_order_id bigint) LANGUAGE plpgsql AS $$
BEGIN
  UPDATE public.orders
  SET status = 'confirmed',
      total  = public.calculate_order_total(p_order_id)
  WHERE id = p_order_id AND status = 'pending';
END;
$$;

-- [9] RAW_DDL: raw
ALTER TYPE public.order_status ADD VALUE IF NOT EXISTS 'refunded' AFTER 'cancelled';

-- [10] RAW_DDL: raw
CREATE SCHEMA IF NOT EXISTS audit;

COMMIT;

-- The following statements use CONCURRENTLY and run outside the transaction.
-- [8] CREATE_INDEX: public.idx_products_sku_nulls_nd
CREATE UNIQUE INDEX CONCURRENTLY IF NOT EXISTS idx_products_sku_nulls_nd ON public.products USING btree (sku) NULLS NOT DISTINCT;

