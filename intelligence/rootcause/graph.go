package rootcause

import (
	"fmt"
	"sync"
)

// DependencyGraph represents service dependencies for causal chain analysis.
type DependencyGraph struct {
	mu    sync.RWMutex
	nodes map[string]*ServiceNode
	edges map[string]map[string]bool // from -> set of to
}

// ServiceNode represents a service in the dependency graph.
type ServiceNode struct {
	Name      string            `json:"name"`
	Type      string            `json:"type"` // "deployment", "statefulset", "service", "topic"
	Namespace string            `json:"namespace"`
	Labels    map[string]string `json:"labels"`
}

// NewDependencyGraph creates an empty dependency graph.
func NewDependencyGraph() *DependencyGraph {
	return &DependencyGraph{
		nodes: make(map[string]*ServiceNode),
		edges: make(map[string]map[string]bool),
	}
}

// AddNode adds a service node to the graph.
func (g *DependencyGraph) AddNode(node *ServiceNode) {
	g.mu.Lock()
	defer g.mu.Unlock()
	g.nodes[node.Name] = node
	if _, ok := g.edges[node.Name]; !ok {
		g.edges[node.Name] = make(map[string]bool)
	}
}

// AddEdge adds a dependency edge (from depends on to).
func (g *DependencyGraph) AddEdge(from, to string) error {
	g.mu.Lock()
	defer g.mu.Unlock()

	if _, ok := g.nodes[from]; !ok {
		return fmt.Errorf("node %q not found", from)
	}
	if _, ok := g.nodes[to]; !ok {
		return fmt.Errorf("node %q not found", to)
	}

	if _, ok := g.edges[from]; !ok {
		g.edges[from] = make(map[string]bool)
	}
	g.edges[from][to] = true
	return nil
}

// RemoveEdge removes a dependency edge.
func (g *DependencyGraph) RemoveEdge(from, to string) {
	g.mu.Lock()
	defer g.mu.Unlock()
	if deps, ok := g.edges[from]; ok {
		delete(deps, to)
	}
}

// Dependencies returns the direct dependencies of a service.
func (g *DependencyGraph) Dependencies(name string) []string {
	g.mu.RLock()
	defer g.mu.RUnlock()
	deps, ok := g.edges[name]
	if !ok {
		return nil
	}
	result := make([]string, 0, len(deps))
	for dep := range deps {
		result = append(result, dep)
	}
	return result
}

// Dependents returns services that depend on the given service.
func (g *DependencyGraph) Dependents(name string) []string {
	g.mu.RLock()
	defer g.mu.RUnlock()
	var dependents []string
	for from, deps := range g.edges {
		if deps[name] {
			dependents = append(dependents, from)
		}
	}
	return dependents
}

// ImpactAnalysis returns all services transitively affected by a failure of the given service.
func (g *DependencyGraph) ImpactAnalysis(failedService string) []string {
	g.mu.RLock()
	defer g.mu.RUnlock()

	visited := make(map[string]bool)
	var result []string

	var dfs func(name string)
	dfs = func(name string) {
		for from, deps := range g.edges {
			if deps[name] && !visited[from] {
				visited[from] = true
				result = append(result, from)
				dfs(from)
			}
		}
	}

	visited[failedService] = true
	dfs(failedService)
	return result
}

// CausalChain finds the dependency chain from a failing service down to potential root causes.
func (g *DependencyGraph) CausalChain(failedService string, isHealthy func(string) bool) []string {
	g.mu.RLock()
	defer g.mu.RUnlock()

	visited := make(map[string]bool)
	var chain []string

	var dfs func(name string)
	dfs = func(name string) {
		if visited[name] {
			return
		}
		visited[name] = true

		if !isHealthy(name) {
			chain = append(chain, name)
		}

		deps, ok := g.edges[name]
		if !ok {
			return
		}
		for dep := range deps {
			dfs(dep)
		}
	}

	dfs(failedService)
	return chain
}

// TopologicalSort returns nodes in dependency order.
func (g *DependencyGraph) TopologicalSort() ([]string, error) {
	g.mu.RLock()
	defer g.mu.RUnlock()

	inDegree := make(map[string]int)
	for name := range g.nodes {
		inDegree[name] = 0
	}
	for _, deps := range g.edges {
		for dep := range deps {
			inDegree[dep]++
		}
	}

	var queue []string
	for name, deg := range inDegree {
		if deg == 0 {
			queue = append(queue, name)
		}
	}

	var sorted []string
	for len(queue) > 0 {
		node := queue[0]
		queue = queue[1:]
		sorted = append(sorted, node)

		for dep := range g.edges[node] {
			inDegree[dep]--
			if inDegree[dep] == 0 {
				queue = append(queue, dep)
			}
		}
	}

	if len(sorted) != len(g.nodes) {
		return nil, fmt.Errorf("cycle detected in dependency graph")
	}

	return sorted, nil
}

// Nodes returns all nodes in the graph.
func (g *DependencyGraph) Nodes() []*ServiceNode {
	g.mu.RLock()
	defer g.mu.RUnlock()
	result := make([]*ServiceNode, 0, len(g.nodes))
	for _, n := range g.nodes {
		result = append(result, n)
	}
	return result
}
