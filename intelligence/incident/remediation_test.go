package incident

import (
	"context"
	"testing"
	"time"

	"github.com/kronveil/kronveil/core/engine"
)

func TestDetermineStrategy_Critical(t *testing.T) {
	inc := &engine.Incident{
		Severity:          engine.SeverityCritical,
		AffectedResources: []string{"deploy/app"},
	}
	strategy := DetermineStrategy(inc)
	if strategy == nil {
		t.Fatal("expected strategy for critical incident")
	}
	if strategy.ActionType != engine.RemediationScaleDeployment {
		t.Errorf("expected scale_deployment, got %s", strategy.ActionType)
	}
}

func TestDetermineStrategy_High(t *testing.T) {
	inc := &engine.Incident{
		Severity:          engine.SeverityHigh,
		AffectedResources: []string{"pod/app"},
	}
	strategy := DetermineStrategy(inc)
	if strategy == nil {
		t.Fatal("expected strategy for high incident")
	}
	if strategy.ActionType != engine.RemediationRestartPod {
		t.Errorf("expected restart_pod, got %s", strategy.ActionType)
	}
}

func TestDetermineStrategy_Low_ReturnsNil(t *testing.T) {
	inc := &engine.Incident{
		Severity: engine.SeverityLow,
	}
	if DetermineStrategy(inc) != nil {
		t.Error("expected nil strategy for low severity")
	}
}

func TestCircuitBreaker(t *testing.T) {
	// Reset global state for test isolation.
	circuitBreakerMu.Lock()
	circuitBreakerState = make(map[string]*circuitBreaker)
	circuitBreakerMu.Unlock()

	check := circuitBreakerCheck()

	// First 5 attempts should pass.
	for i := 0; i < 5; i++ {
		ok, _ := check()
		if !ok {
			t.Fatalf("attempt %d should pass", i+1)
		}
	}

	// 6th attempt should trip.
	ok, msg := check()
	if ok {
		t.Error("6th attempt should trip circuit breaker")
	}
	if msg == "" {
		t.Error("expected message when circuit breaker trips")
	}
}

func TestMaxScaleCheck(t *testing.T) {
	// Under max.
	check := maxScaleCheck(20)
	ok, _ := check()
	if !ok {
		t.Error("maxScaleCheck(20) should pass")
	}

	// Over max (>50 fails).
	check = maxScaleCheck(51)
	ok, _ = check()
	if ok {
		t.Error("maxScaleCheck(51) should fail")
	}
}

func TestHistory_RecordAndRecent(t *testing.T) {
	h := NewHistory()

	for i := 0; i < 5; i++ {
		h.Record(&engine.RemediationAction{
			ID:        "rem-" + string(rune('A'+i)),
			CreatedAt: time.Now(),
		})
	}

	recent := h.Recent(3)
	if len(recent) != 3 {
		t.Fatalf("expected 3 recent, got %d", len(recent))
	}

	// Recent(100) with only 5 items should return 5.
	all := h.Recent(100)
	if len(all) != 5 {
		t.Errorf("expected 5, got %d", len(all))
	}
}

func TestHistory_Cap(t *testing.T) {
	h := NewHistory()
	for i := 0; i < 1100; i++ {
		h.Record(&engine.RemediationAction{ID: "rem"})
	}

	all := h.Recent(2000)
	if len(all) > 1000 {
		t.Errorf("history should cap at 1000, got %d", len(all))
	}
}

func TestExecute_DryRun(t *testing.T) {
	strategy := &RemediationStrategy{
		ActionType: engine.RemediationRestartPod,
		Target:     "test-pod",
		Parameters: map[string]string{},
	}

	action := &engine.RemediationAction{
		Type:   engine.RemediationRestartPod,
		Target: "test-pod",
		DryRun: true,
	}

	err := strategy.Execute(context.Background(), action)
	if err != nil {
		t.Fatalf("DryRun Execute should succeed: %v", err)
	}
}

func TestExecute_RestartPod(t *testing.T) {
	strategy := &RemediationStrategy{
		ActionType: engine.RemediationRestartPod,
		Target:     "test-pod",
		Parameters: map[string]string{},
	}

	action := &engine.RemediationAction{
		Type:   engine.RemediationRestartPod,
		Target: "test-pod",
		DryRun: false,
	}

	err := strategy.Execute(context.Background(), action)
	if err != nil {
		t.Fatalf("RestartPod Execute should succeed: %v", err)
	}
}

func TestExecute_ScaleDeployment(t *testing.T) {
	strategy := &RemediationStrategy{
		ActionType: engine.RemediationScaleDeployment,
		Target:     "test-deploy",
		Parameters: map[string]string{"replicas": "5"},
	}

	action := &engine.RemediationAction{
		Type:       engine.RemediationScaleDeployment,
		Target:     "test-deploy",
		Parameters: map[string]string{"replicas": "5"},
	}

	err := strategy.Execute(context.Background(), action)
	if err != nil {
		t.Fatalf("ScaleDeployment Execute should succeed: %v", err)
	}
}

func TestExecute_Rollback(t *testing.T) {
	action := &engine.RemediationAction{
		Type:   engine.RemediationRollback,
		Target: "test-deploy",
	}
	strategy := &RemediationStrategy{}
	err := strategy.Execute(context.Background(), action)
	if err != nil {
		t.Fatalf("Rollback Execute should succeed: %v", err)
	}
}

func TestExecute_DrainNode(t *testing.T) {
	action := &engine.RemediationAction{
		Type:   engine.RemediationDrainNode,
		Target: "node-1",
	}
	strategy := &RemediationStrategy{}
	err := strategy.Execute(context.Background(), action)
	if err != nil {
		t.Fatalf("DrainNode Execute should succeed: %v", err)
	}
}

func TestExecute_UnknownType(t *testing.T) {
	action := &engine.RemediationAction{
		Type:   "unknown_action",
		Target: "test",
	}
	strategy := &RemediationStrategy{}
	err := strategy.Execute(context.Background(), action)
	if err == nil {
		t.Error("expected error for unknown remediation type")
	}
}

func TestExecute_SafetyCheckFails(t *testing.T) {
	strategy := &RemediationStrategy{
		ActionType: engine.RemediationRestartPod,
		SafetyChecks: []SafetyCheck{
			{
				Name: "always-fail",
				Check: func() (bool, string) {
					return false, "safety check blocked"
				},
			},
		},
	}

	action := &engine.RemediationAction{
		Type:   engine.RemediationRestartPod,
		Target: "pod",
	}
	err := strategy.Execute(context.Background(), action)
	if err == nil {
		t.Error("expected error when safety check fails")
	}
}
