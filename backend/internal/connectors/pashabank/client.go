package pashabank

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

// Config for PASHA Bank Openbanking REST API (developer.pashabank.digital).
type Config struct {
	BaseURL string `json:"base_url"` // e.g. https://openapi.pashabank.digital or https://sandbox.pashabank.digital
	Token   string `json:"token"`    // bank-issued API token (sent as Bearer)
}

type Client struct {
	cfg  Config
	http *http.Client
}

func New(cfg Config) *Client {
	cfg.BaseURL = strings.TrimRight(strings.TrimSpace(cfg.BaseURL), "/")
	if cfg.BaseURL == "" {
		cfg.BaseURL = "https://openapi.pashabank.digital"
	}
	return &Client{cfg: cfg, http: &http.Client{Timeout: 25 * time.Second}}
}

func (c *Client) api(method, path string, body any, out any) error {
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
		return json.Unmarshal(data, out)
	}
	return nil
}

// Accounts returns the business accounts visible to this token.
func (c *Client) Accounts() ([]map[string]any, error) {
	var out []map[string]any
	if err := c.api("GET", "/api/v1/accounts", nil, &out); err != nil {
		return nil, err
	}
	return out, nil
}

// Test verifies credentials by listing accounts.
func (c *Client) Test() error {
	_, err := c.Accounts()
	return err
}

// Entities lists syncable entities: the accounts list itself plus a
// statement stream per account IBAN.
func (c *Client) Entities() ([]string, error) {
	accs, err := c.Accounts()
	if err != nil {
		return nil, err
	}
	names := []string{"accounts"}
	ibans := []string{}
	for _, a := range accs {
		if iban, _ := a["iban"].(string); iban != "" {
			ibans = append(ibans, "statements:"+iban)
		}
	}
	sort.Strings(ibans)
	return append(names, ibans...), nil
}

type statementReq struct {
	FromDate string    `json:"fromDate"`
	ToDate   string    `json:"toDate"`
	Paging   pagingReq `json:"operationPaging"`
}

type pagingReq struct {
	Page int `json:"page"`
	Size int `json:"size"`
}

type statementResp struct {
	Content    []map[string]any `json:"content"`
	TotalPages int              `json:"totalPages"`
}

// FetchPage returns one page of the given entity and whether more pages remain.
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
	req := statementReq{
		FromDate: now.AddDate(-2, 0, 0).Format("2006-01-02"),
		ToDate:   now.Format("2006-01-02"),
		Paging:   pagingReq{Page: page, Size: size},
	}
	var out statementResp
	if err := c.api("POST", "/api/v1/accounts/"+iban+"/statements/detailed", req, &out); err != nil {
		return nil, false, err
	}
	return out.Content, page+1 < out.TotalPages, nil
}

// ExternalID builds a stable id for a fetched record.
func ExternalID(rec map[string]any, fallback string) string {
	for _, k := range []string{"transactionId", "iban", "accountNo"} {
		if v, _ := rec[k].(string); v != "" {
			return v
		}
	}
	return fallback
}
