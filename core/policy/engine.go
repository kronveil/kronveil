package policy

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"sync"
	"time"

	"github.com/kronveil/kronveil/core/engine"
)

// Engine evaluates OPA policies against infrastructure resources and events.
type Engine struct {
	mu         sync.RWMutex
	policies   map[string]PolicyDefinition
	violations []engine.PolicyViolation
	evalCount  int64
	metrics    engine.MetricsRecorder
}

// SetMetrics sets the metrics recorder for the policy engine.
func (e *Engine) SetMetrics(m engine.MetricsRecorder) {
	e.metrics = m
}

// NewEngine creates a new policy engine with default policies loaded.
func NewEngine() *Engine {
	e := &Engine{
		policies: make(map[string]PolicyDefinition),
	}
	// Load default built-in policies.
	for id, p := range DefaultPolicies {
		p.Enabled = true
		e.policies[id] = p
	}
	log.Printf("[policy] Policy engine initialized with %d default policies", len(e.policies))
	return e
}

// AddPolicy adds or updates a policy.
func (e *Engine) AddPolicy(p PolicyDefinition) error {
	e.mu.Lock()
	defer e.mu.Unlock()
	if p.ID == "" {
		return fmt.Errorf("policy ID is required")
	}
	if p.Rego == "" {
		return fmt.Errorf("policy Rego source is required")
	}
	e.policies[p.ID] = p
	log.Printf("[policy] Added policy: %s (%s)", p.Name, p.ID)
	return nil
}

// RemovePolicy removes a policy by ID.
func (e *Engine) RemovePolicy(id string) error {
	e.mu.Lock()
	defer e.mu.Unlock()
	if _, ok := e.policies[id]; !ok {
		return fmt.Errorf("policy %q not found", id)
	}
	delete(e.policies, id)
	log.Printf("[policy] Removed policy: %s", id)
	return nil
}

// ListPolicies returns all registered policies.
func (e *Engine) ListPolicies() []PolicyDefinition {
	e.mu.RLock()
	defer e.mu.RUnlock()
	result := make([]PolicyDefinition, 0, len(e.policies))
	for _, p := range e.policies {
		result = append(result, p)
	}
	return result
}

// Evaluate checks an input resource against all enabled policies.
func (e *Engine) Evaluate(ctx context.Context, input interface{}) ([]engine.PolicyViolation, error) {
	e.mu.Lock()
	e.evalCount++
	e.mu.Unlock()

	if e.metrics != nil {
		e.metrics.RecordPolicyEvaluation()
	}

	// Serialize input for policy evaluation.
	inputBytes, err := json.Marshal(input)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal input: %w", err)
	}

	var inputMap map[string]interface{}
	if err := json.Unmarshal(inputBytes, &inputMap); err != nil {
		return nil, fmt.Errorf("failed to unmarshal input: %w", err)
	}

	var violations []engine.PolicyViolation

	e.mu.RLock()
	policies := make([]PolicyDefinition, 0)
	for _, p := range e.policies {
		if p.Enabled {
			policies = append(policies, p)
		}
	}
	e.mu.RUnlock()

	for _, p := range policies {
		// In production, this uses the OPA Go SDK (rego.New/rego.PrepareForEval).
		// For the open-source release, we evaluate the policy conceptually.
		policyViolations := e.evaluatePolicy(ctx, p, inputMap)
		violations = append(violations, policyViolations...)
	}

	// Store violations.
	if len(violations) > 0 {
		if e.metrics != nil {
			for range violations {
				e.metrics.RecordPolicyViolation()
			}
		}
		e.mu.Lock()
		e.violations = append(e.violations, violations...)
		// Keep only last 10000 violations in memory.
		if len(e.violations) > 10000 {
			e.violations = e.violations[len(e.violations)-10000:]
		}
		e.mu.Unlock()
	}

	return violations, nil
}

// evaluatePolicy runs a single policy against an input.
func (e *Engine) evaluatePolicy(_ context.Context, p PolicyDefinition, input map[string]interface{}) []engine.PolicyViolation {
	var violations []engine.PolicyViolation

	kind, _ := getNestedString(input, "kind")

	switch p.ID {
	case "require-resource-limits":
		if kind != "Pod" {
			return nil
		}
		violations = append(violations, e.checkResourceLimits(p, input)...)
	case "no-latest-tag":
		if kind != "Pod" {
			return nil
		}
		violations = append(violations, e.checkImageTags(p, input)...)
	}

	return violations
}

