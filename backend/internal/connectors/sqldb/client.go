package sqldb

import (
	"crypto/md5"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"strings"
	"time"

	_ "github.com/go-sql-driver/mysql"
	_ "github.com/jackc/pgx/v5/stdlib"
	_ "github.com/microsoft/go-mssqldb"
)

// Config for direct SQL database connections.
type Config struct {
	Host     string `json:"host"`
	Port     string `json:"port"`
	Database string `json:"database"`
	Username string `json:"username"`
	Password string `json:"password"`
	SSLMode  string `json:"sslmode"` // postgres only, optional
}

type Client struct {
	driver string // postgres | mysql | mssql
	cfg    Config
}

func New(driver string, cfg Config) *Client {
	cfg.Host = strings.TrimSpace(cfg.Host)
	cfg.Port = strings.TrimSpace(cfg.Port)
	cfg.Database = strings.TrimSpace(cfg.Database)
	return &Client{driver: driver, cfg: cfg}
}

func (c *Client) defaultPort() string {
	switch c.driver {
	case "postgres":
		return "5432"
	case "mysql":
		return "3306"
	case "mssql":
		return "1433"
	}
	return ""
}

func (c *Client) open() (*sql.DB, error) {
	port := c.cfg.Port
	if port == "" {
		port = c.defaultPort()
	}
	var dsn, drv string
	switch c.driver {
	case "postgres":
		drv = "pgx"
		ssl := c.cfg.SSLMode
		if ssl == "" {
			ssl = "prefer"
		}
		dsn = fmt.Sprintf("postgres://%s:%s@%s:%s/%s?sslmode=%s",
			url.QueryEscape(c.cfg.Username), url.QueryEscape(c.cfg.Password),
			c.cfg.Host, port, url.PathEscape(c.cfg.Database), url.QueryEscape(ssl))
	case "mysql":
		drv = "mysql"
		dsn = fmt.Sprintf("%s:%s@tcp(%s:%s)/%s?parseTime=true&timeout=15s&readTimeout=90s",
			c.cfg.Username, c.cfg.Password, c.cfg.Host, port, c.cfg.Database)
	case "mssql":
		drv = "sqlserver"
		u := &url.URL{
			Scheme:   "sqlserver",
			User:     url.UserPassword(c.cfg.Username, c.cfg.Password),
			Host:     c.cfg.Host + ":" + port,
			RawQuery: "database=" + url.QueryEscape(c.cfg.Database) + "&connection+timeout=15",
		}
		dsn = u.String()
	default:
		return nil, fmt.Errorf("dəstəklənməyən driver: %s", c.driver)
	}
	db, err := sql.Open(drv, dsn)
	if err != nil {
		return nil, err
	}
	db.SetMaxOpenConns(3)
	db.SetConnMaxLifetime(5 * time.Minute)
	return db, nil
}

func (c *Client) Test() error {
	if c.cfg.Host == "" || c.cfg.Database == "" {
		return errors.New("host və baza adı tələb olunur")
	}
	db, err := c.open()
	if err != nil {
		return err
	}
	defer db.Close()
	if err := db.Ping(); err != nil {
		return fmt.Errorf("qoşulma alınmadı: %v", err)
	}
	return nil
}

// Entities lists base tables (schema-qualified where applicable).
func (c *Client) Entities() ([]string, error) {
	db, err := c.open()
	if err != nil {
		return nil, err
	}
	defer db.Close()

	var q string
	switch c.driver {
	case "postgres":
		q = `SELECT table_schema || '.' || table_name FROM information_schema.tables
		     WHERE table_type='BASE TABLE' AND table_schema NOT IN ('pg_catalog','information_schema')
		     ORDER BY 1`
	case "mysql":
		q = `SELECT table_name FROM information_schema.tables
		     WHERE table_schema = DATABASE() AND table_type='BASE TABLE' ORDER BY 1`
	case "mssql":
		q = `SELECT s.name + '.' + t.name FROM sys.tables t
		     JOIN sys.schemas s ON s.schema_id = t.schema_id ORDER BY 1`
	}
	rows, err := db.Query(q)
	if err != nil {
		return nil, fmt.Errorf("cədvəl siyahısı alınmadı: %v", err)
	}
	defer rows.Close()
	var out []string
	for rows.Next() {
		var t string
		if rows.Scan(&t) == nil {
			out = append(out, t)
		}
	}
	return out, nil
}

