-- pg-flux generated migration
-- DO NOT EDIT unless you know what you are doing.

-- [1] ADD_COLUMN: public.users.test_bits
ALTER TABLE public.users ADD COLUMN test_bits varbit(8);

-- [2] ADD_COLUMN: public.users.test_char
ALTER TABLE public.users ADD COLUMN test_char pg_catalog.bpchar(5);

-- [3] ADD_COLUMN: public.users.test_char1
ALTER TABLE public.users ADD COLUMN test_char1 pg_catalog.bpchar(1);

-- [4] ADD_COLUMN: public.users.test_decimal
ALTER TABLE public.users ADD COLUMN test_decimal pg_catalog.numeric(10, 2);

-- [5] ADD_COLUMN: public.users.test_score
ALTER TABLE public.users ADD COLUMN test_score pg_catalog.int4 DEFAULT 0;

-- [6] ADD_COLUMN: public.users.test_tags
ALTER TABLE public.users ADD COLUMN test_tags pg_catalog.int4[];

-- [7] ADD_COLUMN: public.users.test_time
ALTER TABLE public.users ADD COLUMN test_time pg_catalog.time;

-- [8] ADD_COLUMN: public.users.test_ts
ALTER TABLE public.users ADD COLUMN test_ts pg_catalog.timestamp;

-- [9] ADD_TABLE_CONSTRAINT: public.users/users_score_range
ALTER TABLE public.users ADD CONSTRAINT users_score_range CHECK (test_score >= 0);

