-- pg-flux generated migration
-- DO NOT EDIT unless you know what you are doing.

-- [1] CREATE_INDEX: public.idx_users_phone
CREATE INDEX CONCURRENTLY IF NOT EXISTS idx_users_phone ON public.users USING btree (phone);

