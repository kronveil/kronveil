// Package bedrock provides AWS Bedrock LLM integration for root cause analysis.
package bedrock

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"sync/atomic"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	awsconfig "github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/bedrockruntime"
)

// Config holds AWS Bedrock configuration.
type Config struct {
	Region      string  `yaml:"region" json:"region"`
	ModelID     string  `yaml:"model_id" json:"model_id"`
	MaxTokens   int     `yaml:"max_tokens" json:"max_tokens"`
	Temperature float64 `yaml:"temperature" json:"temperature"`
	MaxRetries  int     `yaml:"max_retries" json:"max_retries"`
}

// DefaultConfig returns default Bedrock configuration.
func DefaultConfig() Config {
	return Config{
		Region:      "us-east-1",
		ModelID:     "anthropic.claude-3-sonnet-20240229-v1:0",
		MaxTokens:   2048,
		Temperature: 0.3,
		MaxRetries:  3,
	}
}

// Client provides LLM inference capabilities via AWS Bedrock.
type Client struct {
	config      Config
	brClient    *bedrockruntime.Client
	totalTokens int64
	totalCalls  int64
}

// NewClient creates a new Bedrock LLM client.
func NewClient(config Config) (*Client, error) {
	if config.Region == "" {
		return nil, fmt.Errorf("bedrock region is required")
	}
	if config.ModelID == "" {
		config.ModelID = DefaultConfig().ModelID
	}
	if config.MaxTokens == 0 {
		config.MaxTokens = 2048
	}

	c := &Client{config: config}

	cfg, err := awsconfig.LoadDefaultConfig(context.Background(),
		awsconfig.WithRegion(config.Region),
	)
	if err != nil {
		log.Printf("[bedrock] WARNING: failed to load AWS config: %v (running in degraded mode)", err)
	} else {
		c.brClient = bedrockruntime.NewFromConfig(cfg)
	}

	log.Printf("[bedrock] Initialized Bedrock client (region: %s, model: %s)",
		config.Region, config.ModelID)
	return c, nil
}

func (c *Client) Name() string { return "aws-bedrock" }

func (c *Client) Initialize(ctx context.Context) error {
	// In production: validates AWS credentials and tests connectivity.
	log.Println("[bedrock] AWS Bedrock integration initialized")
	return nil
}

func (c *Client) Close() error {
	log.Printf("[bedrock] Closed (total calls: %d, total tokens: %d)",
		atomic.LoadInt64(&c.totalCalls), atomic.LoadInt64(&c.totalTokens))
	return nil
}

func (c *Client) Health() struct {
	Name      string    `json:"name"`
	Status    string    `json:"status"`
	Message   string    `json:"message"`
	LastCheck time.Time `json:"last_check"`
} {
	status := "healthy"
	msg := fmt.Sprintf("model: %s, calls: %d", c.config.ModelID, atomic.LoadInt64(&c.totalCalls))
	if c.brClient == nil {
		status = "degraded"
		msg = "no AWS credentials configured"
	}
	return struct {
		Name      string    `json:"name"`
		Status    string    `json:"status"`
		Message   string    `json:"message"`
		LastCheck time.Time `json:"last_check"`
	}{
		Name:      "aws-bedrock",
		Status:    status,
		Message:   msg,
		LastCheck: time.Now(),
	}
}

// Invoke sends a prompt to the Bedrock LLM and returns the response.
func (c *Client) Invoke(ctx context.Context, prompt string) (string, error) {
	return c.InvokeWithSystem(ctx, "", prompt)
}

// InvokeWithSystem sends a prompt with a system message to the Bedrock LLM.
func (c *Client) InvokeWithSystem(ctx context.Context, system, prompt string) (string, error) {
	var lastErr error

	for attempt := 0; attempt <= c.config.MaxRetries; attempt++ {
		if attempt > 0 {
			backoff := time.Duration(1<<uint(attempt-1)) * time.Second
			select {
			case <-ctx.Done():
				return "", ctx.Err()
			case <-time.After(backoff):
			}
		}

		response, err := c.invokeModel(ctx, system, prompt)
		if err != nil {
			lastErr = err
			log.Printf("[bedrock] Attempt %d failed: %v", attempt+1, err)
			continue
		}

		atomic.AddInt64(&c.totalCalls, 1)
		atomic.AddInt64(&c.totalTokens, int64(len(prompt)/4+len(response)/4))
		return response, nil
	}

	return "", fmt.Errorf("bedrock invocation failed after %d retries: %w", c.config.MaxRetries, lastErr)
}

func (c *Client) invokeModel(ctx context.Context, system, prompt string) (string, error) {
	if c.brClient == nil {
		return "", fmt.Errorf("bedrock client not configured (no AWS credentials)")
	}

	reqBody := map[string]interface{}{
		"anthropic_version": "bedrock-2023-05-31",
		"max_tokens":        c.config.MaxTokens,
		"temperature":       c.config.Temperature,
		"messages": []map[string]interface{}{
			{"role": "user", "content": prompt},
		},
	}
	if system != "" {
		reqBody["system"] = system
	}

	jsonBody, err := json.Marshal(reqBody)
	if err != nil {
		return "", fmt.Errorf("failed to marshal request: %w", err)
	}

	output, err := c.brClient.InvokeModel(ctx, &bedrockruntime.InvokeModelInput{
		ModelId:     &c.config.ModelID,
		Body:        jsonBody,
		ContentType: aws.String("application/json"),
	})
	if err != nil {
		return "", fmt.Errorf("bedrock invoke failed: %w", err)
	}

	var resp struct {
		Content []struct {
			Text string `json:"text"`
		} `json:"content"`
	}
	if err := json.Unmarshal(output.Body, &resp); err != nil {
		return "", fmt.Errorf("failed to parse bedrock response: %w", err)
	}
	if len(resp.Content) == 0 {
		return "", fmt.Errorf("empty response from bedrock")
	}

	return resp.Content[0].Text, nil
}

// TokenUsage returns the total token usage.
func (c *Client) TokenUsage() int64 {
	return atomic.LoadInt64(&c.totalTokens)
}

// CallCount returns the total number of API calls.
func (c *Client) CallCount() int64 {
	return atomic.LoadInt64(&c.totalCalls)
}
