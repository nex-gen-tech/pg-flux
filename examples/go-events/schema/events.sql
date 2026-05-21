CREATE TABLE public.events (
  id           bigint               GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
  workspace_id bigint               NOT NULL REFERENCES public.workspaces (id) ON DELETE CASCADE,
  title        text                 NOT NULL,
  title_lower  text                 GENERATED ALWAYS AS (lower(title)) STORED,
  description  text                 NOT NULL DEFAULT '',
  status       public.event_status  NOT NULL DEFAULT 'draft',
  starts_at    timestamptz          NOT NULL,
  ends_at      timestamptz          NOT NULL,
  location     text,
  capacity     int,
  metadata     jsonb                NOT NULL DEFAULT '{}',
  tags         text[]               NOT NULL DEFAULT '{}',
  deleted_at   timestamptz,
  created_at   timestamptz          NOT NULL DEFAULT now(),
  updated_at   timestamptz          NOT NULL DEFAULT now(),

  CONSTRAINT events_ends_after_starts CHECK (ends_at > starts_at),
  CONSTRAINT events_capacity_positive CHECK (capacity IS NULL OR capacity > 0)
);

-- Partial unique index: only one active (non-deleted) event per title per workspace
CREATE UNIQUE INDEX idx_events_workspace_title_active ON public.events (workspace_id, title) WHERE deleted_at IS NULL;

CREATE INDEX idx_events_workspace_id ON public.events (workspace_id);
CREATE INDEX idx_events_starts_at    ON public.events (workspace_id, starts_at) WHERE deleted_at IS NULL;
CREATE INDEX idx_events_status       ON public.events (workspace_id, status) WHERE deleted_at IS NULL;
CREATE INDEX idx_events_active       ON public.events (workspace_id, created_at DESC) WHERE deleted_at IS NULL;
CREATE INDEX idx_events_title_lower  ON public.events (title_lower);
CREATE INDEX idx_events_metadata     ON public.events USING GIN (metadata);
CREATE INDEX idx_events_tags         ON public.events USING GIN (tags);
