ALTER TABLE connections ADD COLUMN IF NOT EXISTS source TEXT NOT NULL DEFAULT '';
-- existing 1C-native connections belong to the 1C integration
UPDATE connections SET source = '1c' WHERE connector_type IN ('1c_odata', '1c_http');
