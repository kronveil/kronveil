// Package cicd collects telemetry from CI/CD pipelines.
package cicd

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/kronveil/kronveil/core/engine"
)

// Config holds the CI/CD collector configuration.
type Config struct {
	WebhookPort   int           `yaml:"webhook_port" json:"webhook_port"`
	WebhookSecret string        `yaml:"webhook_secret" json:"webhook_secret"`
	RepoFilters   []string      `yaml:"repo_filters" json:"repo_filters"`
	PollInterval  time.Duration `yaml:"poll_interval" json:"poll_interval"`
	GithubToken   string        `yaml:"github_token" json:"github_token"`
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
	Name      string         `json:"name"`
	Status    string         `json:"status"`
	StartedAt time.Time      `json:"started_at"`
	Duration  time.Duration  `json:"duration"`
	Steps     []PipelineStep `json:"steps"`
}

// PipelineStep represents a single step within a job.
type PipelineStep struct {
	Name     string        `json:"name"`
	Status   string        `json:"status"`
	Duration time.Duration `json:"duration"`
}

// githubWorkflowRunsResponse is the GitHub API response for listing workflow runs.
type githubWorkflowRunsResponse struct {
	TotalCount   int                 `json:"total_count"`
	WorkflowRuns []githubWorkflowRun `json:"workflow_runs"`
}

// githubWorkflowRun represents a single workflow run from the GitHub API.
type githubWorkflowRun struct {
	ID         int64      `json:"id"`
	Name       string     `json:"name"`
	Status     string     `json:"status"`
	Conclusion *string    `json:"conclusion"`
	HeadBranch string     `json:"head_branch"`
	HeadSHA    string     `json:"head_sha"`
	RunNumber  int        `json:"run_number"`
	Event      string     `json:"event"`
	CreatedAt  time.Time  `json:"created_at"`
	UpdatedAt  time.Time  `json:"updated_at"`
	RunStarted time.Time  `json:"run_started_at"`
	HTMLURL    string     `json:"html_url"`
	Repository ghRepo     `json:"repository"`
}

// ghRepo is a minimal representation of a GitHub repository in API responses.
type ghRepo struct {
	FullName string `json:"full_name"`
}

// Collector gathers CI/CD pipeline telemetry.
type Collector struct {
	config     Config
	httpClient *http.Client
	events     chan *engine.TelemetryEvent
	mu         sync.RWMutex
	running    bool
	cancel     context.CancelFunc
	wg         sync.WaitGroup
	runs       map[string]*PipelineRun
	lastPoll   time.Time
	lastErr    error
}

