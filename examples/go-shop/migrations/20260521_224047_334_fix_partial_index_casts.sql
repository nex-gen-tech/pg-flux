-- pg-flux generated migration
-- DO NOT EDIT unless you know what you are doing.
-- pg-flux-baseline-hash: 4f54ca17b01f9b0dfc497d35d17916dc22f59fe1d30374af4f32b37d61fc8af5

BEGIN;

-- WORKAROUND B1: pg-flux sees auto-created per-partition FKs
-- (order_items_order_id_order_date_fkey/1/2) as undeclared constraints.
-- These are inherited children of order_items_order_fkey and cannot be
-- dropped directly (PG raises "cannot drop inherited constraint").
-- Removed the three DROP CONSTRAINT statements that pg-flux generated.

-- [4] CREATE_FUNCTION: public.process_order(bigint)
CREATE OR REPLACE PROCEDURE public.process_order(p_order_id bigint) LANGUAGE plpgsql AS $$
BEGIN
  UPDATE public.orders
  SET status = 'confirmed',
      total  = public.calculate_order_total(p_order_id)
  WHERE id = p_order_id AND status = 'pending';
END;
$$;

-- [9] RAW_DDL: raw
CREATE SCHEMA IF NOT EXISTS audit;

COMMIT;

-- The following statements use CONCURRENTLY and run outside the transaction.
-- [5] DROP_INDEX: public.idx_products_price
DROP INDEX CONCURRENTLY IF EXISTS public.idx_products_price;

-- [6] DROP_INDEX: public.idx_products_status
DROP INDEX CONCURRENTLY IF EXISTS public.idx_products_status;

-- [7] CREATE_INDEX: public.idx_products_price
CREATE INDEX CONCURRENTLY IF NOT EXISTS idx_products_price ON public.products USING btree (price) WHERE status = 'active'::public.product_status;

-- [8] CREATE_INDEX: public.idx_products_status
CREATE INDEX CONCURRENTLY IF NOT EXISTS idx_products_status ON public.products USING btree (status) WHERE status = 'active'::public.product_status;

