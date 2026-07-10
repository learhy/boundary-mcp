// Package boundaryclient provides an HTTP client for the Boundary REST API (/v1/).
// It reads configuration from environment variables and handles TLS, auth,
// and request execution.
package boundaryclient

import (
	"bytes"
	"context"
	"crypto/tls"
	"crypto/x509"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"time"

	"github.com/learhy/boundary-mcp/internal/config"
)

// Client wraps the Boundary API configuration and HTTP transport.
type Client struct {
	Addr       string
	Token      string
	HTTPClient *http.Client
}

// New creates a new Boundary API client from config.
func New(cfg *config.Config) (*Client, error) {
	tlsConfig := &tls.Config{}

	if cfg.BoundaryTLSInsecure {
		tlsConfig.InsecureSkipVerify = true
	}

	if cfg.BoundaryCACert != "" {
		pemData, err := os.ReadFile(cfg.BoundaryCACert)
		if err != nil {
			return nil, fmt.Errorf("read CA cert: %w", err)
		}
		caPool := x509.NewCertPool()
		if !caPool.AppendCertsFromPEM(pemData) {
			return nil, fmt.Errorf("failed to parse CA cert from %s", cfg.BoundaryCACert)
		}
		tlsConfig.RootCAs = caPool
	}

	if cfg.BoundaryCAPath != "" {
		caPool, err := loadCAsFromDir(cfg.BoundaryCAPath)
		if err != nil {
			return nil, fmt.Errorf("load CA certs from dir: %w", err)
		}
		tlsConfig.RootCAs = caPool
		if cfg.BoundaryCACert != "" {
			// Merge: add the single cert too
			pemData, _ := os.ReadFile(cfg.BoundaryCACert)
			tlsConfig.RootCAs.AppendCertsFromPEM(pemData)
		}
	}

	transport := &http.Transport{
		TLSClientConfig: tlsConfig,
	}

	httpClient := &http.Client{
		Transport: transport,
		Timeout:   cfg.BoundaryClientTimeout,
	}

	return &Client{
		Addr:       cfg.BoundaryAddr,
		Token:      cfg.BoundaryToken,
		HTTPClient: httpClient,
	}, nil
}

// CheckLiveness performs a lightweight GET to the global scope to verify
// the controller is reachable and the token is valid.
func (c *Client) CheckLiveness(ctx context.Context) error {
	if c.Token == "" {
		return fmt.Errorf("no token configured (BOUNDARY_TOKEN not set)")
	}
	resp, err := c.Get(ctx, "/v1/scopes/global")
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("liveness check failed: HTTP %d", resp.StatusCode)
	}
	return nil
}

// Get performs an authenticated GET request.
func (c *Client) Get(ctx context.Context, path string) (*http.Response, error) {
	req, err := http.NewRequestWithContext(ctx, "GET", c.Addr+path, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+c.Token)
	return c.HTTPClient.Do(req)
}

// PostJSON performs an authenticated POST with a JSON body.
func (c *Client) PostJSON(ctx context.Context, path string, body interface{}) (*http.Response, error) {
	var bodyReader io.Reader
	if body != nil {
		data, err := json.Marshal(body)
		if err != nil {
			return nil, err
		}
		bodyReader = bytes.NewReader(data)
	}
	req, err := http.NewRequestWithContext(ctx, "POST", c.Addr+path, bodyReader)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+c.Token)
	req.Header.Set("Content-Type", "application/json")
	return c.HTTPClient.Do(req)
}

// Delete performs an authenticated DELETE request.
func (c *Client) Delete(ctx context.Context, path string) (*http.Response, error) {
	req, err := http.NewRequestWithContext(ctx, "DELETE", c.Addr+path, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+c.Token)
	return c.HTTPClient.Do(req)
}

// ReadResponseBody reads and closes a response body.
func (c *Client) ReadResponseBody(resp *http.Response) ([]byte, error) {
	defer resp.Body.Close()
	return io.ReadAll(resp.Body)
}

// DoGet performs a raw GET and returns the body + status code.
func (c *Client) DoGet(ctx context.Context, path string) (json.RawMessage, int, error) {
	resp, err := c.Get(ctx, path)
	if err != nil {
		return nil, 0, err
	}
	body, err := c.ReadResponseBody(resp)
	if err != nil {
		return nil, resp.StatusCode, err
	}
	return json.RawMessage(body), resp.StatusCode, nil
}

// DoPost performs a raw POST and returns the body + status code.
func (c *Client) DoPost(ctx context.Context, path string, body interface{}) (json.RawMessage, int, error) {
	resp, err := c.PostJSON(ctx, path, body)
	if err != nil {
		return nil, 0, err
	}
	bodyBytes, err := c.ReadResponseBody(resp)
	if err != nil {
		return nil, resp.StatusCode, err
	}
	return json.RawMessage(bodyBytes), resp.StatusCode, nil
}

// DoDelete performs a raw DELETE and returns the body + status code.
func (c *Client) DoDelete(ctx context.Context, path string) (json.RawMessage, int, error) {
	resp, err := c.Delete(ctx, path)
	if err != nil {
		return nil, 0, err
	}
	bodyBytes, err := c.ReadResponseBody(resp)
	if err != nil {
		return nil, resp.StatusCode, err
	}
	return json.RawMessage(bodyBytes), resp.StatusCode, nil
}

// loadCAsFromDir loads all PEM files from a directory into a cert pool.
func loadCAsFromDir(dir string) (*x509.CertPool, error) {
	pool := x509.NewCertPool()
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, err
	}
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		pemData, err := os.ReadFile(dir + "/" + entry.Name())
		if err != nil {
			continue
		}
		pool.AppendCertsFromPEM(pemData)
	}
	return pool, nil
}

// Unused import guard — time is used by config but not this file directly.
var _ = time.Now