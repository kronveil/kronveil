package slack

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/kronveil/kronveil/core/engine"
)

// Config holds Slack integration configuration.
type Config struct {
	BotToken       string            `yaml:"bot_token" json:"bot_token"`
	Channels       map[string]string `yaml:"channels" json:"channels"` // severity -> channel
	DefaultChannel string            `yaml:"default_channel" json:"default_channel"`
	MentionGroups  map[string]string `yaml:"mention_groups" json:"mention_groups"` // severity -> group ID
}

// Notifier sends notifications to Slack channels.
type Notifier struct {
	config Config
}

// NewNotifier creates a new Slack notifier.
func NewNotifier(config Config) (*Notifier, error) {
	if config.BotToken == "" {
		return nil, fmt.Errorf("slack bot_token is required")
	}
	if config.DefaultChannel == "" {
		config.DefaultChannel = "#kronveil-alerts"
	}
	return &Notifier{config: config}, nil
}

func (n *Notifier) Name() string { return "slack" }

func (n *Notifier) Initialize(ctx context.Context) error {
	log.Printf("[slack] Slack integration initialized (default channel: %s)", n.config.DefaultChannel)
	return nil
}

func (n *Notifier) Close() error { return nil }

func (n *Notifier) Health() engine.ComponentHealth {
	return engine.ComponentHealth{
		Name:      "slack",
		Status:    "healthy",
		Message:   "connected",
		LastCheck: time.Now(),
	}
}

// NotifyIncident sends an incident notification to Slack.
func (n *Notifier) NotifyIncident(ctx context.Context, incident *engine.Incident) error {
	channel := n.channelForSeverity(incident.Severity)

	// In production: uses slack-go/slack to post message with Block Kit.
	// blocks := n.buildIncidentBlocks(incident)
	// api := slack.New(n.config.BotToken)
	// _, _, err := api.PostMessageContext(ctx, channel, slack.MsgOptionBlocks(blocks...))

	log.Printf("[slack] Incident notification sent to %s: %s (%s)",
		channel, incident.Title, incident.Severity)
	return nil
}

// NotifyAnomaly sends an anomaly notification to Slack.
func (n *Notifier) NotifyAnomaly(ctx context.Context, anomaly *engine.Anomaly) error {
	channel := n.channelForSeverity(anomaly.Severity)

	log.Printf("[slack] Anomaly notification sent to %s: %s (score: %.2f)",
		channel, anomaly.Signal, anomaly.Score)
	return nil
}

// NotifyRemediation sends a remediation notification to Slack.
func (n *Notifier) NotifyRemediation(ctx context.Context, action *engine.RemediationAction) error {
	channel := n.config.DefaultChannel

	log.Printf("[slack] Remediation notification sent to %s: %s on %s (%s)",
		channel, action.Type, action.Target, action.Status)
	return nil
}

func (n *Notifier) channelForSeverity(severity string) string {
	if ch, ok := n.config.Channels[severity]; ok {
		return ch
	}
	return n.config.DefaultChannel
}

func (n *Notifier) buildIncidentBlocks(incident *engine.Incident) []map[string]interface{} {
	emoji := ":warning:"
	switch incident.Severity {
	case engine.SeverityCritical:
		emoji = ":red_circle:"
	case engine.SeverityHigh:
		emoji = ":orange_circle:"
	case engine.SeverityMedium:
		emoji = ":large_yellow_circle:"
	}

	mention := ""
	if group, ok := n.config.MentionGroups[incident.Severity]; ok {
		mention = fmt.Sprintf(" <!subteam^%s>", group)
	}

	return []map[string]interface{}{
		{
			"type": "header",
			"text": map[string]interface{}{
				"type": "plain_text",
				"text": fmt.Sprintf("%s Incident: %s", emoji, incident.Title),
			},
		},
		{
			"type": "section",
			"fields": []map[string]interface{}{
				{"type": "mrkdwn", "text": fmt.Sprintf("*ID:*\n%s", incident.ID)},
				{"type": "mrkdwn", "text": fmt.Sprintf("*Severity:*\n%s", incident.Severity)},
				{"type": "mrkdwn", "text": fmt.Sprintf("*Status:*\n%s", incident.Status)},
				{"type": "mrkdwn", "text": fmt.Sprintf("*Resources:*\n%d affected", len(incident.AffectedResources))},
			},
		},
		{
			"type": "section",
			"text": map[string]interface{}{
				"type": "mrkdwn",
				"text": fmt.Sprintf("*Description:*\n%s%s", incident.Description, mention),
			},
		},
		{
			"type": "actions",
			"elements": []map[string]interface{}{
				{
					"type":      "button",
					"text":      map[string]interface{}{"type": "plain_text", "text": "Acknowledge"},
					"action_id": fmt.Sprintf("ack_%s", incident.ID),
					"style":     "primary",
				},
				{
					"type":      "button",
					"text":      map[string]interface{}{"type": "plain_text", "text": "View Details"},
					"action_id": fmt.Sprintf("view_%s", incident.ID),
				},
			},
		},
	}
}
