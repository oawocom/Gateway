CREATE TABLE IF NOT EXISTS c1_meta (
  connection_id UUID PRIMARY KEY REFERENCES connections(id) ON DELETE CASCADE,
  platform TEXT NOT NULL DEFAULT '',
  config_name TEXT NOT NULL DEFAULT '',
  config_version TEXT NOT NULL DEFAULT '',
  profile JSONB NOT NULL,
  loaded_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
