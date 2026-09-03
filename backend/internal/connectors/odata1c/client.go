package odata1c

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

// Config holds 1C OData connection settings.
type Config struct {
	BaseURL  string `json:"base_url"`
	Username string `json:"username"`
	Password string `json:"password"`
}

type Client struct {
	cfg  Config
	http *http.Client
}

func New(cfg Config) *Client {
	cfg.BaseURL = strings.TrimRight(strings.TrimSpace(cfg.BaseURL), "/")
	// auto-append the standard OData path if the user pasted only the publication URL
	if cfg.BaseURL != "" && !strings.Contains(strings.ToLower(cfg.BaseURL), "odata") {
		cfg.BaseURL += "/odata/standard.odata"
	}
	return &Client{cfg: cfg, http: &http.Client{Timeout: 60 * time.Second}}
}

func (c *Client) get(path string, query url.Values) (*http.Response, error) {
	u := c.cfg.BaseURL + path
	if query == nil {
		query = url.Values{}
	}
	query.Set("$format", "json")
	u += "?" + query.Encode()
	req, err := http.NewRequest(http.MethodGet, u, nil)
	if err != nil {
		return nil, err
	}
	req.SetBasicAuth(c.cfg.Username, c.cfg.Password)
	req.Header.Set("Accept", "application/json")
	resp, err := c.http.Do(req)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode == http.StatusUnauthorized {
		resp.Body.Close()
		return nil, errors.New("1C: istifadəçi adı və ya şifrə yanlışdır (401)")
	}
	if resp.StatusCode == http.StatusNotFound {
		resp.Body.Close()
		return nil, errors.New("1C: OData interfeysi tapılmadı (404) — bazada OData publikasiya olunmayıb və ya URL yanlışdır. 1C 8.3.5+ tələb olunur; köhnə versiyalar üçün \"1C HTTP Servis\" connectorundan istifadə edin")
	}
	if resp.StatusCode >= 400 {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 500))
		resp.Body.Close()
		return nil, fmt.Errorf("1C xətası (%d): %s", resp.StatusCode, strings.TrimSpace(string(body)))
	}
	return resp, nil
}

// Test verifies the connection by listing the service document.
func (c *Client) Test() error {
	if !strings.HasPrefix(c.cfg.BaseURL, "http://") && !strings.HasPrefix(c.cfg.BaseURL, "https://") {
		return errors.New("URL http:// və ya https:// ilə başlamalıdır")
	}
	resp, err := c.get("", nil)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	var doc struct {
		Value []struct {
			Name string `json:"name"`
		} `json:"value"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&doc); err != nil {
		return errors.New("1C cavabı OData formatında deyil — URL-i yoxlayın")
	}
	return nil
}

// Entities returns available entity set names.
func (c *Client) Entities() ([]string, error) {
	resp, err := c.get("", nil)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	var doc struct {
		Value []struct {
			Name string `json:"name"`
			URL  string `json:"url"`
		} `json:"value"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&doc); err != nil {
		return nil, errors.New("entity siyahısı oxuna bilmədi")
	}
	names := make([]string, 0, len(doc.Value))
	for _, v := range doc.Value {
		n := v.Name
		if n == "" {
			n = v.URL
		}
		if n != "" {
			names = append(names, n)
		}
	}
	return names, nil
}

// FetchPage returns up to `top` records of an entity starting at `skip`.
func (c *Client) FetchPage(entity string, top, skip int) ([]map[string]any, error) {
	q := url.Values{}
	q.Set("$top", fmt.Sprint(top))
	q.Set("$skip", fmt.Sprint(skip))
	resp, err := c.get("/"+url.PathEscape(entity), q)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	var doc struct {
		Value []map[string]any `json:"value"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&doc); err != nil {
		return nil, fmt.Errorf("%s: cavab oxuna bilmədi", entity)
	}
	return doc.Value, nil
}

// ExternalID picks a stable identifier from a 1C record.
func ExternalID(rec map[string]any, fallback string) string {
	for _, k := range []string{"Ref_Key", "Ref", "Number", "Code", "id", "ID"} {
		if v, ok := rec[k]; ok {
			if s, ok := v.(string); ok && s != "" {
				return s
			}
		}
	}
	return fallback
}
