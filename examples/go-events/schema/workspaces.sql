CREATE TABLE public.workspaces (
  id    bigint       GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
  slug  text         NOT NULL,
  name  text         NOT NULL,
  plan  text         NOT NULL DEFAULT 'free',

  CONSTRAINT workspaces_slug_unique UNIQUE (slug),
  CONSTRAINT workspaces_slug_format CHECK (slug ~ '^[a-z0-9-]+$')
);

CREATE INDEX idx_workspaces_slug ON public.workspaces (slug);
