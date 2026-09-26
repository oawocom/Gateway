package api

import (
	"encoding/json"
	"net/http"
	"strings"
	"time"

	"gateway/internal/connectors"
	"gateway/internal/connectors/http1c"
	"gateway/internal/connectors/odata1c"
	"gateway/internal/connectors/epoint"
	"gateway/internal/connectors/kapitalbank"
	"gateway/internal/connectors/pashabank"
	"gateway/internal/connectors/sqldb"
	"gateway/internal/connectors/yigim"
	"gateway/internal/connectors/zoho"
)

func (s *Server) listConnectors(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, 200, map[string]any{"connectors": connectors.Catalog})
}

type connRow struct {
	ID            string     `json:"id"`
	Name          string     `json:"name"`
	ConnectorType string     `json:"connector_type"`
	Status        string     `json:"status"`
	LastSyncAt    *time.Time `json:"last_sync_at"`
	LastError     *string    `json:"last_error"`
	CreatedAt     time.Time  `json:"created_at"`
}

func (s *Server) listConnections(w http.ResponseWriter, r *http.Request) {
	tdb, _, err := s.tenantPool(r)
	if err != nil {
		errJSON(w, 500, "tenant db error")
		return
	}
	rows, err := tdb.Query(r.Context(),
		"SELECT id, name, connector_type, status, last_sync_at, last_error, created_at FROM connections ORDER BY created_at DESC")
	if err != nil {
		errJSON(w, 500, "db error")
		return
	}
	defer rows.Close()
	list := []connRow{}
	for rows.Next() {
		var c connRow
		if err := rows.Scan(&c.ID, &c.Name, &c.ConnectorType, &c.Status, &c.LastSyncAt, &c.LastError, &c.CreatedAt); err != nil {
			errJSON(w, 500, "db error")
			return
		}
		list = append(list, c)
	}
	writeJSON(w, 200, map[string]any{"connections": list})
}

type connCreateReq struct {
	Name          string            `json:"name"`
	ConnectorType string            `json:"connector_type"`
	Config        map[string]string `json:"config"`
}

func validateConfig(req *connCreateReq) (string, bool) {
	desc := connectors.Get(req.ConnectorType)
	if desc == nil || !desc.Available {
		return "bu connector mövcud deyil və ya hələ aktiv deyil", false
	}
	for _, f := range desc.Fields {
		if f.Required && strings.TrimSpace(req.Config[f.Key]) == "" {
			return f.Label + " tələb olunur", false
		}
	}
	return "", true
}

func buildClient(req *connCreateReq) (interface{ Test() error }, error) {
	switch req.ConnectorType {
	case "1c_odata":
		return odata1c.New(odata1c.Config{
			BaseURL:  req.Config["base_url"],
			Username: req.Config["username"],
			Password: req.Config["password"],
		}), nil
	case "1c_http":
		return http1c.New(http1c.Config{
			BaseURL:   req.Config["base_url"],
			Username:  req.Config["username"],
			Password:  req.Config["password"],
			Endpoints: req.Config["endpoints"],
		}), nil
	case "zoho_crm":
		return zoho.New(zoho.Config{
			Region:       req.Config["region"],
			ClientID:     req.Config["client_id"],
			ClientSecret: req.Config["client_secret"],
			RefreshToken: req.Config["refresh_token"],
		}), nil
	case "epoint":
		return epoint.New(epoint.Config{
			PublicKey:  req.Config["public_key"],
			PrivateKey: req.Config["private_key"],
		}), nil
	case "yigim":
		return yigim.New(yigim.Config{
			BaseURL:   req.Config["base_url"],
			Merchant:  req.Config["merchant"],
			SecretKey: req.Config["secret_key"],
		}), nil
	case "kapital_bank":
		return kapitalbank.New(kapitalbank.Config{
			BaseURL: req.Config["base_url"],
			Token:   req.Config["token"],
		}), nil
	case "pasha_bank":
		return pashabank.New(pashabank.Config{
			BaseURL: req.Config["base_url"],
			Token:   req.Config["token"],
		}), nil
	case "postgres", "mysql", "mssql":
		return sqldb.New(req.ConnectorType, sqldb.Config{
			Host:     req.Config["host"],
			Port:     req.Config["port"],
			Database: req.Config["database"],
			Username: req.Config["username"],
			Password: req.Config["password"],
			SSLMode:  req.Config["sslmode"],
		}), nil
	}
	return nil, nil
}

