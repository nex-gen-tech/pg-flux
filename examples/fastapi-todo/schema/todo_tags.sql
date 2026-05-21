CREATE TABLE public.todo_tags (
  todo_id bigint NOT NULL REFERENCES public.todos (id) ON DELETE CASCADE,
  tag_id  int    NOT NULL REFERENCES public.tags  (id) ON DELETE CASCADE,
  PRIMARY KEY (todo_id, tag_id)
);

CREATE INDEX idx_todo_tags_tag ON public.todo_tags (tag_id);
