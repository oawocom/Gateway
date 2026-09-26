package epoint

import (
	"crypto/sha1"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

// Config for EPoint payment gateway (epoint.az).
type Config struct {
	PublicKey  string `json:"public_key"`
	PrivateKey string `json:"private_key"`
}

type Client struct {
	cfg  Config
	http *http.Client
}

func New(cfg Config) *Client {
	return &Client{cfg: cfg, http: &http.Client{Timeout: 20 * time.Second}}
}

// sign builds EPoint's signature: base64(sha1(private + data + private)).
func (c *Client) sign(data string) string {
	h := sha1.Sum([]byte(c.cfg.PrivateKey + data + c.cfg.PrivateKey))
	return base64.StdEncoding.EncodeToString(h[:])
}

// Test sends a signed get-status request; a signature-level rejection
// means the keys are wrong, any other well-formed answer means they work.
func (c *Client) Test() error {
	payload, _ := json.Marshal(map[string]string{
		"public_key":  c.cfg.PublicKey,
		"transaction": "gateway-connection-test",
	})
	data := base64.StdEncoding.EncodeToString(payload)
	form := url.Values{"data": {data}, "signature": {c.sign(data)}}
	resp, err := c.http.PostForm("https://epoint.az/api/1/get-status", form)
	if err != nil {
		return fmt.Errorf("EPoint API əlçatan deyil: %v", err)
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
	var out struct {
		Status  string `json:"status"`
		Message string `json:"message"`
	}
	if err := json.Unmarshal(body, &out); err != nil {
		return fmt.Errorf("EPoint cavabı oxunmadı (HTTP %d)", resp.StatusCode)
	}
	low := strings.ToLower(out.Message)
	if strings.Contains(low, "sign") || strings.Contains(low, "imza") || strings.Contains(low, "public") {
		return fmt.Errorf("açarlar qəbul edilmədi: %s", out.Message)
	}
	return nil
}

// Entities: transaction sync will be enabled next.
func (c *Client) Entities() ([]string, error) {
	return nil, fmt.Errorf("EPoint əməliyyat sinxronizasiyası tezliklə aktiv olunacaq — açarlar yadda saxlanılıb")
}
