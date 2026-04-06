// Package runbook provides automated runbook execution for incident response.
package runbook

import (
	"context"
	"fmt"
	"log"
	"sync"
	"time"
)

// Runbook defines an automated remediation playbook attached to incident types.
type Runbook struct {
	ID            string        `json:"id"`
	Name          string        `json:"name"`
	Description   string        `json:"description"`
	IncidentTypes []string      `json:"incident_types"`
	Steps         []Step        `json:"steps"`
	Enabled       bool          `json:"enabled"`
	AutoExecute   bool          `json:"auto_execute"`
	MaxRetries    int           `json:"max_retries"`
	Timeout       time.Duration `json:"timeout"`
}

// Step defines a single action within a runbook.
type Step struct {
	Name            string            `json:"name"`
	Action          string            `json:"action"`
	Params          map[string]string `json:"params"`
	ContinueOnError bool             `json:"continue_on_error"`
	Timeout         time.Duration     `json:"timeout"`
}

// ExecutionResult tracks the outcome of a runbook execution.
type ExecutionResult struct {
	RunbookID   string       `json:"runbook_id"`
	IncidentID  string       `json:"incident_id"`
	Status      string       `json:"status"`
	StepResults []StepResult `json:"step_results"`
	StartedAt   time.Time    `json:"started_at"`
	CompletedAt time.Time    `json:"completed_at"`
	Duration    time.Duration `json:"duration"`
}

// StepResult tracks the outcome of a single step execution.
type StepResult struct {
	StepName string        `json:"step_name"`
	Status   string        `json:"status"`
	Output   string        `json:"output"`
	Error    string        `json:"error"`
	Duration time.Duration `json:"duration"`
}

// Executor manages runbook registration, indexing, and execution.
type Executor struct {
	runbooks   map[string]*Runbook
	typeIndex  map[string][]*Runbook
	executions map[string]*ExecutionResult
	mu         sync.RWMutex
}

// New creates a new runbook Executor.
func New() *Executor {
	return &Executor{
		runbooks:   make(map[string]*Runbook),
		typeIndex:  make(map[string][]*Runbook),
		executions: make(map[string]*ExecutionResult),
	}
}

// RegisterRunbook adds a runbook and indexes it by its incident types.
func (e *Executor) RegisterRunbook(rb *Runbook) error {
	if rb == nil {
		return fmt.Errorf("runbook must not be nil")
	}
	if rb.ID == "" {
		return fmt.Errorf("runbook ID must not be empty")
	}

	e.mu.Lock()
	defer e.mu.Unlock()

	if _, exists := e.runbooks[rb.ID]; exists {
		return fmt.Errorf("runbook %s already registered", rb.ID)
	}

	e.runbooks[rb.ID] = rb
	for _, incType := range rb.IncidentTypes {
		e.typeIndex[incType] = append(e.typeIndex[incType], rb)
	}

	log.Printf("[runbook] Registered runbook %s (%s) for incident types %v", rb.ID, rb.Name, rb.IncidentTypes)
	return nil
}

// RemoveRunbook removes a runbook and its type index entries.
func (e *Executor) RemoveRunbook(id string) {
	e.mu.Lock()
	defer e.mu.Unlock()

	rb, ok := e.runbooks[id]
	if !ok {
		return
	}

	// Remove from type index.
	for _, incType := range rb.IncidentTypes {
		runbooks := e.typeIndex[incType]
		filtered := make([]*Runbook, 0, len(runbooks))
		for _, r := range runbooks {
			if r.ID != id {
				filtered = append(filtered, r)
			}
		}
		if len(filtered) == 0 {
			delete(e.typeIndex, incType)
		} else {
			e.typeIndex[incType] = filtered
		}
	}

	delete(e.runbooks, id)
	log.Printf("[runbook] Removed runbook %s", id)
}

// ListRunbooks returns all registered runbooks.
func (e *Executor) ListRunbooks() []*Runbook {
	e.mu.RLock()
	defer e.mu.RUnlock()

	result := make([]*Runbook, 0, len(e.runbooks))
	for _, rb := range e.runbooks {
		result = append(result, rb)
	}
	return result
}

// FindRunbooks returns all runbooks matching the given incident type.
func (e *Executor) FindRunbooks(incidentType string) []*Runbook {
	e.mu.RLock()
	defer e.mu.RUnlock()

	runbooks := e.typeIndex[incidentType]
	result := make([]*Runbook, 0, len(runbooks))
	for _, rb := range runbooks {
		if rb.Enabled {
			result = append(result, rb)
		}
	}
	return result
}

