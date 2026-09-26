package yigim

import (
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

// Config for YIĞIM Payment Services (yigim.az).
type Config struct {
	BaseURL   string `json:"base_url"` // default: https://api.yigim.az
	Merchant  string `json:"merchant"` // X-Merchant code
	SecretKey string `json:"secret_key"`
}

type Client struct {
	cfg  Config
	http *http.Client
}

func New(cfg Config) *Client {
	cfg.BaseURL = strings.TrimRight(strings.TrimSpace(cfg.BaseURL), "/")
	if cfg.BaseURL == "" {
		cfg.BaseURL = "https://api.yigim.az"
	}
	return &Client{cfg: cfg, http: &http.Client{Timeout: 20 * time.Second}}
}

// Test verifies the API is reachable and merchant credentials are not rejected.
func (c *Client) Test() error {
	req, err := http.NewRequest("GET", c.cfg.BaseURL+"/", nil)
	if err != nil {
		return err
	}
	req.Header.Set("X-Merchant", c.cfg.Merchant)
	resp, err := c.http.Do(req)
	if err != nil {
		return fmt.Errorf("YIĞIM API əlçatan deyil: %v", err)
	}
	defer resp.Body.Close()
	io.Copy(io.Discard, io.LimitReader(resp.Body, 4096))
	switch {
	case resp.StatusCode == 401 || resp.StatusCode == 403:
		return fmt.Errorf("merchant məlumatları qəbul edilmədi (HTTP %d)", resp.StatusCode)
	case resp.StatusCode >= 500:
		return fmt.Errorf("YIĞIM API xətası (HTTP %d)", resp.StatusCode)
	}
	return nil
}

// Entities: payment sync will be enabled next.
func (c *Client) Entities() ([]string, error) {
	return nil, fmt.Errorf("YIĞIM ödəniş sinxronizasiyası tezliklə aktiv olunacaq — bağlantı yadda saxlanılıb")
}
