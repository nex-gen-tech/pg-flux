-- pg-flux generated migration
-- DO NOT EDIT unless you know what you are doing.

-- [1] ALTER_DEFAULT: 
ALTER TABLE public.users ALTER COLUMN created_at SET DEFAULT now();

-- [2] ALTER_DEFAULT: 
ALTER TABLE public.users ALTER COLUMN updated_at SET DEFAULT now();