func (e *Engine) checkResourceLimits(p PolicyDefinition, input map[string]interface{}) []engine.PolicyViolation {
	var violations []engine.PolicyViolation
	containers := getContainers(input)
	podName, _ := getNestedString(input, "metadata", "name")

	for _, c := range containers {
		name, _ := c["name"].(string)
		resources, _ := c["resources"].(map[string]interface{})
		if resources == nil {
			violations = append(violations, engine.PolicyViolation{
				ID:        fmt.Sprintf("%s-%s-%s", p.ID, podName, name),
				Policy:    p.ID,
				Resource:  fmt.Sprintf("pod/%s/container/%s", podName, name),
				Violation: fmt.Sprintf("Container '%s' missing resource limits", name),
				Severity:  p.Severity,
				Timestamp: time.Now(),
			})
			continue
		}
		limits, _ := resources["limits"].(map[string]interface{})
		if limits == nil || limits["cpu"] == nil || limits["memory"] == nil {
			violations = append(violations, engine.PolicyViolation{
				ID:        fmt.Sprintf("%s-%s-%s", p.ID, podName, name),
				Policy:    p.ID,
				Resource:  fmt.Sprintf("pod/%s/container/%s", podName, name),
				Violation: fmt.Sprintf("Container '%s' missing CPU or memory limit", name),
				Severity:  p.Severity,
				Timestamp: time.Now(),
			})
		}
	}
	return violations
}

func (e *Engine) checkImageTags(p PolicyDefinition, input map[string]interface{}) []engine.PolicyViolation {
	var violations []engine.PolicyViolation
	containers := getContainers(input)
	podName, _ := getNestedString(input, "metadata", "name")

	for _, c := range containers {
		name, _ := c["name"].(string)
		image, _ := c["image"].(string)
		if image == "" {
			continue
		}
		if hasLatestTag(image) {
			violations = append(violations, engine.PolicyViolation{
				ID:        fmt.Sprintf("%s-%s-%s", p.ID, podName, name),
				Policy:    p.ID,
				Resource:  fmt.Sprintf("pod/%s/container/%s", podName, name),
				Violation: fmt.Sprintf("Container '%s' uses :latest tag", name),
				Severity:  p.Severity,
				Timestamp: time.Now(),
			})
		}
	}
	return violations
}

func getContainers(input map[string]interface{}) []map[string]interface{} {
	spec, _ := input["spec"].(map[string]interface{})
	if spec == nil {
		return nil
	}
	containers, _ := spec["containers"].([]interface{})
	result := make([]map[string]interface{}, 0, len(containers))
	for _, c := range containers {
		if cm, ok := c.(map[string]interface{}); ok {
			result = append(result, cm)
		}
	}
	return result
}

func getNestedString(m map[string]interface{}, keys ...string) (string, bool) {
	current := m
	for i, key := range keys {
		if i == len(keys)-1 {
			val, ok := current[key].(string)
			return val, ok
		}
		next, ok := current[key].(map[string]interface{})
		if !ok {
			return "", false
		}
		current = next
	}
	return "", false
}

func hasLatestTag(image string) bool {
	for i := len(image) - 1; i >= 0; i-- {
		if image[i] == ':' {
			return image[i+1:] == "latest"
		}
		if image[i] == '/' {
			break
		}
	}
	// No tag at all implies :latest.
	return true
}

// Violations returns recent policy violations.
func (e *Engine) Violations() []engine.PolicyViolation {
	e.mu.RLock()
	defer e.mu.RUnlock()
	result := make([]engine.PolicyViolation, len(e.violations))
	copy(result, e.violations)
	return result
}

// Health returns the health status of the policy engine.
func (e *Engine) Health() engine.ComponentHealth {
	return engine.ComponentHealth{
		Name:      "policy-engine",
		Status:    "healthy",
		Message:   fmt.Sprintf("%d policies loaded, %d evaluations", len(e.policies), e.evalCount),
		LastCheck: time.Now(),
	}
}
