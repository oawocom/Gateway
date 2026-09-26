package syncer

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"gateway/internal/connectors/http1c"
	"gateway/internal/connectors/odata1c"
	"gateway/internal/connectors/pashabank"
	"gateway/internal/connectors/sqldb"
	"gateway/internal/connectors/zoho"
	"gateway/internal/crypto"
)

const pageSize = 500
const maxRecordsPerSync = 50000

// Syncer pulls records from an external system into a tenant database.
type Syncer struct {
	Box *crypto.Box
}

// LoadConfig decrypts a connection's config into the given struct.
func (s *Syncer) LoadConfig(configEnc string, out any) error {
	plain, err := s.Box.Decrypt(configEnc)
	if err != nil {
		return fmt.Errorf("config decrypt: %w", err)
	}
	return json.Unmarshal(plain, out)
}

// SyncEntity syncs one entity of a 1C connection into the tenant DB.
// Returns the number of records upserted.
func (s *Syncer) SyncEntity(ctx context.Context, tdb *pgxpool.Pool, connectionID, connectorType, configEnc, entity string) (int64, error) {
	var total int64
	var syncErr error

	switch connectorType {
	case "1c_odata":
		var cfg odata1c.Config
		if err := s.LoadConfig(configEnc, &cfg); err != nil {
			syncErr = err
			break
		}
		client := odata1c.New(cfg)
		skip := 0
		for {
			page, err := client.FetchPage(entity, pageSize, skip)
			if err != nil {
				syncErr = err
				break
			}
			for i, rec := range page {
				data, err := json.Marshal(rec)
				if err != nil {
					continue
				}
				extID := odata1c.ExternalID(rec, fmt.Sprintf("row_%d", skip+i))
				if _, err := tdb.Exec(ctx, `
					INSERT INTO records (connection_id, entity_name, external_id, data, synced_at)
					VALUES ($1,$2,$3,$4,now())
					ON CONFLICT (connection_id, entity_name, external_id)
					DO UPDATE SET data=EXCLUDED.data, synced_at=now()`,
					connectionID, entity, extID, data); err != nil {
					syncErr = err
					break
				}
				total++
			}
			if syncErr != nil || len(page) < pageSize || total >= maxRecordsPerSync {
				break
			}
			skip += pageSize
		}
	case "1c_http":
		var cfg http1c.Config
		if err := s.LoadConfig(configEnc, &cfg); err != nil {
			syncErr = err
			break
		}
		recs, err := http1c.New(cfg).Fetch(entity)
		if err != nil {
			syncErr = err
			break
		}
		for i, rec := range recs {
			if total >= maxRecordsPerSync {
				break
			}
			data, err := json.Marshal(rec)
			if err != nil {
				continue
			}
			extID := odata1c.ExternalID(rec, fmt.Sprintf("row_%d", i))
			if _, err := tdb.Exec(ctx, `
				INSERT INTO records (connection_id, entity_name, external_id, data, synced_at)
				VALUES ($1,$2,$3,$4,now())
				ON CONFLICT (connection_id, entity_name, external_id)
				DO UPDATE SET data=EXCLUDED.data, synced_at=now()`,
				connectionID, entity, extID, data); err != nil {
				syncErr = err
				break
			}
			total++
		}
	case "pasha_bank":
		var cfg pashabank.Config
		if err := s.LoadConfig(configEnc, &cfg); err != nil {
			syncErr = err
			break
		}
		client := pashabank.New(cfg)
		page := 0
		for {
			recs, more, err := client.FetchPage(entity, page, 200)
			if err != nil {
				syncErr = err
				break
			}
			for i, rec := range recs {
				data, err := json.Marshal(rec)
				if err != nil {
					continue
				}
				extID := pashabank.ExternalID(rec, fmt.Sprintf("row_%d_%d", page, i))
				if _, err := tdb.Exec(ctx, `
					INSERT INTO records (connection_id, entity_name, external_id, data, synced_at)
					VALUES ($1,$2,$3,$4,now())
					ON CONFLICT (connection_id, entity_name, external_id)
					DO UPDATE SET data=EXCLUDED.data, synced_at=now()`,
					connectionID, entity, extID, data); err != nil {
					syncErr = err
					break
				}
				total++
			}
			if syncErr != nil || !more || total >= maxRecordsPerSync {
				break
			}
			page++
		}
	case "postgres", "mysql", "mssql":
		var cfg sqldb.Config
		if err := s.LoadConfig(configEnc, &cfg); err != nil {
			syncErr = err
			break
		}
		client := sqldb.New(connectorType, cfg)
		skip := 0
		for {
			page, ids, err := client.FetchPage(entity, pageSize, skip)
			if err != nil {
				syncErr = err
				break
			}
			for i, rec := range page {
				data, err := json.Marshal(rec)
				if err != nil {
					continue
				}
				if _, err := tdb.Exec(ctx, `
					INSERT INTO records (connection_id, entity_name, external_id, data, synced_at)
					VALUES ($1,$2,$3,$4,now())
					ON CONFLICT (connection_id, entity_name, external_id)
					DO UPDATE SET data=EXCLUDED.data, synced_at=now()`,
					connectionID, entity, ids[i], data); err != nil {
					syncErr = err
					break
				}
				total++
			}
			if syncErr != nil || len(page) < pageSize || total >= maxRecordsPerSync {
				break
			}
			skip += pageSize
		}
	case "zoho_crm":
		var cfg zoho.Config
		if err := s.LoadConfig(configEnc, &cfg); err != nil {
			syncErr = err
			break
		}
		client := zoho.New(cfg)
		page := 1
		for {
			recs, more, err := client.FetchPage(entity, page)
			if err != nil {
				syncErr = err
				break
			}
			for i, rec := range recs {
				data, err := json.Marshal(rec)
				if err != nil {
					continue
				}
				extID := zoho.ExternalID(rec, fmt.Sprintf("row_%d_%d", page, i))
				if _, err := tdb.Exec(ctx, `
					INSERT INTO records (connection_id, entity_name, external_id, data, synced_at)
					VALUES ($1,$2,$3,$4,now())
					ON CONFLICT (connection_id, entity_name, external_id)
					DO UPDATE SET data=EXCLUDED.data, synced_at=now()`,
					connectionID, entity, extID, data); err != nil {
					syncErr = err
					break
				}
				total++
			}
			if syncErr != nil || !more || total >= maxRecordsPerSync {
				break
			}
			page++
		}
	default:
		syncErr = fmt.Errorf("connector %s hələ dəstəklənmir", connectorType)
	}

	// bookkeeping
	status, msg := "success", ""
	if syncErr != nil {
		status, msg = "error", syncErr.Error()
	}
	tdb.Exec(ctx, `
		INSERT INTO synced_entities (connection_id, entity_name, record_count, last_synced_at)
		VALUES ($1,$2,(SELECT count(*) FROM records WHERE connection_id=$1 AND entity_name=$2),now())
		ON CONFLICT (connection_id, entity_name)
		DO UPDATE SET record_count=(SELECT count(*) FROM records WHERE connection_id=$1 AND entity_name=$2), last_synced_at=now()`,
		connectionID, entity)
	tdb.Exec(ctx, `INSERT INTO sync_log (connection_id, entity_name, status, message, records_synced) VALUES ($1,$2,$3,$4,$5)`,
		connectionID, entity, status, msg, total)
	if syncErr != nil {
		tdb.Exec(ctx, `UPDATE connections SET last_error=$1 WHERE id=$2`, syncErr.Error(), connectionID)
	} else {
		tdb.Exec(ctx, `UPDATE connections SET last_sync_at=now(), last_error=NULL WHERE id=$1`, connectionID)
	}
	_ = time.Now
	return total, syncErr
}
