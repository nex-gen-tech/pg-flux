-- pg-flux generated migration
-- DO NOT EDIT unless you know what you are doing.

-- [1] CREATE_TYPE: raw
DO $pgflux$ BEGIN CREATE TYPE public.visibility_level AS ENUM ('public', 'private', 'friends_only'); EXCEPTION WHEN duplicate_object THEN NULL; END $pgflux$;

-- [2] ADD_COLUMN: public.posts.visibility
ALTER TABLE public.posts ADD COLUMN visibility public.visibility_level NOT NULL DEFAULT 'public';

