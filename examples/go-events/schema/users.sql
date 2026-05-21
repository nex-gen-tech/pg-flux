CREATE TABLE public.users (
  id           bigint            GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
  workspace_id bigint            NOT NULL REFERENCES public.workspaces (id) ON DELETE CASCADE,
  email        text              NOT NULL,
  -- @renamed from=username
  display_name text              NOT NULL,
  role         public.user_role  NOT NULL DEFAULT 'member',
  created_at   timestamptz       NOT NULL DEFAULT now(),

  CONSTRAINT users_workspace_email_unique UNIQUE (workspace_id, email),
  CONSTRAINT users_email_format           CHECK  (email LIKE '%@%')
);

CREATE INDEX idx_users_workspace_id ON public.users (workspace_id);
CREATE INDEX idx_users_created_at   ON public.users (created_at DESC);
