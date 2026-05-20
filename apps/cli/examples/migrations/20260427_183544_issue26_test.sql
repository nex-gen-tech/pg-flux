-- pg-flux generated migration
-- DO NOT EDIT unless you know what you are doing.

-- [1] RAW_DDL: raw
ALTER TYPE public.user_status ADD VALUE IF NOT EXISTS 'pending_review';

