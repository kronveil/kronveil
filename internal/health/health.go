package health

import (
	"encoding/json"
	"net/http"
	"sync"
	"time"
)

// Checker aggregates health checks from all components.
type Checker struct {
	mu     sync.RWMutex
	checks map[string]CheckFunc
}

// CheckFunc is a function that returns the health status of a component.
type CheckFunc func() Status

// Status represents a component's health status.
type Status struct {
	Healthy bool   `json:"healthy"`
	Message string `json:"message"`
}

// Response is the health check HTTP response.
type Response struct {
	Status     string            `json:"status"`
	Components map[string]Status `json:"components"`
	Timestamp  time.Time         `json:"timestamp"`
}

// NewChecker creates a new health checker.
func NewChecker() *Checker {
	return &Checker{
		checks: make(map[string]CheckFunc),
	}
}

// Register adds a health check for a component.
func (c *Checker) Register(name string, check CheckFunc) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.checks[name] = check
}

// Check runs all health checks and returns the aggregate result.
func (c *Checker) Check() Response {
	c.mu.RLock()
	defer c.mu.RUnlock()

	components := make(map[string]Status)
	allHealthy := true

	for name, check := range c.checks {
		status := check()
		components[name] = status
		if !status.Healthy {
			allHealthy = false
		}
	}

	overallStatus := "healthy"
	if !allHealthy {
		overallStatus = "degraded"
	}

	return Response{
		Status:     overallStatus,
		Components: components,
		Timestamp:  time.Now(),
	}
}

// HTTPHandler returns an http.HandlerFunc for the /healthz endpoint.
func (c *Checker) HTTPHandler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		result := c.Check()
		w.Header().Set("Content-Type", "application/json")
		if result.Status != "healthy" {
			w.WriteHeader(http.StatusServiceUnavailable)
		}
		json.NewEncoder(w).Encode(result)
	}
}

// ReadinessHandler returns an http.HandlerFunc for the /readyz endpoint.
func (c *Checker) ReadinessHandler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		result := c.Check()
		w.Header().Set("Content-Type", "application/json")
		if result.Status != "healthy" {
			w.WriteHeader(http.StatusServiceUnavailable)
		}
		json.NewEncoder(w).Encode(result)
	}
}
