-- pg-flux generated migration
-- DO NOT EDIT unless you know what you are doing.

-- [1] CREATE_POLICY: public.users/users_select
CREATE POLICY users_select ON public.users FOR SELECT USING (is_verified = true OR id = current_setting('app.user_id', true)::bigint);

