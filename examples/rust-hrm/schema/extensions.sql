-- btree_gist: GIST support for btree types (uuid, text, etc.)
-- Required for EXCLUDE constraints that mix UUID equality with range overlap.
CREATE EXTENSION IF NOT EXISTS btree_gist;

-- pg_trgm: trigram similarity support.
-- Enables GIN trigram indexes for fast fuzzy / LIKE / ILIKE searches on name fields.
CREATE EXTENSION IF NOT EXISTS pg_trgm;
