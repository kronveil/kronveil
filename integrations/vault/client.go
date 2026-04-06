// Package vault provides HashiCorp Vault integration.
package vault

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"sync"
	"time"

	vaultapi "github.com/hashicorp/vault/api"
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
	config      Config
	vaultClient *vaultapi.Client
	mu          sync.RWMutex
	secrets     map[string]*SecretMetadata
	certs       map[string]*CertificateInfo
	lastErr     error
	cancel      context.CancelFunc
	wg          sync.WaitGroup
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

	c := &Client{
		config:  config,
		secrets: make(map[string]*SecretMetadata),
		certs:   make(map[string]*CertificateInfo),
	}

	vaultCfg := vaultapi.DefaultConfig()
	vaultCfg.Address = config.Address
	vc, err := vaultapi.NewClient(vaultCfg)
	if err != nil {
		log.Printf("[vault] WARNING: failed to create Vault API client: %v (running in degraded mode)", err)
	} else {
		if config.Token != "" {
			vc.SetToken(config.Token)
		}
		if config.Namespace != "" {
			vc.SetNamespace(config.Namespace)
		}
		c.vaultClient = vc
	}

	return c, nil
}

func (c *Client) Name() string { return "hashicorp-vault" }

func (c *Client) Initialize(ctx context.Context) error {
	if c.vaultClient != nil {
		health, err := c.vaultClient.Sys().Health()
		if err != nil {
			c.mu.Lock()
			c.lastErr = fmt.Errorf("vault health check failed: %w", err)
			c.mu.Unlock()
			log.Printf("[vault] WARNING: health check failed: %v (continuing in degraded mode)", err)
		} else {
			log.Printf("[vault] Vault integration initialized (address: %s, sealed: %v)", c.config.Address, health.Sealed)
		}
	} else {
		log.Printf("[vault] Vault integration initialized in degraded mode (no client)")
	}
	return nil
}

func (c *Client) Close() error {
	if c.cancel != nil {
		c.cancel()
	}
	c.wg.Wait()
	log.Println("[vault] Vault integration closed")
	return nil
}

