-- View WITH options.
-- security_invoker requires PG15+ — the differ will fail loud below that.

CREATE TABLE public.priv_data (
  id    bigserial PRIMARY KEY,
  owner text      NOT NULL,
  body  text      NOT NULL
);

CREATE VIEW public.priv_view
  WITH (security_barrier = true, security_invoker = true)
  AS SELECT * FROM public.priv_data;

-- check_option: rows inserted/updated through the view must satisfy the WHERE.
CREATE VIEW public.recent_priv
  WITH (check_option = local)
  AS SELECT * FROM public.priv_data WHERE id > 0;
