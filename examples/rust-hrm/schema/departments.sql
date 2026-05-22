-- Self-referential hierarchy: departments can nest under parent departments.
-- depth tracks how many levels down from the root (root = 0).
CREATE TABLE public.departments (
  id          bigint GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
  org_id      bigint    NOT NULL,
  parent_id   bigint    REFERENCES public.departments (id) ON DELETE SET NULL,
  name        text      NOT NULL,
  code        text      NOT NULL,
  description text,
  depth       smallint  NOT NULL DEFAULT 0,
  metadata    jsonb     NOT NULL DEFAULT '{}',
  created_at  timestamptz NOT NULL DEFAULT now(),
  updated_at  timestamptz NOT NULL DEFAULT now(),
  CONSTRAINT departments_code_org_unique UNIQUE (org_id, code),
  CONSTRAINT departments_depth_check    CHECK (depth >= 0 AND depth <= 10)
);

CREATE INDEX idx_departments_org      ON public.departments (org_id);
CREATE INDEX idx_departments_parent   ON public.departments (parent_id) WHERE parent_id IS NOT NULL;
CREATE INDEX idx_departments_metadata ON public.departments USING GIN (metadata);
