-- Separate schema for audit trail so app tables stay clean.
-- pg-flux tracks both schemas via target_schemas in .pg-flux.yml.
CREATE SCHEMA IF NOT EXISTS audit;
