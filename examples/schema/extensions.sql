-- extensions: pg_trgm for full-text similarity search.
CREATE EXTENSION IF NOT EXISTS pg_trgm;

-- Issue 23 test: sequence that will need alter when params change.
CREATE SEQUENCE public.counter_seq START 10 INCREMENT 10 CACHE 2;
