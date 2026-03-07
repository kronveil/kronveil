package vault

import (
	"context"
	"fmt"
	"log"
	"sync"
	"time"

	"github.com/kronveil/kronveil/core/engine"
)

// Config holds HashiCorp Vault configuration.
type Config struct {
	Address    string `yaml:"address" json:"address"`
	Token      string `yaml:"token" json:"token"`
	AuthMethod string `yaml:"auth_method" json:"auth_method"` // "token", "kubernetes", "approle"
	MountPath  string `yaml:"mount_path" json:"mount_path"`
	Namespace  string `yaml:"namespace" json:"namespace"`
}

// Client integrates with HashiCorp Vault for secret-aware monitoring.
type Client struct {
	config   Config
	mu       sync.RWMutex
	secrets  map[string]*SecretMetadata
	certs    map[string]*CertificateInfo
	lastErr  error
}

// SecretMetadata tracks metadata about a monitored secret.
type SecretMetadata struct {
	Path        string    `json:"path"`
	Version     int       `json:"version"`
	CreatedAt   time.Time `json:"created_at"`
	ExpiresAt   *time.Time `json:"expires_at,omitempty"`
	LastRotated time.Time `json:"last_rotated"`
	RotationDue bool      `json:"rotation_due"`
}

// CertificateInfo tracks TLS certificate lifecycle.
type CertificateInfo struct {
	CommonName  string    `json:"common_name"`
	Issuer      string    `json:"issuer"`
	NotBefore   time.Time `json:"not_before"`
	NotAfter    time.Time `json:"not_after"`
	DaysToExpiry int      `json:"days_to_expiry"`
	AutoRenew   bool      `json:"auto_renew"`
}

// NewClient creates a new Vault client.
func NewClient(config Config) (*Client, error) {
	if config.Address == "" {
		return nil, fmt.Errorf("vault address is required")
	}

	return &Client{
		config:  config,
		secrets: make(map[string]*SecretMetadata),
		certs:   make(map[string]*CertificateInfo),
	}, nil
}

func (c *Client) Name() string { return "hashicorp-vault" }

func (c *Client) Initialize(ctx context.Context) error {
	// In production: creates Vault API client and validates auth.
	log.Printf("[vault] Vault integration initialized (address: %s, auth: %s)",
		c.config.Address, c.config.AuthMethod)
	return nil
}

func (c *Client) Close() error {
	log.Println("[vault] Vault integration closed")
	return nil
}

func (c *Client) Health() engine.ComponentHealth {
	c.mu.RLock()
	defer c.mu.RUnlock()
	status := "healthy"
	msg := fmt.Sprintf("monitoring %d secrets, %d certificates", len(c.secrets), len(c.certs))
	if c.lastErr != nil {
		status = "degraded"
		msg = c.lastErr.Error()
	}
	return engine.ComponentHealth{
		Name:      "vault",
		Status:    status,
		Message:   msg,
		LastCheck: time.Now(),
	}
}

// ReadSecret reads a secret from Vault.
func (c *Client) ReadSecret(ctx context.Context, path string) (map[string]interface{}, error) {
	// In production: uses vault/api client.Logical().Read().
	_ = ctx
	return nil, fmt.Errorf("vault read requires configured credentials")
}

// MonitorSecretRotation checks if secrets are within their rotation window.
func (c *Client) MonitorSecretRotation(ctx context.Context, paths []string, maxAge time.Duration) []SecretMetadata {
	var dueForRotation []SecretMetadata

	c.mu.RLock()
	defer c.mu.RUnlock()

	for _, meta := range c.secrets {
		if time.Since(meta.LastRotated) > maxAge {
			meta.RotationDue = true
			dueForRotation = append(dueForRotation, *meta)
		}
	}

	return dueForRotation
}

// MonitorCertificates checks certificate expiry.
func (c *Client) MonitorCertificates(ctx context.Context, warningDays int) []CertificateInfo {
	var expiring []CertificateInfo

	c.mu.RLock()
	defer c.mu.RUnlock()

	for _, cert := range c.certs {
		cert.DaysToExpiry = int(time.Until(cert.NotAfter).Hours() / 24)
		if cert.DaysToExpiry <= warningDays {
			expiring = append(expiring, *cert)
		}
	}

	return expiring
}

// TrackSecret adds a secret path to monitoring.
func (c *Client) TrackSecret(path string, meta SecretMetadata) {
	c.mu.Lock()
	defer c.mu.Unlock()
	meta.Path = path
	c.secrets[path] = &meta
}

// TrackCertificate adds a certificate to monitoring.
func (c *Client) TrackCertificate(name string, cert CertificateInfo) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.certs[name] = &cert
}
