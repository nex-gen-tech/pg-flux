CREATE TABLE public.price_rules (
  id          bigint      GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
  product_id  bigint      NOT NULL REFERENCES public.products (id) ON DELETE CASCADE,
  price       public.positive_amount NOT NULL,
  valid_during tstzrange  NOT NULL DEFAULT tstzrange(now(), 'infinity'),
  label       text,
  CONSTRAINT price_rules_no_overlap
    EXCLUDE USING gist (product_id WITH =, valid_during WITH &&)
);

CREATE INDEX idx_price_rules_product ON public.price_rules (product_id);
CREATE INDEX idx_price_rules_during  ON public.price_rules USING GIST (valid_during);
