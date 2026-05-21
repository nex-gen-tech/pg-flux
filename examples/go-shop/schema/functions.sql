CREATE OR REPLACE FUNCTION public.calculate_order_total(p_order_id bigint)
RETURNS numeric
LANGUAGE sql
STABLE
SECURITY DEFINER
AS $$
  SELECT COALESCE(SUM(qty * unit_price), 0)
  FROM public.order_items
  WHERE order_id = p_order_id;
$$;

CREATE OR REPLACE PROCEDURE public.process_order(p_order_id bigint)
LANGUAGE plpgsql
AS $$
BEGIN
  UPDATE public.orders
  SET status = 'confirmed',
      total  = public.calculate_order_total(p_order_id)
  WHERE id = p_order_id AND status = 'pending';
END;
$$;
