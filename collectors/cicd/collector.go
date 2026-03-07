package cicd

import (
	"context"
	"fmt"
	"log"
	"sync"
	"time"

	"github.com/kronveil/kronveil/core/engine"
)

// Config holds the CI/CD collector configuration.
type Config struct {
	WebhookPort   int      `yaml:"webhook_port" json:"webhook_port"`
	WebhookSecret string   `yaml:"webhook_secret" json:"webhook_secret"`
	RepoFilters   []string `yaml:"repo_filters" json:"repo_filters"`
	PollInterval  time.Duration `yaml:"poll_interval" json:"poll_interval"`
}

// PipelineRun represents a CI/CD pipeline execution.
type PipelineRun struct {
	ID          string            `json:"id"`
	Repo        string            `json:"repo"`
	Branch      string            `json:"branch"`
	Commit      string            `json:"commit"`
	Status      string            `json:"status"` // queued, in_progress, completed, failure
	Conclusion  string            `json:"conclusion,omitempty"`
	StartedAt   time.Time         `json:"started_at"`
	CompletedAt *time.Time        `json:"completed_at,omitempty"`
	Duration    time.Duration     `json:"duration,omitempty"`
	Jobs        []PipelineJob     `json:"jobs"`
	Labels      map[string]string `json:"labels"`
}

// PipelineJob represents a single job within a pipeline.
type PipelineJob struct {
	Name       string        `json:"name"`
	Status     string        `json:"status"`
	StartedAt  time.Time     `json:"started_at"`
	Duration   time.Duration `json:"duration"`
	Steps      []PipelineStep `json:"steps"`
}

// PipelineStep represents a single step within a job.
type PipelineStep struct {
	Name     string        `json:"name"`
	Status   string        `json:"status"`
	Duration time.Duration `json:"duration"`
}

// Collector gathers CI/CD pipeline telemetry.
type Collector struct {
	config  Config
	events  chan *engine.TelemetryEvent
	mu      sync.RWMutex
	running bool
	cancel  context.CancelFunc
	wg      sync.WaitGroup
	runs    map[string]*PipelineRun
}

// New creates a new CI/CD collector.
func New(config Config) *Collector {
	return &Collector{
		config: config,
		events: make(chan *engine.TelemetryEvent, 200),
		runs:   make(map[string]*PipelineRun),
	}
}

func (c *Collector) Name() string { return "cicd" }

func (c *Collector) Start(ctx context.Context) error {
	c.mu.Lock()
	if c.running {
		c.mu.Unlock()
		return fmt.Errorf("cicd collector already running")
	}
	c.running = true
	_, c.cancel = context.WithCancel(ctx)
	c.mu.Unlock()

	log.Printf("[cicd-collector] Starting CI/CD collector (repos: %v)", c.config.RepoFilters)

	c.wg.Add(1)
	go c.poll(ctx)

	return nil
}

func (c *Collector) Stop() error {
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
	close(c.events)
	return nil
}

func (c *Collector) Events() <-chan *engine.TelemetryEvent { return c.events }

func (c *Collector) Health() engine.ComponentHealth {
	return engine.ComponentHealth{
		Name:      "cicd-collector",
		Status:    "healthy",
		Message:   fmt.Sprintf("tracking %d repos", len(c.config.RepoFilters)),
		LastCheck: time.Now(),
	}
}

// HandleWebhook processes incoming GitHub Actions webhook events.
func (c *Collector) HandleWebhook(payload map[string]interface{}) {
	action, _ := payload["action"].(string)
	c.emitEvent("pipeline_event", map[string]interface{}{
		"action":  action,
		"payload": payload,
	}, engine.SeverityInfo)
}

func (c *Collector) poll(ctx context.Context) {
	defer c.wg.Done()
	interval := c.config.PollInterval
	if interval == 0 {
		interval = 60 * time.Second
	}
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			// In production: polls GitHub API for workflow runs.
		}
	}
}

func (c *Collector) emitEvent(eventType string, payload map[string]interface{}, severity string) {
	event := &engine.TelemetryEvent{
		ID:        fmt.Sprintf("cicd-%d", time.Now().UnixNano()),
		Source:    "cicd",
		Type:      eventType,
		Timestamp: time.Now(),
		Payload:   payload,
		Metadata:  map[string]string{"collector": "cicd"},
		Severity:  severity,
	}
	select {
	case c.events <- event:
	default:
	}
}
