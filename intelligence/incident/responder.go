package incident

import (
	"context"
	"fmt"
	"log"
	"sync"
	"time"

	"github.com/kronveil/kronveil/core/engine"
)

// Config holds incident responder configuration.
type Config struct {
	CorrelationWindow time.Duration `yaml:"correlation_window" json:"correlation_window"`
	AutoRemediate     bool          `yaml:"auto_remediate" json:"auto_remediate"`
	EscalationDelay   time.Duration `yaml:"escalation_delay" json:"escalation_delay"`
	MaxRetries        int           `yaml:"max_retries" json:"max_retries"`
	DryRun            bool          `yaml:"dry_run" json:"dry_run"`
}

// DefaultConfig returns default incident responder configuration.
func DefaultConfig() Config {
	return Config{
		CorrelationWindow: 5 * time.Minute,
		AutoRemediate:     true,
		EscalationDelay:   10 * time.Minute,
		MaxRetries:        3,
		DryRun:            false,
	}
}

// Responder manages incident lifecycle and auto-remediation.
type Responder struct {
	config     Config
	mu         sync.RWMutex
	incidents  map[string]*engine.Incident
	notifiers  []engine.Notifier
	llm        engine.LLMProvider
	metrics    engine.MetricsRecorder
	running    bool
	cancel     context.CancelFunc
	incidentCh chan *engine.Incident
	counter    int64
}

// SetMetrics sets the metrics recorder for the responder.
func (r *Responder) SetMetrics(m engine.MetricsRecorder) {
	r.metrics = m
}

// New creates a new incident responder.
func New(config Config, notifiers []engine.Notifier, llm engine.LLMProvider) *Responder {
	return &Responder{
		config:     config,
		incidents:  make(map[string]*engine.Incident),
		notifiers:  notifiers,
		llm:        llm,
		incidentCh: make(chan *engine.Incident, 50),
	}
}

func (r *Responder) Name() string { return "incident-responder" }

func (r *Responder) Start(ctx context.Context) error {
	r.mu.Lock()
	r.running = true
	_, r.cancel = context.WithCancel(ctx)
	r.mu.Unlock()

	log.Printf("[incident] Incident responder started (auto-remediate: %v, dry-run: %v)",
		r.config.AutoRemediate, r.config.DryRun)
	return nil
}

func (r *Responder) Stop() error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.running = false
	if r.cancel != nil {
		r.cancel()
	}
	close(r.incidentCh)
	return nil
}

// Analyze processes telemetry events for incident detection.
func (r *Responder) Analyze(ctx context.Context, event *engine.TelemetryEvent) error {
	if event.Severity != engine.SeverityCritical && event.Severity != engine.SeverityHigh {
		return nil
	}

	// Check if this event correlates with an existing incident.
	r.mu.RLock()
	for _, inc := range r.incidents {
		if inc.Status != engine.StatusResolved && r.eventsCorrelate(event, inc) {
			r.mu.RUnlock()
			r.addToIncident(inc.ID, event)
			return nil
		}
	}
	r.mu.RUnlock()

	// Create a new incident.
	incident := r.createIncident(ctx, event)

	// Notify.
	for _, n := range r.notifiers {
		if err := n.NotifyIncident(ctx, incident); err != nil {
			log.Printf("[incident] Notification error: %v", err)
		}
	}

	// Auto-remediate if enabled.
	if r.config.AutoRemediate {
		go r.attemptRemediation(ctx, incident)
	}

	return nil
}

func (r *Responder) Health() engine.ComponentHealth {
	r.mu.RLock()
	active := 0
	for _, inc := range r.incidents {
		if inc.Status != engine.StatusResolved {
			active++
		}
	}
	r.mu.RUnlock()

	return engine.ComponentHealth{
		Name:      "incident-responder",
		Status:    "healthy",
		Message:   fmt.Sprintf("%d active incidents, %d total", active, len(r.incidents)),
		LastCheck: time.Now(),
	}
}

func (r *Responder) createIncident(ctx context.Context, event *engine.TelemetryEvent) *engine.Incident {
	r.mu.Lock()
	defer r.mu.Unlock()

	r.counter++
	id := fmt.Sprintf("INC-%04d", r.counter)

	title := fmt.Sprintf("%s event from %s", event.Severity, event.Source)
	if eventType, ok := event.Payload["type"].(string); ok {
		title = eventType
	}

	incident := &engine.Incident{
		ID:          id,
		Title:       title,
		Description: fmt.Sprintf("Detected %s severity event from %s collector", event.Severity, event.Source),
		Severity:    event.Severity,
		Status:      engine.StatusActive,
		AffectedResources: []string{event.Source},
		CreatedAt:   time.Now(),
		Timeline: []engine.TimelineEntry{
			{
				Timestamp: time.Now(),
				Action:    "created",
				Details:   "Incident auto-detected from telemetry event",
				Actor:     "system",
			},
		},
		CorrelatedEvents: []string{event.ID},
	}

	r.incidents[id] = incident
	if r.metrics != nil {
		r.metrics.RecordIncidentCreated()
	}
	log.Printf("[incident] Created incident %s: %s (severity: %s)", id, title, event.Severity)

	select {
	case r.incidentCh <- incident:
	default:
	}

	return incident
}

