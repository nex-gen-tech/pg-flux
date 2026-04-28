-- pg-flux generated migration
-- DO NOT EDIT unless you know what you are doing.

-- [1] DROP_SEQUENCE: public.counter_seq
DROP SEQUENCE IF EXISTS public.counter_seq CASCADE;

-- [2] CREATE_SEQUENCE: public.counter_seq
CREATE SEQUENCE public.counter_seq START 10 INCREMENT 5;

