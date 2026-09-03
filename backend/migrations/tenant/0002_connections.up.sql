CREATE TABLE connections (
  id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  name TEXT NOT NULL,
  connector_type TEXT NOT NULL,
  config_enc TEXT NOT NULL,
  status TEXT NOT NULL DEFAULT 'active',
  last_sync_at TIMESTAMPTZ,
  last_error TEXT,
  created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE synced_entities (
  id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  connection_id UUID NOT NULL REFERENCES connections(id) ON DELETE CASCADE,
  entity_name TEXT NOT NULL,
  record_count BIGINT NOT NULL DEFAULT 0,
  last_synced_at TIMESTAMPTZ,
  UNIQUE (connection_id, entity_name)
);

CREATE TABLE records (
  id BIGSERIAL PRIMARY KEY,
  connection_id UUID NOT NULL REFERENCES connections(id) ON DELETE CASCADE,
  entity_name TEXT NOT NULL,
  external_id TEXT NOT NULL,
  data JSONB NOT NULL,
  synced_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  UNIQUE (connection_id, entity_name, external_id)
);
CREATE INDEX records_conn_entity_idx ON records(connection_id, entity_name);

CREATE TABLE automations (
  id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  name TEXT NOT NULL,
  connection_id UUID NOT NULL REFERENCES connections(id) ON DELETE CASCADE,
  entity_name TEXT NOT NULL,
  interval_minutes INT NOT NULL CHECK (interval_minutes >= 5),
  enabled BOOLEAN NOT NULL DEFAULT true,
  last_run_at TIMESTAMPTZ,
  last_status TEXT,
  created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE sync_log (
  id BIGSERIAL PRIMARY KEY,
  connection_id UUID,
  entity_name TEXT,
  status TEXT NOT NULL,
  message TEXT,
  records_synced BIGINT DEFAULT 0,
  created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
