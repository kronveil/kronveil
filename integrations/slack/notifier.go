// Package slack provides Slack notification integration.
package slack

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/kronveil/kronveil/core/engine"
	slackapi "github.com/slack-go/slack"
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
	api    *slackapi.Client
}

// NewNotifier creates a new Slack notifier.
func NewNotifier(config Config) (*Notifier, error) {
	if config.BotToken == "" {
		return nil, fmt.Errorf("slack bot_token is required")
	}
	if config.DefaultChannel == "" {
		config.DefaultChannel = "#kronveil-alerts"
	}
	n := &Notifier{
		config: config,
		api:    slackapi.New(config.BotToken),
	}
	return n, nil
}

func (n *Notifier) Name() string { return "slack" }

func (n *Notifier) Initialize(ctx context.Context) error {
	if n.api != nil {
		_, err := n.api.AuthTestContext(ctx)
		if err != nil {
			log.Printf("[slack] WARNING: auth test failed: %v (notifications may fail)", err)
		}
	}
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

	if n.api == nil {
		log.Printf("[slack] Incident notification (no client): %s (%s)", incident.Title, incident.Severity)
		return nil
	}

	blocks := n.buildSlackBlocks(incident)
	_, _, err := n.api.PostMessageContext(ctx, channel, slackapi.MsgOptionBlocks(blocks...))
	if err != nil {
		return fmt.Errorf("failed to post incident to slack: %w", err)
	}

	log.Printf("[slack] Incident notification sent to %s: %s (%s)",
		channel, incident.Title, incident.Severity)
	return nil
}

// NotifyAnomaly sends an anomaly notification to Slack.
func (n *Notifier) NotifyAnomaly(ctx context.Context, anomaly *engine.Anomaly) error {
	channel := n.channelForSeverity(anomaly.Severity)

	if n.api == nil {
		log.Printf("[slack] Anomaly notification (no client): %s (score: %.2f)", anomaly.Signal, anomaly.Score)
		return nil
	}

	text := fmt.Sprintf("Anomaly detected: *%s* (score: %.2f, severity: %s, source: %s)",
		anomaly.Signal, anomaly.Score, anomaly.Severity, anomaly.Source)
	_, _, err := n.api.PostMessageContext(ctx, channel, slackapi.MsgOptionText(text, false))
	if err != nil {
		return fmt.Errorf("failed to post anomaly to slack: %w", err)
	}

	log.Printf("[slack] Anomaly notification sent to %s: %s (score: %.2f)",
		channel, anomaly.Signal, anomaly.Score)
	return nil
}

// NotifyRemediation sends a remediation notification to Slack.
func (n *Notifier) NotifyRemediation(ctx context.Context, action *engine.RemediationAction) error {
	channel := n.config.DefaultChannel

	if n.api == nil {
		log.Printf("[slack] Remediation notification (no client): %s on %s (%s)",
			action.Type, action.Target, action.Status)
		return nil
	}

	text := fmt.Sprintf("Remediation *%s* on `%s`: %s", action.Type, action.Target, action.Status)
	_, _, err := n.api.PostMessageContext(ctx, channel, slackapi.MsgOptionText(text, false))
	if err != nil {
		return fmt.Errorf("failed to post remediation to slack: %w", err)
	}

	log.Printf("[slack] Remediation notification sent to %s: %s on %s (%s)",
		channel, action.Type, action.Target, action.Status)
	return nil
}

func (n *Notifier) buildSlackBlocks(incident *engine.Incident) []slackapi.Block {
	emoji := ":warning:"
	switch incident.Severity {
	case engine.SeverityCritical:
		emoji = ":red_circle:"
	case engine.SeverityHigh:
		emoji = ":orange_circle:"
	case engine.SeverityMedium:
		emoji = ":large_yellow_circle:"
	}

	headerText := slackapi.NewTextBlockObject("plain_text", fmt.Sprintf("%s Incident: %s", emoji, incident.Title), true, false)
	header := slackapi.NewHeaderBlock(headerText)

	fields := []*slackapi.TextBlockObject{
		slackapi.NewTextBlockObject("mrkdwn", fmt.Sprintf("*ID:*\n%s", incident.ID), false, false),
		slackapi.NewTextBlockObject("mrkdwn", fmt.Sprintf("*Severity:*\n%s", incident.Severity), false, false),
		slackapi.NewTextBlockObject("mrkdwn", fmt.Sprintf("*Status:*\n%s", incident.Status), false, false),
		slackapi.NewTextBlockObject("mrkdwn", fmt.Sprintf("*Resources:*\n%d affected", len(incident.AffectedResources)), false, false),
	}
	section := slackapi.NewSectionBlock(nil, fields, nil)

	descText := fmt.Sprintf("*Description:*\n%s", incident.Description)
	if group, ok := n.config.MentionGroups[incident.Severity]; ok {
		descText += fmt.Sprintf(" <!subteam^%s>", group)
	}
	descSection := slackapi.NewSectionBlock(slackapi.NewTextBlockObject("mrkdwn", descText, false, false), nil, nil)

	ackBtn := slackapi.NewButtonBlockElement(fmt.Sprintf("ack_%s", incident.ID), "ack",
		slackapi.NewTextBlockObject("plain_text", "Acknowledge", true, false))
	ackBtn.Style = slackapi.StylePrimary
	viewBtn := slackapi.NewButtonBlockElement(fmt.Sprintf("view_%s", incident.ID), "view",
		slackapi.NewTextBlockObject("plain_text", "View Details", true, false))
	actions := slackapi.NewActionBlock("", ackBtn, viewBtn)

	return []slackapi.Block{header, section, descSection, actions}
}

func (n *Notifier) channelForSeverity(severity string) string {
	if ch, ok := n.config.Channels[severity]; ok {
		return ch
	}
	return n.config.DefaultChannel
}

// BuildIncidentBlocks creates Slack Block Kit blocks for an incident notification.
func (n *Notifier) BuildIncidentBlocks(incident *engine.Incident) []map[string]interface{} {
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
