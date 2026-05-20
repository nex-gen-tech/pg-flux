-- pg-flux generated migration
-- DO NOT EDIT unless you know what you are doing.

-- [1] ALTER_DEFAULT: 
ALTER TABLE public.users ALTER COLUMN status SET DEFAULT 'suspended';

-- [2] ADD_TABLE_CONSTRAINT: public.posts/posts_user_title_unique
ALTER TABLE public.posts ADD CONSTRAINT posts_user_title_unique UNIQUE (user_id, title);