// testConnectionConfig tests credentials before saving.
func (s *Server) testConnectionConfig(w http.ResponseWriter, r *http.Request) {
	var req connCreateReq
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		errJSON(w, 400, "invalid json")
		return
	}
	if msg, ok := validateConfig(&req); !ok {
		errJSON(w, 400, msg)
		return
	}
	client, _ := buildClient(&req)
	if client == nil {
		errJSON(w, 400, "unsupported connector")
		return
	}
	if err := client.Test(); err != nil {
		writeJSON(w, 200, map[string]any{"ok": false, "message": err.Error()})
		return
	}
	writeJSON(w, 200, map[string]any{"ok": true, "message": "Bağlantı uğurludur"})
}

func (s *Server) createConnection(w http.ResponseWriter, r *http.Request) {
	var req connCreateReq
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		errJSON(w, 400, "invalid json")
		return
	}
	req.Name = strings.TrimSpace(req.Name)
	if req.Name == "" {
		errJSON(w, 400, "ad tələb olunur")
		return
	}
	if msg, ok := validateConfig(&req); !ok {
		errJSON(w, 400, msg)
		return
	}

	client, _ := buildClient(&req)
	if client == nil {
		errJSON(w, 400, "unsupported connector")
		return
	}
	if err := client.Test(); err != nil {
		errJSON(w, 400, "Bağlantı testi uğursuz: "+err.Error())
		return
	}

	cfgJSON, err := json.Marshal(req.Config)
	if err != nil {
		errJSON(w, 500, "internal error")
		return
	}
	enc, err := s.Box.Encrypt(cfgJSON)
	if err != nil {
		errJSON(w, 500, "encryption error")
		return
	}

	tdb, _, err := s.tenantPool(r)
	if err != nil {
		errJSON(w, 500, "tenant db error")
		return
	}
	var id string
	err = tdb.QueryRow(r.Context(),
		"INSERT INTO connections (name, connector_type, config_enc) VALUES ($1,$2,$3) RETURNING id",
		req.Name, req.ConnectorType, enc).Scan(&id)
	if err != nil {
		errJSON(w, 500, "could not create connection")
		return
	}
	writeJSON(w, 201, map[string]string{"id": id, "name": req.Name, "connector_type": req.ConnectorType})
}

// loadConnection fetches type+config for a connection in the tenant DB.
func (s *Server) loadConnection(r *http.Request, id string) (connectorType, configEnc string, err error) {
	tdb, _, err := s.tenantPool(r)
	if err != nil {
		return "", "", err
	}
	err = tdb.QueryRow(r.Context(),
		"SELECT connector_type, config_enc FROM connections WHERE id=$1", id).Scan(&connectorType, &configEnc)
	return
}

func (s *Server) testConnection(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	ctype, enc, err := s.loadConnection(r, id)
	if err != nil {
		errJSON(w, 404, "connection not found")
		return
	}
	var client interface{ Test() error }
	switch ctype {
	case "1c_odata":
		var cfg odata1c.Config
		if err := s.Sync.LoadConfig(enc, &cfg); err != nil {
			errJSON(w, 500, "config error")
			return
		}
		client = odata1c.New(cfg)
	case "1c_http":
		var cfg http1c.Config
		if err := s.Sync.LoadConfig(enc, &cfg); err != nil {
			errJSON(w, 500, "config error")
			return
		}
		client = http1c.New(cfg)
	case "zoho_crm":
		var cfg zoho.Config
		if err := s.Sync.LoadConfig(enc, &cfg); err != nil {
			errJSON(w, 500, "config error")
			return
		}
		client = zoho.New(cfg)
	case "pasha_bank":
		var cfg pashabank.Config
		if err := s.Sync.LoadConfig(enc, &cfg); err != nil {
			errJSON(w, 500, "config error")
			return
		}
		client = pashabank.New(cfg)
	case "kapital_bank":
		var cfg kapitalbank.Config
		if err := s.Sync.LoadConfig(enc, &cfg); err != nil {
			errJSON(w, 500, "config error")
			return
		}
		client = kapitalbank.New(cfg)
	case "epoint":
		var cfg epoint.Config
		if err := s.Sync.LoadConfig(enc, &cfg); err != nil {
			errJSON(w, 500, "config error")
			return
		}
		client = epoint.New(cfg)
	case "yigim":
		var cfg yigim.Config
		if err := s.Sync.LoadConfig(enc, &cfg); err != nil {
			errJSON(w, 500, "config error")
			return
		}
		client = yigim.New(cfg)
	case "postgres", "mysql", "mssql":
		var cfg sqldb.Config
		if err := s.Sync.LoadConfig(enc, &cfg); err != nil {
			errJSON(w, 500, "config error")
			return
		}
		client = sqldb.New(ctype, cfg)
	default:
		errJSON(w, 400, "unsupported connector")
		return
	}
	if err := client.Test(); err != nil {
		writeJSON(w, 200, map[string]any{"ok": false, "message": err.Error()})
		return
	}
	writeJSON(w, 200, map[string]any{"ok": true, "message": "Bağlantı uğurludur"})
}