func (r *Responder) addToIncident(incidentID string, event *engine.TelemetryEvent) {
	r.mu.Lock()
	defer r.mu.Unlock()

	inc, ok := r.incidents[incidentID]
	if !ok {
		return
	}

	inc.CorrelatedEvents = append(inc.CorrelatedEvents, event.ID)
	inc.Timeline = append(inc.Timeline, engine.TimelineEntry{
		Timestamp: time.Now(),
		Action:    "correlated",
		Details:   fmt.Sprintf("Event %s correlated to incident", event.ID),
		Actor:     "system",
	})
}

// AcknowledgeIncident marks an incident as acknowledged.
func (r *Responder) AcknowledgeIncident(id string) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	inc, ok := r.incidents[id]
	if !ok {
		return fmt.Errorf("incident %s not found", id)
	}
	now := time.Now()
	inc.Status = engine.StatusAcknowledged
	inc.AcknowledgedAt = &now
	inc.Timeline = append(inc.Timeline, engine.TimelineEntry{
		Timestamp: now, Action: "acknowledged", Details: "Incident acknowledged", Actor: "user",
	})
	return nil
}

// ResolveIncident marks an incident as resolved.
func (r *Responder) ResolveIncident(id string) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	inc, ok := r.incidents[id]
	if !ok {
		return fmt.Errorf("incident %s not found", id)
	}
	now := time.Now()
	inc.Status = engine.StatusResolved
	inc.ResolvedAt = &now
	mttr := now.Sub(inc.CreatedAt)
	inc.MTTR = &mttr
	inc.Timeline = append(inc.Timeline, engine.TimelineEntry{
		Timestamp: now, Action: "resolved", Details: fmt.Sprintf("Incident resolved (MTTR: %s)", mttr), Actor: "system",
	})
	if r.metrics != nil {
		r.metrics.RecordIncidentResolved()
		r.metrics.SetMTTR(mttr.Seconds())
	}
	return nil
}

// ListIncidents returns all incidents, optionally filtered by status.
func (r *Responder) ListIncidents(status string) []*engine.Incident {
	r.mu.RLock()
	defer r.mu.RUnlock()
	var result []*engine.Incident
	for _, inc := range r.incidents {
		if status == "" || inc.Status == status {
			result = append(result, inc)
		}
	}
	return result
}

// GetIncident returns a specific incident by ID.
func (r *Responder) GetIncident(id string) (*engine.Incident, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	inc, ok := r.incidents[id]
	return inc, ok
}

// Incidents returns the channel of new incidents.
func (r *Responder) Incidents() <-chan *engine.Incident {
	return r.incidentCh
}

func (r *Responder) eventsCorrelate(event *engine.TelemetryEvent, incident *engine.Incident) bool {
	// Correlate if same source and within the correlation window.
	if time.Since(incident.CreatedAt) > r.config.CorrelationWindow {
		return false
	}
	for _, res := range incident.AffectedResources {
		if res == event.Source {
			return true
		}
	}
	return false
}

func (r *Responder) attemptRemediation(ctx context.Context, incident *engine.Incident) {
	strategy := DetermineStrategy(incident)
	if strategy == nil {
		return
	}

	action := &engine.RemediationAction{
		ID:         fmt.Sprintf("rem-%s", incident.ID),
		IncidentID: incident.ID,
		Type:       strategy.ActionType,
		Target:     strategy.Target,
		Parameters: strategy.Parameters,
		Status:     "pending",
		DryRun:     r.config.DryRun,
		CreatedAt:  time.Now(),
	}

	r.mu.Lock()
	incident.Timeline = append(incident.Timeline, engine.TimelineEntry{
		Timestamp: time.Now(),
		Action:    "remediation_started",
		Details:   fmt.Sprintf("Auto-remediation: %s on %s", action.Type, action.Target),
		Actor:     "ai",
	})
	r.mu.Unlock()

	if err := strategy.Execute(ctx, action); err != nil {
		log.Printf("[incident] Remediation failed for %s: %v", incident.ID, err)
		action.Status = "failed"
		action.Result = err.Error()
		if r.metrics != nil {
			r.metrics.RecordRemediation(false)
		}
	} else {
		action.Status = "completed"
		now := time.Now()
		action.ExecutedAt = &now
		if r.metrics != nil {
			r.metrics.RecordRemediation(true)
		}

		// Auto-resolve the incident if remediation succeeds.
		if err := r.ResolveIncident(incident.ID); err != nil {
			log.Printf("[incident] Failed to resolve incident after remediation: %v", err)
		}
	}

	// Notify about remediation.
	for _, n := range r.notifiers {
		if err := n.NotifyRemediation(ctx, action); err != nil {
			log.Printf("[incident] Remediation notification error: %v", err)
		}
	}
}
