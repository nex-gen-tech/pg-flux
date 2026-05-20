-- pg-flux generated migration
-- DO NOT EDIT unless you know what you are doing.

-- [1] DROP_COLUMN: 
ALTER TABLE public.users DROP COLUMN search_name;

-- [2] ADD_COLUMN: public.users.search_name
ALTER TABLE public.users ADD COLUMN search_name text GENERATED ALWAYS AS (upper(COALESCE(full_name, handle))) STORED;

