CREATE TABLE IF NOT EXISTS c1_settings (
  connection_id UUID PRIMARY KEY REFERENCES connections(id) ON DELETE CASCADE,
  data JSONB NOT NULL DEFAULT '{}'::jsonb,
  updated_at timestamptz NOT NULL DEFAULT now()
);
