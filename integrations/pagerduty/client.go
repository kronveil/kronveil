package pagerduty

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"time"

	"github.com/kronveil/kronveil/core/engine"
)

const eventsAPIURL = "https://events.pagerduty.com/v2/enqueue"

// Config holds PagerDuty configuration.
type Config struct {
	RoutingKey string `yaml:"routing_key" json:"routing_key"`
	ServiceID  string `yaml:"service_id" json:"service_id"`
}

// Client integrates with PagerDuty for incident alerting.
type Client struct {
	config     Config
	httpClient *http.Client
}

// NewClient creates a new PagerDuty client.
func NewClient(config Config) (*Client, error) {
	if config.RoutingKey == "" {
		return nil, fmt.Errorf("pagerduty routing_key is required")
	}
	return &Client{
		config:     config,
		httpClient: &http.Client{Timeout: 10 * time.Second},
	}, nil
}

func (c *Client) Name() string { return "pagerduty" }

func (c *Client) Initialize(ctx context.Context) error {
	log.Println("[pagerduty] PagerDuty integration initialized")
	return nil
}

func (c *Client) Close() error { return nil }

func (c *Client) Health() engine.ComponentHealth {
	return engine.ComponentHealth{
		Name:      "pagerduty",
		Status:    "healthy",
		Message:   "connected",
		LastCheck: time.Now(),
	}
}

// TriggerIncident creates a PagerDuty incident.
func (c *Client) TriggerIncident(ctx context.Context, incident *engine.Incident) error {
	severity := mapSeverity(incident.Severity)
	dedupKey := fmt.Sprintf("kronveil-%s", incident.ID)

	payload := map[string]interface{}{
		"routing_key":  c.config.RoutingKey,
		"event_action": "trigger",
		"dedup_key":    dedupKey,
		"payload": map[string]interface{}{
			"summary":    fmt.Sprintf("[Kronveil] %s", incident.Title),
			"severity":   severity,
			"source":     "kronveil-agent",
			"component":  incident.AffectedResources[0],
			"group":      "infrastructure",
			"class":      "incident",
			"timestamp":  incident.CreatedAt.Format(time.RFC3339),
			"custom_details": map[string]interface{}{
				"incident_id":        incident.ID,
				"description":        incident.Description,
				"root_cause":         incident.RootCause,
				"affected_resources": incident.AffectedResources,
			},
		},
		"links": []map[string]string{
			{
				"href": fmt.Sprintf("https://kronveil.local/incidents/%s", incident.ID),
				"text": "View in Kronveil Dashboard",
			},
		},
	}

	return c.sendEvent(ctx, payload)
}

// AcknowledgeIncident acknowledges a PagerDuty incident.
func (c *Client) AcknowledgeIncident(ctx context.Context, incidentID string) error {
	payload := map[string]interface{}{
		"routing_key":  c.config.RoutingKey,
		"event_action": "acknowledge",
		"dedup_key":    fmt.Sprintf("kronveil-%s", incidentID),
	}
	return c.sendEvent(ctx, payload)
}

// ResolveIncident resolves a PagerDuty incident.
func (c *Client) ResolveIncident(ctx context.Context, incidentID string) error {
	payload := map[string]interface{}{
		"routing_key":  c.config.RoutingKey,
		"event_action": "resolve",
		"dedup_key":    fmt.Sprintf("kronveil-%s", incidentID),
	}
	return c.sendEvent(ctx, payload)
}

func (c *Client) sendEvent(ctx context.Context, payload map[string]interface{}) error {
	body, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("failed to marshal PagerDuty event: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, "POST", eventsAPIURL, bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("failed to create PagerDuty request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("failed to send PagerDuty event: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		return fmt.Errorf("PagerDuty API error: %d", resp.StatusCode)
	}

	action := payload["event_action"].(string)
	log.Printf("[pagerduty] Event sent: %s (dedup: %s)", action, payload["dedup_key"])
	return nil
}

func mapSeverity(kronveilSeverity string) string {
	switch kronveilSeverity {
	case engine.SeverityCritical:
		return "critical"
	case engine.SeverityHigh:
		return "error"
	case engine.SeverityMedium:
		return "warning"
	default:
		return "info"
	}
}
