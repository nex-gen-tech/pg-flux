CREATE TABLE public.todos (
  id           bigserial             PRIMARY KEY,
  user_id      bigint                NOT NULL REFERENCES public.users (id) ON DELETE CASCADE,
  category_id  smallint              REFERENCES public.categories (id) ON DELETE SET NULL,
  title        text                  NOT NULL,
  title_lower  text                  GENERATED ALWAYS AS (lower(title)) STORED,
  body         text                  NOT NULL DEFAULT '',
  priority     public.todo_priority  NOT NULL DEFAULT 'normal',
  done         boolean               NOT NULL DEFAULT false,
  deleted_at   timestamptz,
  created_at   timestamptz           NOT NULL DEFAULT now(),
  updated_at   timestamptz           NOT NULL DEFAULT now(),

  CONSTRAINT todos_title_not_blank CHECK (length(trim(title)) > 0),
  CONSTRAINT todos_body_length     CHECK (length(body) <= 4000)
);

CREATE INDEX idx_todos_user_id ON public.todos (user_id);
CREATE INDEX idx_todos_user_done ON public.todos (user_id, done) WHERE done = false;
CREATE INDEX idx_todos_category ON public.todos (category_id) WHERE category_id IS NOT NULL;
CREATE INDEX idx_todos_priority ON public.todos (priority) WHERE priority = ANY (ARRAY['high'::todo_priority, 'urgent'::todo_priority]);
CREATE INDEX idx_todos_active ON public.todos (user_id, created_at DESC) WHERE deleted_at IS NULL;
CREATE INDEX idx_todos_title_lower ON public.todos (title_lower);
