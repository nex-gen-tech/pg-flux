-- pg-flux generated migration
-- DO NOT EDIT unless you know what you are doing.

-- [1] CREATE_TYPE: raw
DO $pgflux$ BEGIN CREATE DOMAIN public.email_address AS text CHECK (value LIKE '%@%' AND length(value) BETWEEN 3 AND 254); EXCEPTION WHEN duplicate_object THEN NULL; END $pgflux$;

