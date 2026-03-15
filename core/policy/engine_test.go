package policy

import (
	"context"
	"testing"

	"github.com/kronveil/kronveil/internal/testutil"
)

func TestNewEngine_LoadsDefaults(t *testing.T) {
	e := NewEngine()
	policies := e.ListPolicies()
	if len(policies) == 0 {
		t.Fatal("expected default policies to be loaded")
	}
	if len(policies) != len(DefaultPolicies) {
		t.Errorf("expected %d policies, got %d", len(DefaultPolicies), len(policies))
	}
}

func TestAddRemovePolicy(t *testing.T) {
	e := NewEngine()

	p := PolicyDefinition{
		ID:   "test-policy",
		Name: "Test",
		Rego: "package test",
	}
	if err := e.AddPolicy(p); err != nil {
		t.Fatalf("AddPolicy failed: %v", err)
	}

	// Missing ID should error.
	if err := e.AddPolicy(PolicyDefinition{Rego: "package x"}); err == nil {
		t.Error("expected error for missing ID")
	}

	// Missing Rego should error.
	if err := e.AddPolicy(PolicyDefinition{ID: "x"}); err == nil {
		t.Error("expected error for missing Rego")
	}

	if err := e.RemovePolicy("test-policy"); err != nil {
		t.Fatalf("RemovePolicy failed: %v", err)
	}

	if err := e.RemovePolicy("nonexistent"); err == nil {
		t.Error("expected error for removing nonexistent policy")
	}
}

func TestResourceLimits_Violation(t *testing.T) {
	e := NewEngine()

	// Pod without resource limits should trigger violation.
	input := map[string]interface{}{
		"kind": "Pod",
		"metadata": map[string]interface{}{
			"name": "test-pod",
		},
		"spec": map[string]interface{}{
			"containers": []interface{}{
				map[string]interface{}{
					"name":  "app",
					"image": "myapp:v1",
				},
			},
		},
	}

	violations, err := e.Evaluate(context.Background(), input)
	if err != nil {
		t.Fatalf("Evaluate failed: %v", err)
	}
	if len(violations) == 0 {
		t.Error("expected violations for pod without resource limits")
	}

	found := false
	for _, v := range violations {
		if v.Policy == "require-resource-limits" {
			found = true
			break
		}
	}
	if !found {
		t.Error("expected require-resource-limits violation")
	}
}

func TestResourceLimits_NoViolation(t *testing.T) {
	e := NewEngine()

	input := map[string]interface{}{
		"kind": "Pod",
		"metadata": map[string]interface{}{
			"name": "test-pod",
		},
		"spec": map[string]interface{}{
			"containers": []interface{}{
				map[string]interface{}{
					"name":  "app",
					"image": "myapp:v1.0",
					"resources": map[string]interface{}{
						"limits": map[string]interface{}{
							"cpu":    "500m",
							"memory": "256Mi",
						},
					},
				},
			},
		},
	}

	violations, err := e.Evaluate(context.Background(), input)
	if err != nil {
		t.Fatalf("Evaluate failed: %v", err)
	}

	for _, v := range violations {
		if v.Policy == "require-resource-limits" {
			t.Error("expected no resource-limits violation for pod with proper limits")
		}
	}
}

func TestImageTags_LatestViolation(t *testing.T) {
	e := NewEngine()

	input := map[string]interface{}{
		"kind": "Pod",
		"metadata": map[string]interface{}{
			"name": "test-pod",
		},
		"spec": map[string]interface{}{
			"containers": []interface{}{
				map[string]interface{}{
					"name":  "app",
					"image": "myapp:latest",
					"resources": map[string]interface{}{
						"limits": map[string]interface{}{
							"cpu":    "500m",
							"memory": "256Mi",
						},
					},
				},
			},
		},
	}

	violations, err := e.Evaluate(context.Background(), input)
	if err != nil {
		t.Fatalf("Evaluate failed: %v", err)
	}

	found := false
	for _, v := range violations {
		if v.Policy == "no-latest-tag" {
			found = true
		}
	}
	if !found {
		t.Error("expected no-latest-tag violation for :latest image")
	}
}

func TestImageTags_NoTag(t *testing.T) {
	e := NewEngine()

	input := map[string]interface{}{
		"kind": "Pod",
		"metadata": map[string]interface{}{
			"name": "test-pod",
		},
		"spec": map[string]interface{}{
			"containers": []interface{}{
				map[string]interface{}{
					"name":  "app",
					"image": "myapp",
					"resources": map[string]interface{}{
						"limits": map[string]interface{}{
							"cpu":    "500m",
							"memory": "256Mi",
						},
					},
				},
			},
		},
	}

	violations, err := e.Evaluate(context.Background(), input)
	if err != nil {
		t.Fatalf("Evaluate failed: %v", err)
	}

	found := false
	for _, v := range violations {
		if v.Policy == "no-latest-tag" {
			found = true
		}
	}
	if !found {
		t.Error("expected no-latest-tag violation for image without tag")
	}
}

func TestHasLatestTag(t *testing.T) {
	tests := []struct {
		image string
		want  bool
	}{
		{"nginx:latest", true},
		{"nginx:1.21", false},
		{"nginx", true},           // no tag implies :latest
		{"myrepo/app:v2.0", false},
		{"myrepo/app:latest", true},
		{"myrepo/app", true},
	}
	for _, tt := range tests {
		got := hasLatestTag(tt.image)
		if got != tt.want {
			t.Errorf("hasLatestTag(%q) = %v, want %v", tt.image, got, tt.want)
		}
	}
}

func TestViolationCap(t *testing.T) {
	e := NewEngine()

	// Generate more than 10000 violations to test cap.
	for i := 0; i < 200; i++ {
		input := map[string]interface{}{
			"kind": "Pod",
			"metadata": map[string]interface{}{
				"name": "pod",
			},
			"spec": map[string]interface{}{
				"containers": []interface{}{
					map[string]interface{}{
						"name":  "app",
						"image": "myapp:latest",
					},
				},
			},
		}
		_, _ = e.Evaluate(context.Background(), input)
	}

	violations := e.Violations()
	if len(violations) > 10000 {
		t.Errorf("violations should be capped at 10000, got %d", len(violations))
	}
}

func TestMetricsRecording(t *testing.T) {
	e := NewEngine()
	m := &testutil.MockMetricsRecorder{}
	e.SetMetrics(m)

	input := map[string]interface{}{
		"kind": "Pod",
		"metadata": map[string]interface{}{
			"name": "test-pod",
		},
		"spec": map[string]interface{}{
			"containers": []interface{}{
				map[string]interface{}{
					"name":  "app",
					"image": "myapp:latest",
				},
			},
		},
	}

	_, _ = e.Evaluate(context.Background(), input)

	if m.PolicyEvals.Load() != 1 {
		t.Errorf("PolicyEvals = %d, want 1", m.PolicyEvals.Load())
	}
	if m.PolicyViolations.Load() == 0 {
		t.Error("expected at least one policy violation recorded")
	}
}
