// Package pagerduty provides PagerDuty incident alerting integration.
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

	component := "unknown"
	if len(incident.AffectedResources) > 0 {
		component = incident.AffectedResources[0]
	}

	payload := map[string]interface{}{
		"routing_key":  c.config.RoutingKey,
		"event_action": "trigger",
		"dedup_key":    dedupKey,
		"payload": map[string]interface{}{
			"summary":    fmt.Sprintf("[Kronveil] %s", incident.Title),
			"severity":   severity,
			"source":     "kronveil-agent",
			"component":  component,
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
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode >= 400 {
		return fmt.Errorf("PagerDuty API error: %d", resp.StatusCode)
	}

	action := payload["event_action"].(string)
	log.Printf("[pagerduty] Event sent: %s (dedup: %s)", action, payload["dedup_key"])
	return nil
}

// NotifyIncident implements engine.Notifier — triggers a PagerDuty event for an incident.
func (c *Client) NotifyIncident(ctx context.Context, incident *engine.Incident) error {
	return c.TriggerIncident(ctx, incident)
}

// NotifyAnomaly implements engine.Notifier — triggers a PagerDuty event for an anomaly.
func (c *Client) NotifyAnomaly(ctx context.Context, a *engine.Anomaly) error {
	severity := mapSeverity(a.Severity)
	payload := map[string]interface{}{
		"routing_key":  c.config.RoutingKey,
		"event_action": "trigger",
		"dedup_key":    fmt.Sprintf("kronveil-anomaly-%s", a.ID),
		"payload": map[string]interface{}{
			"summary":  fmt.Sprintf("[Kronveil Anomaly] %s (score: %.2f)", a.Signal, a.Score),
			"severity": severity,
			"source":   "kronveil-agent",
			"group":    "anomaly-detection",
			"class":    "anomaly",
			"custom_details": map[string]interface{}{
				"anomaly_id":  a.ID,
				"signal":      a.Signal,
				"score":       a.Score,
				"description": a.Description,
				"source":      a.Source,
			},
		},
	}
	return c.sendEvent(ctx, payload)
}

// NotifyRemediation implements engine.Notifier — triggers a PagerDuty event for a remediation action.
func (c *Client) NotifyRemediation(ctx context.Context, action *engine.RemediationAction) error {
	payload := map[string]interface{}{
		"routing_key":  c.config.RoutingKey,
		"event_action": "trigger",
		"dedup_key":    fmt.Sprintf("kronveil-remediation-%s", action.ID),
		"payload": map[string]interface{}{
			"summary":  fmt.Sprintf("[Kronveil Remediation] %s on %s (%s)", action.Type, action.Target, action.Status),
			"severity": "info",
			"source":   "kronveil-agent",
			"group":    "remediation",
			"class":    "remediation",
			"custom_details": map[string]interface{}{
				"action_id":   action.ID,
				"incident_id": action.IncidentID,
				"type":        action.Type,
				"target":      action.Target,
				"status":      action.Status,
				"dry_run":     action.DryRun,
				"result":      action.Result,
			},
		},
	}
	return c.sendEvent(ctx, payload)
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
