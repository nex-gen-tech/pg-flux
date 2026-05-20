-- pg-flux generated migration
-- DO NOT EDIT unless you know what you are doing.

-- [1] DROP_TABLE_CONSTRAINT: public.users/users_score_range
ALTER TABLE public.users DROP CONSTRAINT IF EXISTS users_score_range;

-- [2] DROP_COLUMN: 
ALTER TABLE public.users DROP COLUMN test_bits;

-- [3] DROP_COLUMN: 
ALTER TABLE public.users DROP COLUMN test_char;

-- [4] DROP_COLUMN: 
ALTER TABLE public.users DROP COLUMN test_char1;

-- [5] DROP_COLUMN: 
ALTER TABLE public.users DROP COLUMN test_decimal;

-- [6] DROP_COLUMN: 
ALTER TABLE public.users DROP COLUMN test_score;

-- [7] DROP_COLUMN: 
ALTER TABLE public.users DROP COLUMN test_tags;

-- [8] DROP_COLUMN: 
ALTER TABLE public.users DROP COLUMN test_time;

-- [9] DROP_COLUMN: 
ALTER TABLE public.users DROP COLUMN test_ts;

