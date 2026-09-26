package kapitalbank

import (
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

// Config for Kapital Bank (Birbank Biznes) open API.
// Data endpoints will be enabled once the portal spec is finalised;
// for now the connector stores credentials and verifies them.
type Config struct {
	BaseURL string `json:"base_url"` // default: https://my.birbank.business/b2b/api/public/v1
	Token   string `json:"token"`
}

type Client struct {
	cfg  Config
	http *http.Client
}

func New(cfg Config) *Client {
	cfg.BaseURL = strings.TrimRight(strings.TrimSpace(cfg.BaseURL), "/")
	if cfg.BaseURL == "" {
		cfg.BaseURL = "https://my.birbank.business/b2b/api/public/v1"
	}
	return &Client{cfg: cfg, http: &http.Client{Timeout: 20 * time.Second}}
}

// Test verifies that the API is reachable and the token is not rejected.
func (c *Client) Test() error {
	req, err := http.NewRequest("GET", c.cfg.BaseURL+"/", nil)
	if err != nil {
		return err
	}
	req.Header.Set("Accept", "application/json")
	if c.cfg.Token != "" {
		req.Header.Set("Authorization", "Bearer "+c.cfg.Token)
	}
	resp, err := c.http.Do(req)
	if err != nil {
		return fmt.Errorf("bank API əlçatan deyil: %v", err)
	}
	defer resp.Body.Close()
	io.Copy(io.Discard, io.LimitReader(resp.Body, 4096))
	switch {
	case resp.StatusCode == 401 || resp.StatusCode == 403:
		return fmt.Errorf("token bank tərəfindən qəbul edilmədi (HTTP %d)", resp.StatusCode)
	case resp.StatusCode >= 500:
		return fmt.Errorf("bank API xətası (HTTP %d)", resp.StatusCode)
	}
	return nil
}

// Entities: data products are not wired yet.
func (c *Client) Entities() ([]string, error) {
	return nil, fmt.Errorf("Kapital Bank data sinxronizasiyası tezliklə aktiv olunacaq — bağlantı və token yadda saxlanılıb")
}
