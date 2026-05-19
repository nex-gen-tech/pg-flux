-- Sequence with non-default AS type + OWNED BY a specific column.

CREATE TABLE public.seq_demo (
  id integer PRIMARY KEY
);

CREATE SEQUENCE public.seq_demo_counter
  AS integer
  INCREMENT BY 1
  MINVALUE 1
  MAXVALUE 2147483647
  START WITH 1
  CACHE 1
  OWNED BY public.seq_demo.id;
