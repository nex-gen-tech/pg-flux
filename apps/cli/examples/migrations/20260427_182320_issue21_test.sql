-- pg-flux generated migration
-- DO NOT EDIT unless you know what you are doing.

-- [1] CREATE_TABLE: public.comments
CREATE TABLE public.comments (
  id bigserial PRIMARY KEY,
  post_id pg_catalog.int8 NOT NULL,
  user_id pg_catalog.int8,
  body text NOT NULL,
  CONSTRAINT comments_post_id_fkey FOREIGN KEY (post_id) REFERENCES public.posts (id) ON DELETE CASCADE,
  CONSTRAINT comments_user_id_fkey FOREIGN KEY (user_id) REFERENCES public.users (id) ON DELETE SET NULL
);