// New creates a new CI/CD collector.
func New(config Config) *Collector {
	return &Collector{
		config: config,
		httpClient: &http.Client{
			Timeout: 10 * time.Second,
		},
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
	c.mu.RLock()
	runCount := len(c.runs)
	lastPoll := c.lastPoll
	lastErr := c.lastErr
	c.mu.RUnlock()

	status := "healthy"
	msg := fmt.Sprintf("tracking %d runs across %d repos, last poll: %s",
		runCount, len(c.config.RepoFilters), lastPoll.Format(time.RFC3339))

	if lastErr != nil {
		status = "degraded"
		msg = fmt.Sprintf("%s (last error: %v)", msg, lastErr)
	}
	if c.config.GithubToken == "" {
		status = "degraded"
		msg = fmt.Sprintf("%s (no github token configured)", msg)
	}

	return engine.ComponentHealth{
		Name:      "cicd-collector",
		Status:    status,
		Message:   msg,
		LastCheck: time.Now(),
	}
}

// HandleWebhook processes incoming GitHub Actions webhook events.
func (c *Collector) HandleWebhook(payload map[string]interface{}) {
	action, _ := payload["action"].(string)

	workflowRun, ok := payload["workflow_run"].(map[string]interface{})
	if !ok {
		c.emitEvent("pipeline_webhook", map[string]interface{}{
			"action":  action,
			"payload": payload,
		}, engine.SeverityInfo)
		return
	}

	runID := ""
	if id, ok := workflowRun["id"].(float64); ok {
		runID = fmt.Sprintf("%d", int64(id))
	}
	status, _ := workflowRun["status"].(string)
	conclusion, _ := workflowRun["conclusion"].(string)
	headBranch, _ := workflowRun["head_branch"].(string)
	headSHA, _ := workflowRun["head_sha"].(string)
	name, _ := workflowRun["name"].(string)
	htmlURL, _ := workflowRun["html_url"].(string)

	repo := ""
	if repoObj, ok := workflowRun["repository"].(map[string]interface{}); ok {
		repo, _ = repoObj["full_name"].(string)
	}
	if repo == "" {
		if repoObj, ok := payload["repository"].(map[string]interface{}); ok {
			repo, _ = repoObj["full_name"].(string)
		}
	}

	var duration time.Duration
	if startedStr, ok := workflowRun["run_started_at"].(string); ok {
		if updatedStr, ok := workflowRun["updated_at"].(string); ok {
			if started, err := time.Parse(time.RFC3339, startedStr); err == nil {
				if updated, err := time.Parse(time.RFC3339, updatedStr); err == nil {
					duration = updated.Sub(started)
				}
			}
		}
	}

	severity := engine.SeverityInfo
	if conclusion == "failure" || conclusion == "timed_out" {
		severity = engine.SeverityHigh
	} else if conclusion == "cancelled" {
		severity = engine.SeverityMedium
	}

	// Update tracked run.
	c.mu.Lock()
	run := c.runs[runID]
	if run == nil {
		run = &PipelineRun{
			ID:     runID,
			Repo:   repo,
			Branch: headBranch,
			Commit: headSHA,
			Labels: map[string]string{},
		}
		c.runs[runID] = run
	}
	run.Status = status
	run.Conclusion = conclusion
	run.Duration = duration
	c.mu.Unlock()

	c.emitEvent("pipeline_webhook", map[string]interface{}{
		"action":     action,
		"run_id":     runID,
		"name":       name,
		"repo":       repo,
		"branch":     headBranch,
		"commit":     headSHA,
		"status":     status,
		"conclusion": conclusion,
		"duration_s": duration.Seconds(),
		"url":        htmlURL,
	}, severity)
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
			c.pollGitHub(ctx)
		}
	}
}

// pollGitHub fetches workflow runs from the GitHub API for all configured repos.
func (c *Collector) pollGitHub(ctx context.Context) {
	if c.config.GithubToken == "" {
		log.Printf("[cicd-collector] No github_token configured, skipping API poll")
		return
	}

	repos := c.config.RepoFilters
	if len(repos) == 0 {
		log.Printf("[cicd-collector] No repo_filters configured, skipping API poll")
		return
	}

	c.mu.Lock()
	c.lastPoll = time.Now()
	c.mu.Unlock()

	for _, repoFilter := range repos {
		parts := strings.SplitN(repoFilter, "/", 2)
		if len(parts) != 2 || parts[0] == "" || parts[1] == "" {
			log.Printf("[cicd-collector] Skipping invalid repo filter %q (expected owner/repo)", repoFilter)
			continue
		}
		owner, repo := parts[0], parts[1]

		if err := c.fetchWorkflowRuns(ctx, owner, repo); err != nil {
			log.Printf("[cicd-collector] Error polling %s/%s: %v", owner, repo, err)
			c.mu.Lock()
			c.lastErr = fmt.Errorf("poll %s/%s: %w", owner, repo, err)
			c.mu.Unlock()
		}
	}
}

