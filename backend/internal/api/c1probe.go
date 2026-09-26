package api

import (
	"encoding/json"
	"net/http"

	"gateway/internal/connectors/c1"
	_ "gateway/internal/connectors/c1/azstandart1" // registers the AzStandart 1.x adapter
	"gateway/internal/connectors/c1meta"
	"gateway/internal/connectors/sqldb"
)

// loadProfile returns the cached c1meta profile for a connection, reading it
// from the 1C base when not cached yet.
func (s *Server) loadProfile(r *http.Request, id string) (*c1meta.Profile, *sqldb.Config, error) {
	tdb, _, err := s.tenantPool(r)
	if err != nil {
		return nil, nil, err
	}
	var ctype, enc string
	if err := tdb.QueryRow(r.Context(), `SELECT connector_type, config_enc FROM connections WHERE id=$1`, id).Scan(&ctype, &enc); err != nil {
		return nil, nil, err
	}
	var cfg sqldb.Config
	if err := s.Sync.LoadConfig(enc, &cfg); err != nil {
		return nil, nil, err
	}
	var raw []byte
	if err := tdb.QueryRow(r.Context(), `SELECT profile FROM c1_meta WHERE connection_id=$1`, id).Scan(&raw); err == nil {
		var p c1meta.Profile
		if json.Unmarshal(raw, &p) == nil && len(p.Tables) > 0 {
			return &p, &cfg, nil
		}
	}
	db, err := sqldb.New("mssql", cfg).Open()
	if err != nil {
		return nil, nil, err
	}
	defer db.Close()
	p, err := c1meta.Load(db)
	if err != nil {
		return nil, nil, err
	}
	raw, _ = json.Marshal(p)
	tdb.Exec(r.Context(), `INSERT INTO c1_meta (connection_id, platform, config_name, config_version, profile)
		VALUES ($1,$2,$3,$4,$5) ON CONFLICT (connection_id) DO UPDATE SET profile=EXCLUDED.profile, loaded_at=now()`,
		id, p.Platform, p.ConfigName, p.ConfigVersion, raw)
	return p, &cfg, nil
}

// connectionC1Probe proves the adapter chain on a live base: adapter choice,
// row counts and 3 sample rows per object.
func (s *Server) connectionC1Probe(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	prof, cfg, err := s.loadProfile(r, id)
	if err != nil {
		errJSON(w, 502, "profil yüklənmədi: "+err.Error())
		return
	}
	db, err := sqldb.New("mssql", *cfg).Open()
	if err != nil {
		errJSON(w, 502, "1C bazasına qoşulmaq alınmadı: "+err.Error())
		return
	}
	defer db.Close()

	ad, err := c1.Select(db, prof)
	if err != nil {
		errJSON(w, 400, err.Error())
		return
	}
	ctx := r.Context()
	q := c1.Query{Limit: 3, DefaultDueDays: 30}
	out := map[string]any{"info": ad.Info()}
	errs := map[string]string{}
	if v, err := ad.Counts(ctx); err != nil {
		errs["counts"] = err.Error()
	} else {
		out["counts"] = v
	}
	if v, err := ad.Customers(ctx, q); err != nil {
		errs["customers"] = err.Error()
	} else {
		out["customers"] = v
	}
	if v, err := ad.Contracts(ctx, q); err != nil {
		errs["contracts"] = err.Error()
	} else {
		out["contracts"] = v
	}
	if v, err := ad.Invoices(ctx, q); err != nil {
		errs["invoices"] = err.Error()
	} else {
		out["invoices"] = v
	}
	if v, err := ad.InvoiceLines(ctx, q); err != nil {
		errs["invoice_lines"] = err.Error()
	} else {
		out["invoice_lines"] = v
	}
	if v, err := ad.Payments(ctx, q); err != nil {
		errs["payments"] = err.Error()
	} else {
		out["payments"] = v
	}
	if len(errs) > 0 {
		out["errors"] = errs
	}
	writeJSON(w, 200, out)
}
