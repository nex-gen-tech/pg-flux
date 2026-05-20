-- pg-flux generated migration
-- DO NOT EDIT unless you know what you are doing.

-- [1] DROP_POLICY: public.users/users_select
DROP POLICY IF EXISTS users_select ON public.users;

-- [2] CREATE_POLICY: public.users/users_select
CREATE POLICY users_select ON public.users FOR SELECT TO public USING (true);