func (c *Client) Health() engine.ComponentHealth {
	c.mu.RLock()
	defer c.mu.RUnlock()
	status := "healthy"
	msg := fmt.Sprintf("monitoring %d secrets, %d certificates", len(c.secrets), len(c.certs))
	if c.vaultClient == nil {
		status = "degraded"
		msg = "no vault client configured"
	} else if c.lastErr != nil {
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
	if c.vaultClient == nil {
		return nil, fmt.Errorf("vault client not configured")
	}

	secret, err := c.vaultClient.Logical().ReadWithContext(ctx, path)
	if err != nil {
		c.mu.Lock()
		c.lastErr = err
		c.mu.Unlock()
		return nil, fmt.Errorf("vault read failed: %w", err)
	}
	if secret == nil {
		return nil, fmt.Errorf("secret not found at path: %s", path)
	}

	// Handle KV v2 nested data structure.
	if data, ok := secret.Data["data"].(map[string]interface{}); ok {
		return data, nil
	}
	return secret.Data, nil
}

// StartBackgroundSync begins periodic synchronization of tracked secrets and
// certificates from Vault. It runs until the provided context is cancelled or
// Close is called.
func (c *Client) StartBackgroundSync(ctx context.Context, interval time.Duration) {
	ctx, cancel := context.WithCancel(ctx)
	c.mu.Lock()
	c.cancel = cancel
	c.mu.Unlock()

	c.wg.Add(1)
	go func() {
		defer c.wg.Done()
		ticker := time.NewTicker(interval)
		defer ticker.Stop()

		// Run an initial sync immediately.
		c.syncSecrets(ctx)

		for {
			select {
			case <-ctx.Done():
				log.Println("[vault] background sync stopped")
				return
			case <-ticker.C:
				c.syncSecrets(ctx)
			}
		}
	}()
	log.Printf("[vault] background sync started (interval: %s)", interval)
}

// syncSecrets fetches metadata for all tracked secrets and certificates from Vault.
func (c *Client) syncSecrets(ctx context.Context) {
	if c.vaultClient == nil {
		return
	}

	c.mu.RLock()
	paths := make([]string, 0, len(c.secrets))
	for p := range c.secrets {
		paths = append(paths, p)
	}
	certPaths := make([]string, 0, len(c.certs))
	for p := range c.certs {
		certPaths = append(certPaths, p)
	}
	c.mu.RUnlock()

	for _, path := range paths {
		// Verify the secret still exists.
		_, err := c.ReadSecret(ctx, path)
		if err != nil {
			log.Printf("[vault] sync: secret at %s is not readable: %v", path, err)
			continue
		}

		// Fetch KV v2 metadata.
		metaSecret, err := c.vaultClient.Logical().ReadWithContext(ctx, "secret/metadata/"+path)
		if err != nil {
			log.Printf("[vault] sync: failed to read metadata for %s: %v", path, err)
			continue
		}
		if metaSecret == nil {
			continue
		}

		c.mu.Lock()
		meta, ok := c.secrets[path]
		if ok {
			if versionRaw, exists := metaSecret.Data["current_version"]; exists {
				if vNum, numOk := versionRaw.(json.Number); numOk {
					if v, convErr := vNum.Int64(); convErr == nil {
						meta.Version = int(v)
					}
				}
			}
			if createdRaw, exists := metaSecret.Data["created_time"]; exists {
				if ts, tsOk := createdRaw.(string); tsOk {
					if t, parseErr := time.Parse(time.RFC3339Nano, ts); parseErr == nil {
						meta.CreatedAt = t
						meta.LastRotated = t
					}
				}
			}
		}
		c.mu.Unlock()
	}

	for _, path := range certPaths {
		c.syncCertificate(ctx, path)
	}
}

// syncCertificate fetches certificate data from Vault and updates expiry info.
func (c *Client) syncCertificate(ctx context.Context, path string) {
	if c.vaultClient == nil {
		return
	}

	secret, err := c.vaultClient.Logical().ReadWithContext(ctx, path)
	if err != nil {
		log.Printf("[vault] sync: failed to read certificate at %s: %v", path, err)
		return
	}
	if secret == nil || secret.Data == nil {
		return
	}

	c.mu.Lock()
	defer c.mu.Unlock()

	cert, ok := c.certs[path]
	if !ok {
		return
	}

	if nb, exists := secret.Data["not_before"]; exists {
		if ts, tsOk := nb.(string); tsOk {
			if t, parseErr := time.Parse(time.RFC3339Nano, ts); parseErr == nil {
				cert.NotBefore = t
			}
		}
	}
	if na, exists := secret.Data["not_after"]; exists {
		if ts, tsOk := na.(string); tsOk {
			if t, parseErr := time.Parse(time.RFC3339Nano, ts); parseErr == nil {
				cert.NotAfter = t
				cert.DaysToExpiry = int(time.Until(t).Hours() / 24)
			}
		}
	}
}

// MonitorSecretRotation checks if secrets are within their rotation window.
// When a Vault client is available, it also fetches metadata from Vault to
// determine the actual last rotation time.
func (c *Client) MonitorSecretRotation(ctx context.Context, paths []string, maxAge time.Duration) []SecretMetadata {
	var dueForRotation []SecretMetadata

	// If Vault client is available, refresh metadata for requested paths.
	if c.vaultClient != nil {
		for _, path := range paths {
			metaSecret, err := c.vaultClient.Logical().ReadWithContext(ctx, "secret/metadata/"+path)
			if err != nil {
				log.Printf("[vault] monitor rotation: failed to read metadata for %s: %v", path, err)
				continue
			}
			if metaSecret == nil {
				continue
			}

			c.mu.Lock()
			meta, ok := c.secrets[path]
			if ok {
				if createdRaw, exists := metaSecret.Data["created_time"]; exists {
					if ts, tsOk := createdRaw.(string); tsOk {
						if t, parseErr := time.Parse(time.RFC3339Nano, ts); parseErr == nil {
							meta.LastRotated = t
						}
					}
				}
				if versionRaw, exists := metaSecret.Data["current_version"]; exists {
					if vNum, numOk := versionRaw.(json.Number); numOk {
						if v, convErr := vNum.Int64(); convErr == nil {
							meta.Version = int(v)
						}
					}
				}
			}
			c.mu.Unlock()
		}
	}

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

// MonitorCertificates checks certificate expiry. When a Vault client is
// available, it reads PKI cert data from Vault for each tracked certificate
// path before evaluating expiry.
func (c *Client) MonitorCertificates(ctx context.Context, warningDays int) []CertificateInfo {
	// If Vault client is available, refresh certificate data from Vault.
	if c.vaultClient != nil {
		c.mu.RLock()
		certPaths := make([]string, 0, len(c.certs))
		for p := range c.certs {
			certPaths = append(certPaths, p)
		}
		c.mu.RUnlock()

		for _, path := range certPaths {
			c.syncCertificate(ctx, path)
		}
	}

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
