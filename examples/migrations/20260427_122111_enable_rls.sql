-- pg-flux generated migration
-- DO NOT EDIT unless you know what you are doing.

-- [1] TOGGLE_RLS: public.posts
ALTER TABLE public.posts ENABLE ROW LEVEL SECURITY;

-- [2] TOGGLE_RLS: public.users
ALTER TABLE public.users ENABLE ROW LEVEL SECURITY;

