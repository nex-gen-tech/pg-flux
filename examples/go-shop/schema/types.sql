CREATE TYPE public.address AS (
  line1   text,
  city    text,
  state   text,
  zip     text,
  country text
);

CREATE TYPE public.order_status   AS ENUM ('pending','confirmed','shipped','delivered','cancelled','refunded');
CREATE TYPE public.product_status AS ENUM ('draft','active','archived');
CREATE TYPE public.customer_tier  AS ENUM ('standard','silver','gold','platinum');
