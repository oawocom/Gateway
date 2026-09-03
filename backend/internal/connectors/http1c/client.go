package http1c

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"sort"
	"strings"
	"time"
)

// Config for custom 1C HTTP services (works with any 1C version that can
// publish HTTP services, including setups without OData).
type Config struct {
	BaseURL   string `json:"base_url"`
	Username  string `json:"username"`
	Password  string `json:"password"`
	Endpoints string `json:"endpoints"` // lines of "Name:/hs/path"
}

type Client struct {
	cfg  Config
	http *http.Client
}

func New(cfg Config) *Client {
	cfg.BaseURL = strings.TrimRight(strings.TrimSpace(cfg.BaseURL), "/")
	return &Client{cfg: cfg, http: &http.Client{Timeout: 90 * time.Second}}
}

// parseEndpoints returns name -> path.
func (c *Client) parseEndpoints() map[string]string {
	out := map[string]string{}
	for _, line := range strings.Split(c.cfg.Endpoints, "\n") {
		line = strings.TrimSpace(strings.Trim(line, ","))
		if line == "" {
			continue
		}
		name, path, ok := strings.Cut(line, ":")
		if !ok {
			continue
		}
		name = strings.TrimSpace(name)
		path = strings.TrimSpace(path)
		if name == "" || path == "" {
			continue
		}
		if !strings.HasPrefix(path, "/") {
			path = "/" + path
		}
		out[name] = path
	}
	return out
}

func (c *Client) get(path string) ([]byte, error) {
	req, err := http.NewRequest(http.MethodGet, c.cfg.BaseURL+path, nil)
	if err != nil {
		return nil, err
	}
	req.SetBasicAuth(c.cfg.Username, c.cfg.Password)
	req.Header.Set("Accept", "application/json")
	resp, err := c.http.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode == http.StatusUnauthorized {
		return nil, errors.New("1C: istifadəçi adı və ya şifrə yanlışdır (401)")
	}
	if resp.StatusCode >= 400 {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 300))
		return nil, fmt.Errorf("1C xətası (%d) %s: %s", resp.StatusCode, path, strings.TrimSpace(string(body)))
	}
	return io.ReadAll(io.LimitReader(resp.Body, 200<<20)) // 200MB cap
}

// Test verifies config and the first endpoint.
func (c *Client) Test() error {
	if !strings.HasPrefix(c.cfg.BaseURL, "http://") && !strings.HasPrefix(c.cfg.BaseURL, "https://") {
		return errors.New("URL http:// və ya https:// ilə başlamalıdır")
	}
	eps := c.parseEndpoints()
	if len(eps) == 0 {
		return errors.New("ən azı bir endpoint göstərin (format: Ad:/hs/yol)")
	}
	names := make([]string, 0, len(eps))
	for n := range eps {
		names = append(names, n)
	}
	sort.Strings(names)
	if _, err := c.Fetch(names[0]); err != nil {
		return err
	}
	return nil
}

// Entities lists user-defined endpoint names.
func (c *Client) Entities() ([]string, error) {
	eps := c.parseEndpoints()
	if len(eps) == 0 {
		return nil, errors.New("endpoint təyin olunmayıb")
	}
	names := make([]string, 0, len(eps))
	for n := range eps {
		names = append(names, n)
	}
	sort.Strings(names)
	return names, nil
}

// Fetch downloads one endpoint and normalizes it to a record slice.
// Accepts: bare JSON array, {"value":[...]}, {"data":[...]}, {"items":[...]}.
func (c *Client) Fetch(entity string) ([]map[string]any, error) {
	eps := c.parseEndpoints()
	path, ok := eps[entity]
	if !ok {
		return nil, fmt.Errorf("endpoint tapılmadı: %s", entity)
	}
	raw, err := c.get(path)
	if err != nil {
		return nil, err
	}
	raw = []byte(strings.TrimSpace(string(raw)))

	var arr []map[string]any
	if err := json.Unmarshal(raw, &arr); err == nil {
		return arr, nil
	}
	var wrap map[string]json.RawMessage
	if err := json.Unmarshal(raw, &wrap); err == nil {
		for _, key := range []string{"value", "data", "items", "rows", "list"} {
			if inner, ok := wrap[key]; ok {
				if err := json.Unmarshal(inner, &arr); err == nil {
					return arr, nil
				}
			}
		}
	}
	return nil, fmt.Errorf("%s: cavab JSON massiv formatında deyil", entity)
}
