-- pg-flux generated migration
-- DO NOT EDIT unless you know what you are doing.

-- [1] DROP_TRIGGER: public.users/users_set_updated_at
DROP TRIGGER IF EXISTS users_set_updated_at ON public.users CASCADE;

