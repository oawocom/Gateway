package kapitalbank

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"sort"
	"strings"
	"time"
)

// Config for Kapital Bank (Birbank Biznes) B2B open API.
//
// The public spec sits behind the api.birbank.business portal login, so the
// data endpoints below follow the standard open-banking shape (accounts +
// statements per IBAN) with a tolerant response parser. When the bank's
// credentials arrive, at most the path constants below need adjusting —
// nothing elsewhere in Gateway.
type Config struct {
	BaseURL string `json:"base_url"` // default: https://my.birbank.business/b2b/api/public/v1
	Token   string `json:"token"`
}

// endpoint paths in one place, easy to fix against the real spec
const (
	pathAccounts   = "/accounts"
	pathStatements = "/accounts/%s/statements" // + ?fromDate&toDate&page&size
)

type Client struct {
	cfg  Config
	http *http.Client
}

func New(cfg Config) *Client {
	cfg.BaseURL = strings.TrimRight(strings.TrimSpace(cfg.BaseURL), "/")
	if cfg.BaseURL == "" {
		cfg.BaseURL = "https://my.birbank.business/b2b/api/public/v1"
	}
	return &Client{cfg: cfg, http: &http.Client{Timeout: 25 * time.Second}}
}

func (c *Client) api(method, path string, body any, out *envelope) error {
	var rd io.Reader
	if body != nil {
		b, err := json.Marshal(body)
		if err != nil {
			return err
		}
		rd = bytes.NewReader(b)
	}
	req, err := http.NewRequest(method, c.cfg.BaseURL+path, rd)
	if err != nil {
		return err
	}
	req.Header.Set("Accept", "application/json")
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	if c.cfg.Token != "" {
		req.Header.Set("Authorization", "Bearer "+c.cfg.Token)
	}
	resp, err := c.http.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	data, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if resp.StatusCode < 200 || resp.StatusCode > 299 {
		snip := strings.TrimSpace(string(data))
		if len(snip) > 300 {
			snip = snip[:300]
		}
		return fmt.Errorf("bank API %d: %s", resp.StatusCode, snip)
	}
	if out != nil {
		return out.parse(data)
	}
	return nil
}

// envelope tolerates the common REST list shapes: a bare array, {data:[...]},
// {content:[...], totalPages}, {items:[...]}, {result:[...]}.
type envelope struct {
	Items      []map[string]any
	TotalPages int
}

func (e *envelope) parse(data []byte) error {
	var arr []map[string]any
	if json.Unmarshal(data, &arr) == nil {
		e.Items = arr
		return nil
	}
	var obj map[string]json.RawMessage
	if err := json.Unmarshal(data, &obj); err != nil {
		return fmt.Errorf("gözlənilməz cavab formatı")
	}
	for _, k := range []string{"content", "data", "items", "result", "accounts", "statements", "transactions"} {
		if raw, ok := obj[k]; ok {
			if json.Unmarshal(raw, &arr) == nil {
				e.Items = arr
				break
			}
		}
	}
	if raw, ok := obj["totalPages"]; ok {
		json.Unmarshal(raw, &e.TotalPages)
	}
	return nil
}

// Accounts returns the business accounts visible to this token.
func (c *Client) Accounts() ([]map[string]any, error) {
	var out envelope
	if err := c.api("GET", pathAccounts, nil, &out); err != nil {
		return nil, err
	}
	return out.Items, nil
}

// Test verifies the token by listing accounts (a 401/403 means bad token).
func (c *Client) Test() error {
	_, err := c.Accounts()
	if err != nil && strings.Contains(err.Error(), "bank API 40") {
		return fmt.Errorf("token bank tərəfindən qəbul edilmədi: %v", err)
	}
	return err
}

// Entities: the accounts list plus a statement stream per account IBAN.
func (c *Client) Entities() ([]string, error) {
	accs, err := c.Accounts()
	if err != nil {
		return nil, err
	}
	names := []string{"accounts"}
	ibans := []string{}
	for _, a := range accs {
		for _, k := range []string{"iban", "accountNo", "accountNumber"} {
			if v, _ := a[k].(string); v != "" {
				ibans = append(ibans, "statements:"+v)
				break
			}
		}
	}
	sort.Strings(ibans)
	return append(names, ibans...), nil
}

// FetchPage returns one page of the given entity and whether more remain.
// Statements cover the trailing 24 months.
func (c *Client) FetchPage(entity string, page, size int) ([]map[string]any, bool, error) {
	if entity == "accounts" {
		if page > 0 {
			return nil, false, nil
		}
		accs, err := c.Accounts()
		return accs, false, err
	}
	iban, ok := strings.CutPrefix(entity, "statements:")
	if !ok || iban == "" {
		return nil, false, fmt.Errorf("naməlum entity: %s", entity)
	}
	if size < 1 || size > 200 {
		size = 200
	}
	now := time.Now()
	path := fmt.Sprintf(pathStatements, iban) +
		fmt.Sprintf("?fromDate=%s&toDate=%s&page=%d&size=%d",
			now.AddDate(-2, 0, 0).Format("2006-01-02"), now.Format("2006-01-02"), page, size)
	var out envelope
	if err := c.api("GET", path, nil, &out); err != nil {
		return nil, false, err
	}
	more := out.TotalPages > 0 && page+1 < out.TotalPages
	if out.TotalPages == 0 { // no paging info: stop when a page comes back short
		more = len(out.Items) == size
	}
	return out.Items, more, nil
}

// ExternalID builds a stable id for a fetched record.
func ExternalID(rec map[string]any, fallback string) string {
	for _, k := range []string{"transactionId", "operationId", "id", "iban", "accountNo", "accountNumber"} {
		switch v := rec[k].(type) {
		case string:
			if v != "" {
				return v
			}
		case float64:
			return fmt.Sprintf("%.0f", v)
		}
	}
	return fallback
}