// fetchWorkflowRuns calls the GitHub API and processes workflow runs for a single repo.
func (c *Collector) fetchWorkflowRuns(ctx context.Context, owner, repo string) error {
	url := fmt.Sprintf("https://api.github.com/repos/%s/%s/actions/runs?per_page=10", owner, repo)

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return fmt.Errorf("creating request: %w", err)
	}
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("Authorization", "Bearer "+c.config.GithubToken)
	req.Header.Set("X-GitHub-Api-Version", "2022-11-28")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("executing request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 1024))
		return fmt.Errorf("GitHub API returned %d: %s", resp.StatusCode, string(body))
	}

	var result githubWorkflowRunsResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return fmt.Errorf("decoding response: %w", err)
	}

	for _, run := range result.WorkflowRuns {
		runID := fmt.Sprintf("%d", run.ID)

		c.mu.RLock()
		_, alreadyTracked := c.runs[runID]
		c.mu.RUnlock()

		if alreadyTracked {
			// Check if status changed for already-tracked runs.
			c.mu.RLock()
			existing := c.runs[runID]
			oldStatus := existing.Status
			oldConclusion := existing.Conclusion
			c.mu.RUnlock()

			conclusion := ""
			if run.Conclusion != nil {
				conclusion = *run.Conclusion
			}

			if oldStatus == run.Status && oldConclusion == conclusion {
				continue // No change, skip.
			}

			// Status changed -- update and emit.
			c.mu.Lock()
			existing.Status = run.Status
			existing.Conclusion = conclusion
			if !run.RunStarted.IsZero() {
				existing.Duration = time.Since(run.RunStarted)
				if run.Status == "completed" {
					existing.Duration = run.UpdatedAt.Sub(run.RunStarted)
					completedAt := run.UpdatedAt
					existing.CompletedAt = &completedAt
				}
			}
			c.mu.Unlock()

			severity := c.severityForConclusion(conclusion)
			c.emitEvent("pipeline_status_change", map[string]interface{}{
				"run_id":         runID,
				"name":           run.Name,
				"repo":           run.Repository.FullName,
				"branch":         run.HeadBranch,
				"commit":         run.HeadSHA,
				"status":         run.Status,
				"conclusion":     conclusion,
				"old_status":     oldStatus,
				"old_conclusion": oldConclusion,
				"duration_s":     existing.Duration.Seconds(),
				"url":            run.HTMLURL,
				"event":          run.Event,
				"run_number":     run.RunNumber,
			}, severity)

			continue
		}

		// New run -- track and emit.
		conclusion := ""
		if run.Conclusion != nil {
			conclusion = *run.Conclusion
		}

		var duration time.Duration
		if !run.RunStarted.IsZero() {
			if run.Status == "completed" {
				duration = run.UpdatedAt.Sub(run.RunStarted)
			} else {
				duration = time.Since(run.RunStarted)
			}
		}

		var completedAt *time.Time
		if run.Status == "completed" {
			t := run.UpdatedAt
			completedAt = &t
		}

		pRun := &PipelineRun{
			ID:          runID,
			Repo:        run.Repository.FullName,
			Branch:      run.HeadBranch,
			Commit:      run.HeadSHA,
			Status:      run.Status,
			Conclusion:  conclusion,
			StartedAt:   run.RunStarted,
			CompletedAt: completedAt,
			Duration:    duration,
			Labels: map[string]string{
				"event":      run.Event,
				"run_number": fmt.Sprintf("%d", run.RunNumber),
			},
		}

		c.mu.Lock()
		c.runs[runID] = pRun
		c.mu.Unlock()

		severity := c.severityForConclusion(conclusion)
		c.emitEvent("pipeline_run", map[string]interface{}{
			"run_id":     runID,
			"name":       run.Name,
			"repo":       run.Repository.FullName,
			"branch":     run.HeadBranch,
			"commit":     run.HeadSHA,
			"status":     run.Status,
			"conclusion": conclusion,
			"duration_s": duration.Seconds(),
			"url":        run.HTMLURL,
			"event":      run.Event,
			"run_number": run.RunNumber,
			"created_at": run.CreatedAt.Format(time.RFC3339),
		}, severity)
	}

	return nil
}

// severityForConclusion maps a GitHub workflow conclusion to an event severity.
func (c *Collector) severityForConclusion(conclusion string) string {
	switch conclusion {
	case "failure", "timed_out":
		return engine.SeverityHigh
	case "cancelled":
		return engine.SeverityMedium
	case "action_required":
		return engine.SeverityMedium
	default:
		return engine.SeverityInfo
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
