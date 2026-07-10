// Package config handles environment-based configuration for the Boundary
// MCP server. It reads the standard Boundary client environment variables and
// validates them.
package config

import (
	"fmt"
	"os"
	"strconv"
	"time"
)

// Config holds all server configuration from environment variables.
type Config struct {
	// BoundaryAddr is the controller address (required).
	BoundaryAddr string
	// BoundaryToken is the pre-authenticated token (required for tool calls).
	BoundaryToken string
	// BoundaryCACert is the path to a CA cert PEM file.
	BoundaryCACert string
	// BoundaryCAPath is the path to a directory of CA cert PEM files.
	BoundaryCAPath string
	// BoundaryTLSInsecure skips TLS verification (dev only).
	BoundaryTLSInsecure bool
	// BoundaryClientTimeout is the HTTP client timeout.
	BoundaryClientTimeout time.Duration
	// BoundaryMaxRetries is the max retries on 5xx.
	BoundaryMaxRetries int
}

// FromEnv reads configuration from environment variables.
func FromEnv() (*Config, error) {
	c := &Config{
		BoundaryAddr:    os.Getenv("BOUNDARY_ADDR"),
		BoundaryToken:   os.Getenv("BOUNDARY_TOKEN"),
		BoundaryCACert:  os.Getenv("BOUNDARY_CACERT"),
		BoundaryCAPath:  os.Getenv("BOUNDARY_CAPATH"),
		BoundaryMaxRetries: 2,
		BoundaryClientTimeout: 60 * time.Second,
	}

	if v := os.Getenv("BOUNDARY_TLS_INSECURE"); v == "true" || v == "1" {
		c.BoundaryTLSInsecure = true
	}

	if v := os.Getenv("BOUNDARY_CLIENT_TIMEOUT"); v != "" {
		if secs, err := strconv.Atoi(v); err == nil {
			c.BoundaryClientTimeout = time.Duration(secs) * time.Second
		}
	}

	if v := os.Getenv("BOUNDARY_MAX_RETRIES"); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			c.BoundaryMaxRetries = n
		}
	}

	if c.BoundaryAddr == "" {
		return nil, fmt.Errorf("BOUNDARY_ADDR is required (e.g. https://boundary.example.com:9200)")
	}

	return c, nil
}

// HasToken returns true if a Boundary token is configured.
func (c *Config) HasToken() bool {
	return c.BoundaryToken != ""
}

// MaskToken returns a truncated form of the token for logging.
func MaskToken(token string) string {
	if len(token) <= 8 {
		return "***"
	}
	return token[:4] + "..." + token[len(token)-4:]
}