// quoteTable safely quotes a (possibly schema-qualified) table name.
func (c *Client) quoteTable(table string) string {
	parts := strings.Split(table, ".")
	for i, p := range parts {
		p = strings.ReplaceAll(p, "\"", "")
		p = strings.ReplaceAll(p, "`", "")
		p = strings.ReplaceAll(p, "]", "")
		switch c.driver {
		case "mysql":
			parts[i] = "`" + p + "`"
		case "mssql":
			parts[i] = "[" + p + "]"
		default:
			parts[i] = `"` + p + `"`
		}
	}
	return strings.Join(parts, ".")
}

// primaryKeys returns PK column names of a table (may be empty).
func (c *Client) primaryKeys(db *sql.DB, table string) []string {
	schema, name := "", table
	if i := strings.LastIndex(table, "."); i >= 0 {
		schema, name = table[:i], table[i+1:]
	}
	var q string
	var args []any
	switch c.driver {
	case "postgres":
		q = `SELECT kcu.column_name FROM information_schema.table_constraints tc
		     JOIN information_schema.key_column_usage kcu
		       ON tc.constraint_name=kcu.constraint_name AND tc.table_schema=kcu.table_schema
		     WHERE tc.constraint_type='PRIMARY KEY' AND tc.table_name=$1
		       AND ($2='' OR tc.table_schema=$2) ORDER BY kcu.ordinal_position`
		args = []any{name, schema}
	case "mysql":
		q = `SELECT kcu.column_name FROM information_schema.table_constraints tc
		     JOIN information_schema.key_column_usage kcu
		       ON tc.constraint_name=kcu.constraint_name AND tc.table_schema=kcu.table_schema AND tc.table_name=kcu.table_name
		     WHERE tc.constraint_type='PRIMARY KEY' AND tc.table_name=? AND tc.table_schema=DATABASE()
		     ORDER BY kcu.ordinal_position`
		args = []any{name}
	case "mssql":
		q = `SELECT kcu.COLUMN_NAME FROM INFORMATION_SCHEMA.TABLE_CONSTRAINTS tc
		     JOIN INFORMATION_SCHEMA.KEY_COLUMN_USAGE kcu
		       ON tc.CONSTRAINT_NAME=kcu.CONSTRAINT_NAME
		     WHERE tc.CONSTRAINT_TYPE='PRIMARY KEY' AND tc.TABLE_NAME=@p1
		       AND (@p2='' OR tc.TABLE_SCHEMA=@p2) ORDER BY kcu.ORDINAL_POSITION`
		args = []any{name, schema}
	}
	rows, err := db.Query(q, args...)
	if err != nil {
		return nil
	}
	defer rows.Close()
	var out []string
	for rows.Next() {
		var col string
		if rows.Scan(&col) == nil {
			out = append(out, col)
		}
	}
	return out
}

// FetchPage reads one page of rows as generic records plus stable external IDs.
func (c *Client) FetchPage(table string, top, skip int) (recs []map[string]any, ids []string, err error) {
	db, err := c.open()
	if err != nil {
		return nil, nil, err
	}
	defer db.Close()

	pks := c.primaryKeys(db, table)
	qt := c.quoteTable(table)

	var q string
	switch c.driver {
	case "mssql":
		q = fmt.Sprintf("SELECT * FROM %s ORDER BY (SELECT NULL) OFFSET %d ROWS FETCH NEXT %d ROWS ONLY", qt, skip, top)
	default:
		q = fmt.Sprintf("SELECT * FROM %s LIMIT %d OFFSET %d", qt, top, skip)
	}
	rows, err := db.Query(q)
	if err != nil {
		return nil, nil, fmt.Errorf("%s oxuna bilmədi: %v", table, err)
	}
	defer rows.Close()

	cols, err := rows.Columns()
	if err != nil {
		return nil, nil, err
	}
	for rows.Next() {
		vals := make([]any, len(cols))
		ptrs := make([]any, len(cols))
		for i := range vals {
			ptrs[i] = &vals[i]
		}
		if err := rows.Scan(ptrs...); err != nil {
			return nil, nil, err
		}
		rec := make(map[string]any, len(cols))
		for i, col := range cols {
			rec[col] = normalize(vals[i])
		}
		recs = append(recs, rec)
		ids = append(ids, externalID(rec, pks))
	}
	return recs, ids, rows.Err()
}

func normalize(v any) any {
	switch x := v.(type) {
	case []byte:
		return string(x)
	case time.Time:
		return x.Format(time.RFC3339)
	default:
		return v
	}
}

// externalID builds a stable id from PK values, falling back to a row hash.
func externalID(rec map[string]any, pks []string) string {
	if len(pks) > 0 {
		parts := make([]string, 0, len(pks))
		for _, k := range pks {
			parts = append(parts, fmt.Sprint(rec[k]))
		}
		return strings.Join(parts, "|")
	}
	b, _ := json.Marshal(rec)
	h := md5.Sum(b)
	return hex.EncodeToString(h[:])
}
