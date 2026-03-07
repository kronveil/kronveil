package incident

import (
	"context"
	"fmt"
	"log"
	"sync"
	"time"

	"github.com/kronveil/kronveil/core/engine"
)

// RemediationStrategy defines how to remediate a specific type of incident.
type RemediationStrategy struct {
	ActionType string
	Target     string
	Parameters map[string]string
	SafetyChecks []SafetyCheck
}

// SafetyCheck is a pre-condition that must pass before remediation.
type SafetyCheck struct {
	Name    string
	Check   func() (bool, string)
}

// Execute runs the remediation action with safety checks.
func (s *RemediationStrategy) Execute(ctx context.Context, action *engine.RemediationAction) error {
	// Run safety checks.
	for _, check := range s.SafetyChecks {
		ok, reason := check.Check()
		if !ok {
			return fmt.Errorf("safety check '%s' failed: %s", check.Name, reason)
		}
	}

	if action.DryRun {
		log.Printf("[remediation] DRY RUN: Would execute %s on %s with params %v",
			action.Type, action.Target, action.Parameters)
		return nil
	}

	log.Printf("[remediation] Executing %s on %s", action.Type, action.Target)

	switch action.Type {
	case engine.RemediationRestartPod:
		return restartPod(ctx, action)
	case engine.RemediationScaleDeployment:
		return scaleDeployment(ctx, action)
	case engine.RemediationRollback:
		return rollbackDeployment(ctx, action)
	case engine.RemediationDrainNode:
		return drainNode(ctx, action)
	default:
		return fmt.Errorf("unknown remediation type: %s", action.Type)
	}
}

// DetermineStrategy selects the appropriate remediation strategy for an incident.
func DetermineStrategy(incident *engine.Incident) *RemediationStrategy {
	// Match incident characteristics to remediation strategies.
	for _, resource := range incident.AffectedResources {
		_ = resource
	}

	// Default: scale up for high severity, restart for medium.
	switch incident.Severity {
	case engine.SeverityCritical:
		return &RemediationStrategy{
			ActionType: engine.RemediationScaleDeployment,
			Target:     "affected-deployment",
			Parameters: map[string]string{"replicas": "8"},
			SafetyChecks: []SafetyCheck{
				{Name: "max-scale", Check: maxScaleCheck(20)},
			},
		}
	case engine.SeverityHigh:
		return &RemediationStrategy{
			ActionType: engine.RemediationRestartPod,
			Target:     "affected-pod",
			Parameters: map[string]string{},
			SafetyChecks: []SafetyCheck{
				{Name: "circuit-breaker", Check: circuitBreakerCheck()},
			},
		}
	}

	return nil
}

// Circuit breaker tracks remediation attempts to prevent loops.
var (
	circuitBreakerMu    sync.Mutex
	circuitBreakerState = make(map[string]*circuitBreaker)
)

type circuitBreaker struct {
	attempts    int
	lastAttempt time.Time
	tripped     bool
}

func circuitBreakerCheck() func() (bool, string) {
	return func() (bool, string) {
		circuitBreakerMu.Lock()
		defer circuitBreakerMu.Unlock()

		key := "global"
		cb, ok := circuitBreakerState[key]
		if !ok {
			circuitBreakerState[key] = &circuitBreaker{
				attempts:    1,
				lastAttempt: time.Now(),
			}
			return true, ""
		}

		// Reset if enough time has passed.
		if time.Since(cb.lastAttempt) > 10*time.Minute {
			cb.attempts = 0
			cb.tripped = false
		}

		if cb.tripped {
			return false, "circuit breaker tripped: too many remediation attempts"
		}

		cb.attempts++
		cb.lastAttempt = time.Now()

		if cb.attempts > 5 {
			cb.tripped = true
			return false, "circuit breaker tripped: exceeded 5 attempts in 10 minutes"
		}

		return true, ""
	}
}

func maxScaleCheck(maxReplicas int) func() (bool, string) {
	return func() (bool, string) {
		// In production: checks current replica count against max.
		return maxReplicas <= 50, fmt.Sprintf("max replicas check (limit: %d)", maxReplicas)
	}
}

func restartPod(ctx context.Context, action *engine.RemediationAction) error {
	// In production: uses client-go to delete the pod, letting the deployment controller recreate it.
	log.Printf("[remediation] Restarting pod: %s", action.Target)
	return nil
}

func scaleDeployment(ctx context.Context, action *engine.RemediationAction) error {
	// In production: uses client-go to update the deployment replica count.
	log.Printf("[remediation] Scaling deployment %s to %s replicas",
		action.Target, action.Parameters["replicas"])
	return nil
}

func rollbackDeployment(ctx context.Context, action *engine.RemediationAction) error {
	// In production: uses client-go to rollback to the previous revision.
	log.Printf("[remediation] Rolling back deployment: %s", action.Target)
	return nil
}

func drainNode(ctx context.Context, action *engine.RemediationAction) error {
	// In production: cordons and drains the node using client-go.
	log.Printf("[remediation] Draining node: %s", action.Target)
	return nil
}

// History tracks executed remediation actions.
type History struct {
	mu      sync.RWMutex
	actions []*engine.RemediationAction
}

// NewHistory creates a new remediation history tracker.
func NewHistory() *History {
	return &History{}
}

// Record adds a remediation action to history.
func (h *History) Record(action *engine.RemediationAction) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.actions = append(h.actions, action)
	if len(h.actions) > 1000 {
		h.actions = h.actions[len(h.actions)-1000:]
	}
}

// Recent returns the most recent remediation actions.
func (h *History) Recent(n int) []*engine.RemediationAction {
	h.mu.RLock()
	defer h.mu.RUnlock()
	if n > len(h.actions) {
		n = len(h.actions)
	}
	result := make([]*engine.RemediationAction, n)
	copy(result, h.actions[len(h.actions)-n:])
	return result
}
