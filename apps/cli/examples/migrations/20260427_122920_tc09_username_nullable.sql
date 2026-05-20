-- pg-flux generated migration
-- DO NOT EDIT unless you know what you are doing.

-- [1] ALTER_DEFAULT: 
ALTER TABLE public.users ALTER COLUMN status SET DEFAULT 'suspended';

-- [2] DROP_NOT_NULL: public.users
ALTER TABLE public.users ALTER COLUMN username DROP NOT NULL;

