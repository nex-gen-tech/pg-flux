-- Iteration 4: column rename (name -> full_name)
CREATE TABLE app_users (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    -- @renamed from=name
    full_name text NOT NULL,
    email text NOT NULL DEFAULT '',
    phone text
);
