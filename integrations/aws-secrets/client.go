// Package awssecrets provides AWS Secrets Manager integration.
package awssecrets

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"sync"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	awsconfig "github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/secretsmanager"
	smtypes "github.com/aws/aws-sdk-go-v2/service/secretsmanager/types"
	"github.com/kronveil/kronveil/core/engine"
)

// Config holds AWS Secrets Manager configuration.
type Config struct {
	Region          string        `yaml:"region" json:"region"`
	SecretPrefix    string        `yaml:"secret_prefix" json:"secret_prefix"`
	RotationWindow  time.Duration `yaml:"rotation_window" json:"rotation_window"`
	PollInterval    time.Duration `yaml:"poll_interval" json:"poll_interval"`
	CacheEnabled    bool          `yaml:"cache_enabled" json:"cache_enabled"`
	CacheTTL        time.Duration `yaml:"cache_ttl" json:"cache_ttl"`
}

// Client integrates with AWS Secrets Manager for secret retrieval and rotation monitoring.
type Client struct {
	config   Config
	smClient *secretsmanager.Client
	mu       sync.RWMutex
	secrets  map[string]*SecretEntry
	cache    map[string]*cacheEntry
	lastErr  error
	running  bool
	cancel   context.CancelFunc
	wg       sync.WaitGroup
}

type cacheEntry struct {
	data      map[string]interface{}
	fetchedAt time.Time
}

// SecretEntry tracks a monitored secret in AWS Secrets Manager.
type SecretEntry struct {
	ARN              string     `json:"arn"`
	Name             string     `json:"name"`
	VersionID        string     `json:"version_id"`
	LastAccessed     time.Time  `json:"last_accessed"`
	LastRotated      time.Time  `json:"last_rotated"`
	NextRotation     *time.Time `json:"next_rotation,omitempty"`
	RotationEnabled  bool       `json:"rotation_enabled"`
	RotationDue      bool       `json:"rotation_due"`
	RotationLambdaARN string   `json:"rotation_lambda_arn,omitempty"`
}

// NewClient creates a new AWS Secrets Manager client.
func NewClient(config Config) (*Client, error) {
	if config.Region == "" {
		return nil, fmt.Errorf("aws region is required")
	}
	if config.PollInterval == 0 {
		config.PollInterval = 5 * time.Minute
	}
	if config.CacheTTL == 0 {
		config.CacheTTL = 5 * time.Minute
	}

	c := &Client{
		config:  config,
		secrets: make(map[string]*SecretEntry),
		cache:   make(map[string]*cacheEntry),
	}

	cfg, err := awsconfig.LoadDefaultConfig(context.Background(),
		awsconfig.WithRegion(config.Region),
	)
	if err != nil {
		log.Printf("[aws-secrets] WARNING: failed to load AWS config: %v (running in degraded mode)", err)
	} else {
		c.smClient = secretsmanager.NewFromConfig(cfg)
	}

	return c, nil
}

func (c *Client) Name() string { return "aws-secrets-manager" }

func (c *Client) Initialize(ctx context.Context) error {
	if c.smClient == nil {
		log.Printf("[aws-secrets] AWS Secrets Manager initialized in degraded mode (no credentials)")
		return nil
	}
	log.Printf("[aws-secrets] AWS Secrets Manager initialized (region: %s, prefix: %s)",
		c.config.Region, c.config.SecretPrefix)
	return nil
}

func (c *Client) Start(ctx context.Context) error {
	c.mu.Lock()
	if c.running {
		c.mu.Unlock()
		return fmt.Errorf("aws secrets client already running")
	}
	c.running = true
	var childCtx context.Context
	childCtx, c.cancel = context.WithCancel(ctx)
	c.mu.Unlock()

	c.wg.Add(1)
	go c.monitorRotation(childCtx)

	log.Printf("[aws-secrets] Secret rotation monitoring started (poll: %s)", c.config.PollInterval)
	return nil
}

func (c *Client) Stop() error {
	c.mu.Lock()
	defer c.mu.Unlock()
	if !c.running {
		return nil
	}
	c.running = false
	if c.cancel != nil {
		c.cancel()
	}
	c.wg.Wait()
	log.Println("[aws-secrets] AWS Secrets Manager client stopped")
	return nil
}

func (c *Client) Close() error {
	return c.Stop()
}

func (c *Client) Health() engine.ComponentHealth {
	c.mu.RLock()
	defer c.mu.RUnlock()
	status := "healthy"
	msg := fmt.Sprintf("monitoring %d secrets in %s", len(c.secrets), c.config.Region)
	if c.smClient == nil {
		status = "degraded"
		msg = "no AWS credentials configured"
	} else if c.lastErr != nil {
		status = "degraded"
		msg = c.lastErr.Error()
	}
	return engine.ComponentHealth{
		Name:      "aws-secrets-manager",
		Status:    status,
		Message:   msg,
		LastCheck: time.Now(),
	}
}

