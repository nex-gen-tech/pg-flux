-- pg-flux generated migration
-- DO NOT EDIT unless you know what you are doing.

-- [1] ALTER_DEFAULT: 
ALTER TABLE public.posts ALTER COLUMN body SET DEFAULT '';

-- [2] SET_NOT_NULL: public.posts
ALTER TABLE public.posts ALTER COLUMN body SET NOT NULL;

