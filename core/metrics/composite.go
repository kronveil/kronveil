package metrics

import "github.com/kronveil/kronveil/core/engine"

// CompositeRecorder delegates metrics recording to multiple backends.
type CompositeRecorder struct {
	recorders []engine.MetricsRecorder
}

// NewCompositeRecorder creates a recorder that fans out to all provided backends.
// If no recorders are provided, returns a NoopMetricsRecorder.
func NewCompositeRecorder(recorders ...engine.MetricsRecorder) engine.MetricsRecorder {
	if len(recorders) == 0 {
		return &engine.NoopMetricsRecorder{}
	}
	if len(recorders) == 1 {
		return recorders[0]
	}
	return &CompositeRecorder{recorders: recorders}
}

func (c *CompositeRecorder) RecordEvent(source string) {
	for _, r := range c.recorders {
		r.RecordEvent(source)
	}
}

func (c *CompositeRecorder) RecordCollectorError(source string) {
	for _, r := range c.recorders {
		r.RecordCollectorError(source)
	}
}

func (c *CompositeRecorder) RecordAnomaly() {
	for _, r := range c.recorders {
		r.RecordAnomaly()
	}
}

func (c *CompositeRecorder) RecordIncidentCreated() {
	for _, r := range c.recorders {
		r.RecordIncidentCreated()
	}
}

func (c *CompositeRecorder) RecordIncidentResolved() {
	for _, r := range c.recorders {
		r.RecordIncidentResolved()
	}
}

func (c *CompositeRecorder) RecordRemediation(success bool) {
	for _, r := range c.recorders {
		r.RecordRemediation(success)
	}
}

func (c *CompositeRecorder) SetMTTR(seconds float64) {
	for _, r := range c.recorders {
		r.SetMTTR(seconds)
	}
}

func (c *CompositeRecorder) RecordPolicyEvaluation() {
	for _, r := range c.recorders {
		r.RecordPolicyEvaluation()
	}
}

func (c *CompositeRecorder) RecordPolicyViolation() {
	for _, r := range c.recorders {
		r.RecordPolicyViolation()
	}
}

func (c *CompositeRecorder) SetComponentHealth(healthy, degraded, critical int) {
	for _, r := range c.recorders {
		r.SetComponentHealth(healthy, degraded, critical)
	}
}

func (c *CompositeRecorder) SetUptime(seconds float64) {
	for _, r := range c.recorders {
		r.SetUptime(seconds)
	}
}