func (s *Server) listConnectionEntities(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	ctype, enc, err := s.loadConnection(r, id)
	if err != nil {
		errJSON(w, 404, "connection not found")
		return
	}
	var lister interface{ Entities() ([]string, error) }
	switch ctype {
	case "1c_odata":
		var cfg odata1c.Config
		if err := s.Sync.LoadConfig(enc, &cfg); err != nil {
			errJSON(w, 500, "config error")
			return
		}
		lister = odata1c.New(cfg)
	case "1c_http":
		var cfg http1c.Config
		if err := s.Sync.LoadConfig(enc, &cfg); err != nil {
			errJSON(w, 500, "config error")
			return
		}
		lister = http1c.New(cfg)
	case "zoho_crm":
		var cfg zoho.Config
		if err := s.Sync.LoadConfig(enc, &cfg); err != nil {
			errJSON(w, 500, "config error")
			return
		}
		lister = zoho.New(cfg)
	case "pasha_bank":
		var cfg pashabank.Config
		if err := s.Sync.LoadConfig(enc, &cfg); err != nil {
			errJSON(w, 500, "config error")
			return
		}
		lister = pashabank.New(cfg)
	case "kapital_bank":
		var cfg kapitalbank.Config
		if err := s.Sync.LoadConfig(enc, &cfg); err != nil {
			errJSON(w, 500, "config error")
			return
		}
		lister = kapitalbank.New(cfg)
	case "epoint":
		var cfg epoint.Config
		if err := s.Sync.LoadConfig(enc, &cfg); err != nil {
			errJSON(w, 500, "config error")
			return
		}
		lister = epoint.New(cfg)
	case "yigim":
		var cfg yigim.Config
		if err := s.Sync.LoadConfig(enc, &cfg); err != nil {
			errJSON(w, 500, "config error")
			return
		}
		lister = yigim.New(cfg)
	case "postgres", "mysql", "mssql":
		var cfg sqldb.Config
		if err := s.Sync.LoadConfig(enc, &cfg); err != nil {
			errJSON(w, 500, "config error")
			return
		}
		lister = sqldb.New(ctype, cfg)
	default:
		errJSON(w, 400, "unsupported connector")
		return
	}
	names, err := lister.Entities()
	if err != nil {
		errJSON(w, 502, err.Error())
		return
	}
	writeJSON(w, 200, map[string]any{"entities": names})
}

func (s *Server) syncConnection(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	var req struct {
		Entity string `json:"entity"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || strings.TrimSpace(req.Entity) == "" {
		errJSON(w, 400, "entity tələb olunur")
		return
	}
	ctype, enc, err := s.loadConnection(r, id)
	if err != nil {
		errJSON(w, 404, "connection not found")
		return
	}
	tdb, _, err := s.tenantPool(r)
	if err != nil {
		errJSON(w, 500, "tenant db error")
		return
	}
	n, err := s.Sync.SyncEntity(r.Context(), tdb, id, ctype, enc, req.Entity)
	if err != nil {
		writeJSON(w, 200, map[string]any{"ok": false, "message": err.Error(), "records": n})
		return
	}
	writeJSON(w, 200, map[string]any{"ok": true, "records": n})
}

func (s *Server) deleteConnection(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	tdb, _, err := s.tenantPool(r)
	if err != nil {
		errJSON(w, 500, "tenant db error")
		return
	}
	tag, err := tdb.Exec(r.Context(), "DELETE FROM connections WHERE id=$1", id)
	if err != nil {
		errJSON(w, 500, "db error")
		return
	}
	if tag.RowsAffected() == 0 {
		errJSON(w, 404, "connection not found")
		return
	}
	writeJSON(w, 200, map[string]string{"id": id, "deleted": "true"})
}
