CREATE TABLE public.users (
    id          bigint      GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    handle      text        NOT NULL UNIQUE,
    email       text        NOT NULL UNIQUE,
    bio         text,
    created_at  timestamptz NOT NULL DEFAULT now(),
    updated_at  timestamptz NOT NULL DEFAULT now()
);
