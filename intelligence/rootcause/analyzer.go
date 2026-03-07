package rootcause

import (
	"context"
	"fmt"
	"log"
	"strings"
	"sync"
	"time"

	"github.com/kronveil/kronveil/core/engine"
)

// RootCause represents the identified root cause of an incident.
type RootCause struct {
	Summary     string          `json:"summary"`
	CausalChain []ChainLink     `json:"causal_chain"`
	Evidence    []Evidence      `json:"evidence"`
	Confidence  float64         `json:"confidence"`
	Timestamp   time.Time       `json:"timestamp"`
}

// ChainLink is a single step in the causal chain.
type ChainLink struct {
	Service     string `json:"service"`
	Issue       string `json:"issue"`
	Impact      string `json:"impact"`
}

// Evidence supporting the root cause analysis.
type Evidence struct {
	Type        string      `json:"type"` // "metric", "log", "event", "trace"
	Source      string      `json:"source"`
	Description string      `json:"description"`
	Value       interface{} `json:"value,omitempty"`
	Timestamp   time.Time   `json:"timestamp"`
}

// Analyzer performs LLM-powered root cause analysis.
type Analyzer struct {
	graph  *DependencyGraph
	llm    engine.LLMProvider
	mu     sync.RWMutex
	cache  map[string]*RootCause
	running bool
	cancel  context.CancelFunc
}

// New creates a new root cause analyzer.
func New(graph *DependencyGraph, llm engine.LLMProvider) *Analyzer {
	return &Analyzer{
		graph: graph,
		llm:   llm,
		cache: make(map[string]*RootCause),
	}
}

func (a *Analyzer) Name() string { return "root-cause-analyzer" }

func (a *Analyzer) Start(ctx context.Context) error {
	a.mu.Lock()
	a.running = true
	_, a.cancel = context.WithCancel(ctx)
	a.mu.Unlock()
	log.Println("[rootcause] Root cause analyzer started")
	return nil
}

func (a *Analyzer) Stop() error {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.running = false
	if a.cancel != nil {
		a.cancel()
	}
	return nil
}

func (a *Analyzer) Analyze(ctx context.Context, event *engine.TelemetryEvent) error {
	// Only analyze high/critical severity events.
	if event.Severity != engine.SeverityCritical && event.Severity != engine.SeverityHigh {
		return nil
	}

	go func() {
		rc, err := a.AnalyzeIncident(ctx, event.Source, []engine.TelemetryEvent{*event})
		if err != nil {
			log.Printf("[rootcause] Analysis error: %v", err)
			return
		}
		a.mu.Lock()
		a.cache[event.ID] = rc
		a.mu.Unlock()
		log.Printf("[rootcause] Root cause identified for %s: %s (confidence: %.0f%%)",
			event.ID, rc.Summary, rc.Confidence*100)
	}()

	return nil
}

func (a *Analyzer) Health() engine.ComponentHealth {
	return engine.ComponentHealth{
		Name:      "root-cause-analyzer",
		Status:    "healthy",
		Message:   fmt.Sprintf("%d analyses cached", len(a.cache)),
		LastCheck: time.Now(),
	}
}

// AnalyzeIncident performs root cause analysis for an incident.
func (a *Analyzer) AnalyzeIncident(ctx context.Context, service string, events []engine.TelemetryEvent) (*RootCause, error) {
	// Step 1: Get the causal chain from the dependency graph.
	chain := a.graph.CausalChain(service, func(s string) bool {
		return true // In production: checks actual service health.
	})

	// Step 2: Get impacted services.
	impacted := a.graph.ImpactAnalysis(service)

	// Step 3: Build evidence from events.
	var evidence []Evidence
	for _, event := range events {
		evidence = append(evidence, Evidence{
			Type:        "event",
			Source:      event.Source,
			Description: fmt.Sprintf("%s: %s", event.Type, event.Severity),
			Value:       event.Payload,
			Timestamp:   event.Timestamp,
		})
	}

	// Step 4: Use LLM for analysis if available.
	if a.llm != nil {
		return a.llmAnalysis(ctx, service, chain, impacted, evidence, events)
	}

	// Step 5: Heuristic-based analysis as fallback.
	return a.heuristicAnalysis(service, chain, impacted, evidence), nil
}

func (a *Analyzer) llmAnalysis(ctx context.Context, service string, chain, impacted []string, evidence []Evidence, events []engine.TelemetryEvent) (*RootCause, error) {
	prompt := buildAnalysisPrompt(service, chain, impacted, evidence, events)

	response, err := a.llm.InvokeWithSystem(ctx,
		"You are Kronveil, an AI infrastructure observability agent. Analyze the following infrastructure incident and provide root cause analysis. Be concise and specific.",
		prompt,
	)
	if err != nil {
		// Fall back to heuristic analysis.
		log.Printf("[rootcause] LLM analysis failed, using heuristic: %v", err)
		return a.heuristicAnalysis(service, chain, impacted, evidence), nil
	}

	return &RootCause{
		Summary:    response,
		CausalChain: buildChainLinks(chain),
		Evidence:   evidence,
		Confidence: 0.85,
		Timestamp:  time.Now(),
	}, nil
}

func (a *Analyzer) heuristicAnalysis(service string, chain, impacted []string, evidence []Evidence) *RootCause {
	summary := fmt.Sprintf("Service '%s' experienced a failure", service)
	if len(chain) > 1 {
		summary = fmt.Sprintf("Root cause: '%s' failure cascaded through dependency chain: %s",
			chain[len(chain)-1], strings.Join(chain, " -> "))
	}

	confidence := 0.6
	if len(evidence) > 3 {
		confidence = 0.75
	}

	return &RootCause{
		Summary:     summary,
		CausalChain: buildChainLinks(chain),
		Evidence:    evidence,
		Confidence:  confidence,
		Timestamp:   time.Now(),
	}
}

func buildAnalysisPrompt(service string, chain, impacted []string, evidence []Evidence, events []engine.TelemetryEvent) string {
	var b strings.Builder
	b.WriteString("## Incident Analysis Request\n\n")
	b.WriteString(fmt.Sprintf("**Failing Service:** %s\n\n", service))

	if len(chain) > 0 {
		b.WriteString("**Dependency Chain:**\n")
		for _, s := range chain {
			b.WriteString(fmt.Sprintf("- %s\n", s))
		}
		b.WriteString("\n")
	}

	if len(impacted) > 0 {
		b.WriteString("**Impacted Services:**\n")
		for _, s := range impacted {
			b.WriteString(fmt.Sprintf("- %s\n", s))
		}
		b.WriteString("\n")
	}

	b.WriteString("**Events:**\n")
	for _, e := range events {
		b.WriteString(fmt.Sprintf("- [%s] %s from %s: %v\n",
			e.Severity, e.Type, e.Source, e.Payload))
	}

	b.WriteString("\nProvide: 1) Root cause summary 2) Causal chain 3) Recommended remediation")
	return b.String()
}

func buildChainLinks(chain []string) []ChainLink {
	links := make([]ChainLink, len(chain))
	for i, s := range chain {
		links[i] = ChainLink{
			Service: s,
			Issue:   "failure detected",
			Impact:  "cascading to dependents",
		}
	}
	return links
}

// GetAnalysis returns a cached root cause analysis.
func (a *Analyzer) GetAnalysis(eventID string) (*RootCause, bool) {
	a.mu.RLock()
	defer a.mu.RUnlock()
	rc, ok := a.cache[eventID]
	return rc, ok
}
