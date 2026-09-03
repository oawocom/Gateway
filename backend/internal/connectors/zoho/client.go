package zoho

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

// Config for Zoho CRM via a Self Client (client id/secret + refresh token).
type Config struct {
	Region       string `json:"region"` // com | eu | in | com.au | jp | com.cn
	ClientID     string `json:"client_id"`
	ClientSecret string `json:"client_secret"`
	RefreshToken string `json:"refresh_token"`
}

type Client struct {
	cfg  Config
	http *http.Client
}

func New(cfg Config) *Client {
	cfg.Region = strings.TrimPrefix(strings.TrimSpace(strings.ToLower(cfg.Region)), ".")
	if cfg.Region == "" {
		cfg.Region = "com"
	}
	return &Client{cfg: cfg, http: &http.Client{Timeout: 60 * time.Second}}
}

func (c *Client) accountsHost() string { return "https://accounts.zoho." + c.cfg.Region }
func (c *Client) apiHost() string      { return "https://www.zohoapis." + c.cfg.Region }

// accessToken exchanges the refresh token for a short-lived access token.
func (c *Client) accessToken() (string, error) {
	form := url.Values{
		"refresh_token": {c.cfg.RefreshToken},
		"client_id":     {c.cfg.ClientID},
		"client_secret": {c.cfg.ClientSecret},
		"grant_type":    {"refresh_token"},
	}
	resp, err := c.http.PostForm(c.accountsHost()+"/oauth/v2/token", form)
	if err != nil {
		return "", fmt.Errorf("Zoho accounts serverinə qoşulma alınmadı: %v", err)
	}
	defer resp.Body.Close()
	var out struct {
		AccessToken string `json:"access_token"`
		Error       string `json:"error"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return "", errors.New("Zoho token cavabı oxuna bilmədi")
	}
	if out.Error != "" {
		switch out.Error {
		case "invalid_code":
			return "", errors.New("Zoho: refresh token etibarsızdır və ya vaxtı bitib")
		case "invalid_client":
			return "", errors.New("Zoho: client ID / secret yanlışdır")
		default:
			return "", fmt.Errorf("Zoho token xətası: %s", out.Error)
		}
	}
	if out.AccessToken == "" {
		return "", errors.New("Zoho access token alına bilmədi")
	}
	return out.AccessToken, nil
}

func (c *Client) get(path string, token string) (*http.Response, error) {
	req, err := http.NewRequest(http.MethodGet, c.apiHost()+path, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Zoho-oauthtoken "+token)
	resp, err := c.http.Do(req)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode == http.StatusUnauthorized {
		resp.Body.Close()
		return nil, errors.New("Zoho: avtorizasiya rədd edildi (401)")
	}
	if resp.StatusCode >= 400 {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 300))
		resp.Body.Close()
		return nil, fmt.Errorf("Zoho xətası (%d): %s", resp.StatusCode, strings.TrimSpace(string(body)))
	}
	return resp, nil
}

func (c *Client) Test() error {
	if c.cfg.ClientID == "" || c.cfg.ClientSecret == "" || c.cfg.RefreshToken == "" {
		return errors.New("client ID, client secret və refresh token tələb olunur")
	}
	token, err := c.accessToken()
	if err != nil {
		return err
	}
	resp, err := c.get("/crm/v2/settings/modules", token)
	if err != nil {
		return err
	}
	resp.Body.Close()
	return nil
}

// Entities lists API-supported CRM modules.
func (c *Client) Entities() ([]string, error) {
	token, err := c.accessToken()
	if err != nil {
		return nil, err
	}
	resp, err := c.get("/crm/v2/settings/modules", token)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	var out struct {
		Modules []struct {
			APIName      string `json:"api_name"`
			APISupported bool   `json:"api_supported"`
		} `json:"modules"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return nil, errors.New("modul siyahısı oxuna bilmədi")
	}
	var names []string
	for _, m := range out.Modules {
		if m.APISupported && m.APIName != "" {
			names = append(names, m.APIName)
		}
	}
	return names, nil
}

// FetchPage returns one page of module records (per_page max 200).
// more=false means this was the last page.
func (c *Client) FetchPage(module string, page int) (recs []map[string]any, more bool, err error) {
	token, err := c.accessToken()
	if err != nil {
		return nil, false, err
	}
	path := fmt.Sprintf("/crm/v2/%s?page=%d&per_page=200", url.PathEscape(module), page)
	resp, err := c.get(path, token)
	if err != nil {
		return nil, false, err
	}
	defer resp.Body.Close()
	if resp.StatusCode == http.StatusNoContent {
		return nil, false, nil
	}
	var out struct {
		Data []map[string]any `json:"data"`
		Info struct {
			MoreRecords bool `json:"more_records"`
		} `json:"info"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return nil, false, fmt.Errorf("%s: cavab oxuna bilmədi", module)
	}
	return out.Data, out.Info.MoreRecords, nil
}

// ExternalID picks the Zoho record id.
func ExternalID(rec map[string]any, fallback string) string {
	if v, ok := rec["id"]; ok {
		if s, ok := v.(string); ok && s != "" {
			return s
		}
	}
	return fallback
}
