CREATE TABLE audit.change_log (
  id          bigint      GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
  table_name  text        NOT NULL,
  record_id   bigint      NOT NULL,
  operation   text        NOT NULL CONSTRAINT change_log_operation_check CHECK (operation IN ('INSERT','UPDATE','DELETE')),
  old_data    jsonb,
  new_data    jsonb,
  changed_by  text,
  changed_at  timestamptz NOT NULL DEFAULT now()
);

CREATE INDEX idx_change_log_table   ON audit.change_log (table_name, record_id);
CREATE INDEX idx_change_log_date    ON audit.change_log USING BRIN (changed_at);

CREATE OR REPLACE FUNCTION audit.log_change()
RETURNS trigger
LANGUAGE plpgsql
SECURITY DEFINER
AS $$
BEGIN
  INSERT INTO audit.change_log (table_name, record_id, operation, old_data, new_data)
  VALUES (
    TG_TABLE_NAME,
    CASE TG_OP WHEN 'DELETE' THEN OLD.id ELSE NEW.id END,
    TG_OP,
    CASE TG_OP WHEN 'INSERT' THEN NULL ELSE row_to_json(OLD)::jsonb END,
    CASE TG_OP WHEN 'DELETE' THEN NULL ELSE row_to_json(NEW)::jsonb END
  );
  RETURN NEW;
END;
$$;
