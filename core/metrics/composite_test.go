package metrics

import (
	"testing"

	"github.com/kronveil/kronveil/core/engine"
	"github.com/kronveil/kronveil/internal/testutil"
)

func TestNewCompositeRecorder_Zero(t *testing.T) {
	r := NewCompositeRecorder()
	if _, ok := r.(*engine.NoopMetricsRecorder); !ok {
		t.Errorf("expected NoopMetricsRecorder, got %T", r)
	}
}

func TestNewCompositeRecorder_One(t *testing.T) {
	m := &testutil.MockMetricsRecorder{}
	r := NewCompositeRecorder(m)
	if r != m {
		t.Error("expected same recorder back when only one provided")
	}
}

func TestNewCompositeRecorder_Multiple(t *testing.T) {
	m1 := &testutil.MockMetricsRecorder{}
	m2 := &testutil.MockMetricsRecorder{}
	r := NewCompositeRecorder(m1, m2)
	if _, ok := r.(*CompositeRecorder); !ok {
		t.Errorf("expected CompositeRecorder, got %T", r)
	}
}

func TestCompositeRecorder_Delegation(t *testing.T) {
	m1 := &testutil.MockMetricsRecorder{}
	m2 := &testutil.MockMetricsRecorder{}
	r := NewCompositeRecorder(m1, m2)

	r.RecordEvent("test")
	r.RecordCollectorError("test")
	r.RecordAnomaly()
	r.RecordIncidentCreated()
	r.RecordIncidentResolved()
	r.RecordRemediation(true)
	r.RecordRemediation(false)
	r.RecordPolicyEvaluation()
	r.RecordPolicyViolation()
	r.SetComponentHealth(1, 2, 3)
	r.SetUptime(100)
	r.SetMTTR(30)

	for _, m := range []*testutil.MockMetricsRecorder{m1, m2} {
		if m.Events.Load() != 1 {
			t.Errorf("Events = %d, want 1", m.Events.Load())
		}
		if m.CollectorErrors.Load() != 1 {
			t.Errorf("CollectorErrors = %d, want 1", m.CollectorErrors.Load())
		}
		if m.Anomalies.Load() != 1 {
			t.Errorf("Anomalies = %d, want 1", m.Anomalies.Load())
		}
		if m.IncidentsCreated.Load() != 1 {
			t.Errorf("IncidentsCreated = %d, want 1", m.IncidentsCreated.Load())
		}
		if m.IncidentsResolved.Load() != 1 {
			t.Errorf("IncidentsResolved = %d, want 1", m.IncidentsResolved.Load())
		}
		if m.RemediationsOK.Load() != 1 {
			t.Errorf("RemediationsOK = %d, want 1", m.RemediationsOK.Load())
		}
		if m.RemediationsFail.Load() != 1 {
			t.Errorf("RemediationsFail = %d, want 1", m.RemediationsFail.Load())
		}
		if m.PolicyEvals.Load() != 1 {
			t.Errorf("PolicyEvals = %d, want 1", m.PolicyEvals.Load())
		}
		if m.PolicyViolations.Load() != 1 {
			t.Errorf("PolicyViolations = %d, want 1", m.PolicyViolations.Load())
		}
		if m.HealthyVal.Load() != 1 {
			t.Errorf("HealthyVal = %d, want 1", m.HealthyVal.Load())
		}
		if m.DegradedVal.Load() != 2 {
			t.Errorf("DegradedVal = %d, want 2", m.DegradedVal.Load())
		}
		if m.CriticalVal.Load() != 3 {
			t.Errorf("CriticalVal = %d, want 3", m.CriticalVal.Load())
		}
	}
}

func TestNoopRecorder_NoPanic(t *testing.T) {
	n := &engine.NoopMetricsRecorder{}
	n.RecordEvent("x")
	n.RecordCollectorError("x")
	n.RecordAnomaly()
	n.RecordIncidentCreated()
	n.RecordIncidentResolved()
	n.RecordRemediation(true)
	n.SetMTTR(1.0)
	n.RecordPolicyEvaluation()
	n.RecordPolicyViolation()
	n.SetComponentHealth(0, 0, 0)
	n.SetUptime(0)
}
