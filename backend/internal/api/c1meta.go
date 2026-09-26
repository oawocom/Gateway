package api

import (
	"encoding/json"
	"net/http"
	"time"

	"gateway/internal/connectors/c1meta"
	"gateway/internal/connectors/sqldb"
)

// connectionC1Meta returns the 1C metadata profile of an MSSQL connection
// (platform, configuration name/version, metadata→table map). Cached per
// connection in c1_meta; ?refresh=1 forces a re-read from the 1C base.
func (s *Server) connectionC1Meta(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	tdb, _, err := s.tenantPool(r)
	if err != nil {
		errJSON(w, 500, "tenant db error")
		return
	}
	ctx := r.Context()

	var ctype, enc string
	if err := tdb.QueryRow(ctx, `SELECT connector_type, config_enc FROM connections WHERE id=$1`, id).Scan(&ctype, &enc); err != nil {
		errJSON(w, 404, "bağlantı tapılmadı")
		return
	}
	if ctype != "mssql" {
		errJSON(w, 400, "1C metadata yalnız MSSQL bağlantısı üçün oxunur")
		return
	}

	if r.URL.Query().Get("refresh") == "" {
		var raw []byte
		var loaded time.Time
		if err := tdb.QueryRow(ctx, `SELECT profile, loaded_at FROM c1_meta WHERE connection_id=$1`, id).Scan(&raw, &loaded); err == nil {
			var prof c1meta.Profile
			if json.Unmarshal(raw, &prof) == nil {
				writeJSON(w, 200, map[string]any{"cached": true, "loaded_at": loaded, "profile": summary(&prof)})
				return
			}
		}
	}

	var cfg sqldb.Config
	if err := s.Sync.LoadConfig(enc, &cfg); err != nil {
		errJSON(w, 500, "config decrypt error")
		return
	}
	db, err := sqldb.New("mssql", cfg).Open()
	if err != nil {
		errJSON(w, 502, "1C bazasına qoşulmaq alınmadı: "+err.Error())
		return
	}
	defer db.Close()

	prof, err := c1meta.Load(db)
	if err != nil {
		errJSON(w, 502, "1C metadata oxunmadı: "+err.Error())
		return
	}
	raw, _ := json.Marshal(prof)
	if _, err := tdb.Exec(ctx, `
		INSERT INTO c1_meta (connection_id, platform, config_name, config_version, profile, loaded_at)
		VALUES ($1,$2,$3,$4,$5,now())
		ON CONFLICT (connection_id) DO UPDATE SET platform=EXCLUDED.platform, config_name=EXCLUDED.config_name,
		  config_version=EXCLUDED.config_version, profile=EXCLUDED.profile, loaded_at=now()`,
		id, prof.Platform, prof.ConfigName, prof.ConfigVersion, raw); err != nil {
		errJSON(w, 500, "c1_meta save error: "+err.Error())
		return
	}
	writeJSON(w, 200, map[string]any{"cached": false, "loaded_at": time.Now(), "profile": summary(prof)})
}

// summary trims the full map for the API response.
func summary(p *c1meta.Profile) map[string]any {
	return map[string]any{
		"platform":         p.Platform,
		"config_name":      p.ConfigName,
		"config_synonym":   p.ConfigSynonym,
		"config_version":   p.ConfigVersion,
		"vendor":           p.Vendor,
		"config_guid":      p.ConfigGUID,
		"counts":           map[string]int{"tables": len(p.Tables), "fields": len(p.Fields), "tabular_sections": len(p.TabularSections), "accrg_ed": len(p.AccRgED)},
		"tables":           p.Tables,
		"tabular_sections": p.TabularSections,
		"accrg_ed":         p.AccRgED,
	}
}