// GetSecret retrieves a secret value from AWS Secrets Manager.
func (c *Client) GetSecret(ctx context.Context, secretName string) (map[string]interface{}, error) {
	if c.smClient == nil {
		return nil, fmt.Errorf("aws secrets manager client not configured")
	}

	// Check cache first.
	if c.config.CacheEnabled {
		c.mu.RLock()
		if entry, ok := c.cache[secretName]; ok && time.Since(entry.fetchedAt) < c.config.CacheTTL {
			c.mu.RUnlock()
			return entry.data, nil
		}
		c.mu.RUnlock()
	}

	output, err := c.smClient.GetSecretValue(ctx, &secretsmanager.GetSecretValueInput{
		SecretId: aws.String(secretName),
	})
	if err != nil {
		c.mu.Lock()
		c.lastErr = err
		c.mu.Unlock()
		return nil, fmt.Errorf("failed to get secret %s: %w", secretName, err)
	}

	var result map[string]interface{}
	if output.SecretString != nil {
		if err := json.Unmarshal([]byte(*output.SecretString), &result); err != nil {
			result = map[string]interface{}{"value": *output.SecretString}
		}
	}

	// Update cache.
	if c.config.CacheEnabled {
		c.mu.Lock()
		c.cache[secretName] = &cacheEntry{data: result, fetchedAt: time.Now()}
		c.mu.Unlock()
	}

	return result, nil
}

// ListSecrets lists secrets matching the configured prefix.
func (c *Client) ListSecrets(ctx context.Context) ([]*SecretEntry, error) {
	if c.smClient == nil {
		// Fallback to locally tracked secrets.
		c.mu.RLock()
		defer c.mu.RUnlock()
		result := make([]*SecretEntry, 0, len(c.secrets))
		for _, s := range c.secrets {
			result = append(result, s)
		}
		return result, nil
	}

	input := &secretsmanager.ListSecretsInput{}
	if c.config.SecretPrefix != "" {
		input.Filters = []smtypes.Filter{
			{Key: smtypes.FilterNameStringTypeName, Values: []string{c.config.SecretPrefix}},
		}
	}

	var result []*SecretEntry
	paginator := secretsmanager.NewListSecretsPaginator(c.smClient, input)
	for paginator.HasMorePages() {
		page, err := paginator.NextPage(ctx)
		if err != nil {
			c.mu.Lock()
			c.lastErr = err
			c.mu.Unlock()
			return nil, fmt.Errorf("failed to list secrets: %w", err)
		}
		for _, s := range page.SecretList {
			entry := &SecretEntry{
				Name:            aws.ToString(s.Name),
				ARN:             aws.ToString(s.ARN),
				RotationEnabled: aws.ToBool(s.RotationEnabled),
			}
			if s.LastRotatedDate != nil {
				entry.LastRotated = *s.LastRotatedDate
			}
			if s.LastAccessedDate != nil {
				entry.LastAccessed = *s.LastAccessedDate
			}
			if s.NextRotationDate != nil {
				entry.NextRotation = s.NextRotationDate
			}
			result = append(result, entry)
		}
	}
	return result, nil
}

// TrackSecret adds a secret to rotation monitoring.
func (c *Client) TrackSecret(name string, entry SecretEntry) {
	c.mu.Lock()
	defer c.mu.Unlock()
	entry.Name = name
	c.secrets[name] = &entry
}

// CheckRotation checks if a secret needs rotation based on the configured window.
func (c *Client) CheckRotation(ctx context.Context) []SecretEntry {
	c.mu.RLock()
	defer c.mu.RUnlock()

	var dueForRotation []SecretEntry
	for _, entry := range c.secrets {
		if c.config.RotationWindow > 0 && time.Since(entry.LastRotated) > c.config.RotationWindow {
			entry.RotationDue = true
			dueForRotation = append(dueForRotation, *entry)
		}
	}
	return dueForRotation
}

// TriggerRotation initiates secret rotation via the configured Lambda function.
func (c *Client) TriggerRotation(ctx context.Context, secretName string) error {
	if c.smClient == nil {
		return fmt.Errorf("aws secrets manager client not configured")
	}

	c.mu.RLock()
	entry, ok := c.secrets[secretName]
	c.mu.RUnlock()

	if !ok {
		return fmt.Errorf("secret %s not tracked", secretName)
	}
	if entry.RotationLambdaARN == "" {
		return fmt.Errorf("no rotation lambda configured for %s", secretName)
	}

	_, err := c.smClient.RotateSecret(ctx, &secretsmanager.RotateSecretInput{
		SecretId: aws.String(secretName),
	})
	if err != nil {
		return fmt.Errorf("failed to trigger rotation for %s: %w", secretName, err)
	}

	log.Printf("[aws-secrets] Triggered rotation for secret: %s", secretName)
	return nil
}

func (c *Client) monitorRotation(ctx context.Context) {
	defer c.wg.Done()
	ticker := time.NewTicker(c.config.PollInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			due := c.CheckRotation(ctx)
			if len(due) > 0 {
				log.Printf("[aws-secrets] %d secrets due for rotation", len(due))
			}
		}
	}
}
