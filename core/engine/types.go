package engine

import (
	"context"
	"time"
)

// Severity levels for events and incidents.
const (
	SeverityCritical = "critical"
	SeverityHigh     = "high"
	SeverityMedium   = "medium"
	SeverityLow      = "low"
	SeverityInfo     = "info"
)

// Incident statuses.
const (
	StatusActive       = "active"
	StatusAcknowledged = "acknowledged"
	StatusResolved     = "resolved"
)

// Remediation action types.
const (
	RemediationRestartPod       = "restart_pod"
	RemediationScaleDeployment  = "scale_deployment"
	RemediationRollback         = "rollback"
	RemediationDrainNode        = "drain_node"
	RemediationCordonNode       = "cordon_node"
	RemediationDeletePod        = "delete_pod"
)

// TelemetryEvent represents a single telemetry data point collected from infrastructure.
type TelemetryEvent struct {
	ID        string                 `json:"id"`
	Source    string                 `json:"source"`
	Type      string                 `json:"type"`
	Timestamp time.Time              `json:"timestamp"`
	Payload   map[string]interface{} `json:"payload"`
	Metadata  map[string]string      `json:"metadata"`
	Severity  string                 `json:"severity"`
	Cluster   string                 `json:"cluster,omitempty"`
	Namespace string                 `json:"namespace,omitempty"`
}

// Incident represents a detected operational incident.
type Incident struct {
	ID                string            `json:"id"`
	Title             string            `json:"title"`
	Description       string            `json:"description"`
	Severity          string            `json:"severity"`
	Status            string            `json:"status"`
	RootCause         string            `json:"root_cause,omitempty"`
	Timeline          []TimelineEntry   `json:"timeline"`
	AffectedResources []string          `json:"affected_resources"`
	Labels            map[string]string `json:"labels,omitempty"`
	CreatedAt         time.Time         `json:"created_at"`
	AcknowledgedAt    *time.Time        `json:"acknowledged_at,omitempty"`
	ResolvedAt        *time.Time        `json:"resolved_at,omitempty"`
	MTTR              *time.Duration    `json:"mttr,omitempty"`
	CorrelatedEvents  []string          `json:"correlated_events,omitempty"`
}

// TimelineEntry represents a single entry in an incident timeline.
type TimelineEntry struct {
	Timestamp time.Time `json:"timestamp"`
	Action    string    `json:"action"`
	Details   string    `json:"details"`
	Actor     string    `json:"actor"` // "system", "user", "ai"
}

// Anomaly represents a detected anomaly in a monitored signal.
type Anomaly struct {
	ID          string    `json:"id"`
	Signal      string    `json:"signal"`
	Score       float64   `json:"score"`
	Timestamp   time.Time `json:"timestamp"`
	Predicted   bool      `json:"predicted"`
	Description string    `json:"description"`
	Source      string    `json:"source"`
	Severity    string    `json:"severity"`
	Values      []float64 `json:"values,omitempty"`
	Threshold   float64   `json:"threshold,omitempty"`
}

// RemediationAction represents an automated remediation action.
type RemediationAction struct {
	ID         string            `json:"id"`
	IncidentID string            `json:"incident_id"`
	Type       string            `json:"type"`
	Target     string            `json:"target"`
	Parameters map[string]string `json:"parameters"`
	Status     string            `json:"status"`
	DryRun     bool              `json:"dry_run"`
	Result     string            `json:"result,omitempty"`
	CreatedAt  time.Time         `json:"created_at"`
	ExecutedAt *time.Time        `json:"executed_at,omitempty"`
}

// PolicyViolation represents a policy check failure.
type PolicyViolation struct {
	ID        string    `json:"id"`
	Policy    string    `json:"policy"`
	Resource  string    `json:"resource"`
	Violation string    `json:"violation"`
	Severity  string    `json:"severity"`
	Timestamp time.Time `json:"timestamp"`
}

// HealthStatus represents the overall health of the agent.
type HealthStatus struct {
	Status     string            `json:"status"`
	Components []ComponentHealth `json:"components"`
	Uptime     time.Duration     `json:"uptime"`
	Version    string            `json:"version"`
}

// ComponentHealth represents the health of a single component.
type ComponentHealth struct {
	Name      string    `json:"name"`
	Status    string    `json:"status"`
	Message   string    `json:"message"`
	LastCheck time.Time `json:"last_check"`
}

// Collector is the interface that all telemetry collectors must implement.
type Collector interface {
	Name() string
	Start(ctx context.Context) error
	Stop() error
	Events() <-chan *TelemetryEvent
	Health() ComponentHealth
}

// IntelligenceModule is the interface for AI/ML analysis modules.
type IntelligenceModule interface {
	Name() string
	Analyze(ctx context.Context, event *TelemetryEvent) error
	Start(ctx context.Context) error
	Stop() error
	Health() ComponentHealth
}

// Integration is the interface for external service integrations.
type Integration interface {
	Name() string
	Initialize(ctx context.Context) error
	Close() error
	Health() ComponentHealth
}

// Notifier can send notifications about incidents and anomalies.
type Notifier interface {
	NotifyIncident(ctx context.Context, incident *Incident) error
	NotifyAnomaly(ctx context.Context, anomaly *Anomaly) error
	NotifyRemediation(ctx context.Context, action *RemediationAction) error
}

// LLMProvider provides LLM inference capabilities.
type LLMProvider interface {
	Invoke(ctx context.Context, prompt string) (string, error)
	InvokeWithSystem(ctx context.Context, system, prompt string) (string, error)
}

// EventPublisher publishes events to the event bus.
type EventPublisher interface {
	Publish(ctx context.Context, topic string, event *TelemetryEvent) error
}

// EventSubscriber subscribes to events from the event bus.
type EventSubscriber interface {
	Subscribe(ctx context.Context, topic string, handler func(*TelemetryEvent)) error
	Unsubscribe(topic string) error
}

// MetricsRecorder abstracts metrics recording across backends (Prometheus, OpenTelemetry, etc.).
type MetricsRecorder interface {
	RecordEvent(source string)
	RecordCollectorError(source string)
	RecordAnomaly()
	RecordIncidentCreated()
	RecordIncidentResolved()
	RecordRemediation(success bool)
	SetMTTR(seconds float64)
	RecordPolicyEvaluation()
	RecordPolicyViolation()
	SetComponentHealth(healthy, degraded, critical int)
	SetUptime(seconds float64)
}

// NoopMetricsRecorder is a no-op implementation used when no metrics backends are enabled.
type NoopMetricsRecorder struct{}

func (n *NoopMetricsRecorder) RecordEvent(string)                     {}
func (n *NoopMetricsRecorder) RecordCollectorError(string)            {}
func (n *NoopMetricsRecorder) RecordAnomaly()                         {}
func (n *NoopMetricsRecorder) RecordIncidentCreated()                 {}
func (n *NoopMetricsRecorder) RecordIncidentResolved()                {}
func (n *NoopMetricsRecorder) RecordRemediation(bool)                 {}
func (n *NoopMetricsRecorder) SetMTTR(float64)                        {}
func (n *NoopMetricsRecorder) RecordPolicyEvaluation()                {}
func (n *NoopMetricsRecorder) RecordPolicyViolation()                 {}
func (n *NoopMetricsRecorder) SetComponentHealth(int, int, int)       {}
func (n *NoopMetricsRecorder) SetUptime(float64)                      {}