// Execute runs a runbook step by step for the given incident.
func (e *Executor) Execute(ctx context.Context, rb *Runbook, incidentID string) (*ExecutionResult, error) {
	if rb == nil {
		return nil, fmt.Errorf("runbook must not be nil")
	}

	execID := fmt.Sprintf("%s-%s-%d", rb.ID, incidentID, time.Now().UnixNano())
	result := &ExecutionResult{
		RunbookID:   rb.ID,
		IncidentID:  incidentID,
		Status:      "running",
		StepResults: make([]StepResult, 0, len(rb.Steps)),
		StartedAt:   time.Now(),
	}

	e.mu.Lock()
	e.executions[execID] = result
	e.mu.Unlock()

	log.Printf("[runbook] Executing runbook %s (%s) for incident %s", rb.ID, rb.Name, incidentID)

	// Apply runbook-level timeout if set.
	execCtx := ctx
	if rb.Timeout > 0 {
		var cancel context.CancelFunc
		execCtx, cancel = context.WithTimeout(ctx, rb.Timeout)
		defer cancel()
	}

	for _, step := range rb.Steps {
		stepResult := e.executeStep(execCtx, step)
		result.StepResults = append(result.StepResults, *stepResult)

		if stepResult.Status == "failed" && !step.ContinueOnError {
			result.Status = "failed"
			result.CompletedAt = time.Now()
			result.Duration = result.CompletedAt.Sub(result.StartedAt)
			log.Printf("[runbook] Runbook %s failed at step %s: %s", rb.ID, step.Name, stepResult.Error)
			return result, fmt.Errorf("step %s failed: %s", step.Name, stepResult.Error)
		}
	}

	result.Status = "completed"
	result.CompletedAt = time.Now()
	result.Duration = result.CompletedAt.Sub(result.StartedAt)
	log.Printf("[runbook] Runbook %s completed for incident %s in %s", rb.ID, incidentID, result.Duration)
	return result, nil
}

// executeStep dispatches a single step to the appropriate action handler.
func (e *Executor) executeStep(ctx context.Context, step Step) *StepResult {
	start := time.Now()

	// Apply step-level timeout if set.
	stepCtx := ctx
	if step.Timeout > 0 {
		var cancel context.CancelFunc
		stepCtx, cancel = context.WithTimeout(ctx, step.Timeout)
		defer cancel()
	}

	// Check for context cancellation before executing.
	select {
	case <-stepCtx.Done():
		return &StepResult{
			StepName: step.Name,
			Status:   "failed",
			Error:    "context cancelled before step execution",
			Duration: time.Since(start),
		}
	default:
	}

	var output string
	var stepErr error

	switch step.Action {
	case "kubectl_scale":
		output, stepErr = handleKubectlScale(step.Params)
	case "restart_pod":
		output, stepErr = handleRestartPod(step.Params)
	case "notify_oncall":
		output, stepErr = handleNotifyOncall(step.Params)
	case "run_diagnostic":
		output, stepErr = handleRunDiagnostic(step.Params)
	case "custom_script":
		output, stepErr = handleCustomScript(step.Params)
	default:
		stepErr = fmt.Errorf("unknown action: %s", step.Action)
	}

	result := &StepResult{
		StepName: step.Name,
		Status:   "completed",
		Output:   output,
		Duration: time.Since(start),
	}
	if stepErr != nil {
		result.Status = "failed"
		result.Error = stepErr.Error()
	}

	log.Printf("[runbook] Step %s (%s): %s", step.Name, step.Action, result.Status)
	return result
}

// GetExecution returns an execution result by ID.
func (e *Executor) GetExecution(id string) *ExecutionResult {
	e.mu.RLock()
	defer e.mu.RUnlock()
	return e.executions[id]
}

// ListExecutions returns all execution results.
func (e *Executor) ListExecutions() []*ExecutionResult {
	e.mu.RLock()
	defer e.mu.RUnlock()

	result := make([]*ExecutionResult, 0, len(e.executions))
	for _, exec := range e.executions {
		result = append(result, exec)
	}
	return result
}

// Action handlers — dry-run implementations that log intended actions.
// These are structured for wiring real implementations later.

func handleKubectlScale(params map[string]string) (string, error) {
	deployment := params["deployment"]
	replicas := params["replicas"]
	namespace := params["namespace"]
	if deployment == "" || replicas == "" {
		return "", fmt.Errorf("kubectl_scale requires 'deployment' and 'replicas' params")
	}
	if namespace == "" {
		namespace = "default"
	}
	msg := fmt.Sprintf("[dry-run] kubectl scale deployment/%s --replicas=%s -n %s", deployment, replicas, namespace)
	log.Printf("[runbook] %s", msg)
	return msg, nil
}

func handleRestartPod(params map[string]string) (string, error) {
	pod := params["pod"]
	namespace := params["namespace"]
	if pod == "" {
		return "", fmt.Errorf("restart_pod requires 'pod' param")
	}
	if namespace == "" {
		namespace = "default"
	}
	msg := fmt.Sprintf("[dry-run] kubectl delete pod %s -n %s (restart)", pod, namespace)
	log.Printf("[runbook] %s", msg)
	return msg, nil
}

func handleNotifyOncall(params map[string]string) (string, error) {
	channel := params["channel"]
	message := params["message"]
	if channel == "" {
		channel = "#oncall"
	}
	if message == "" {
		message = "Incident requires attention"
	}
	msg := fmt.Sprintf("[dry-run] Notify %s: %s", channel, message)
	log.Printf("[runbook] %s", msg)
	return msg, nil
}

func handleRunDiagnostic(params map[string]string) (string, error) {
	command := params["command"]
	if command == "" {
		return "", fmt.Errorf("run_diagnostic requires 'command' param")
	}
	msg := fmt.Sprintf("[dry-run] Run diagnostic: %s", command)
	log.Printf("[runbook] %s", msg)
	return msg, nil
}

func handleCustomScript(params map[string]string) (string, error) {
	script := params["script"]
	if script == "" {
		return "", fmt.Errorf("custom_script requires 'script' param")
	}
	msg := fmt.Sprintf("[dry-run] Execute script: %s", script)
	log.Printf("[runbook] %s", msg)
	return msg, nil
}
