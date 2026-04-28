-- pg-flux generated migration
-- DO NOT EDIT unless you know what you are doing.

-- [1] ADD_COLUMN: public.users.test_score
ALTER TABLE public.users ADD COLUMN test_score pg_catalog.int4 DEFAULT 0;

-- [2] ADD_TABLE_CONSTRAINT: public.users/users_score_range
ALTER TABLE public.users ADD CONSTRAINT users_score_range CHECK (test_score >= 0);

