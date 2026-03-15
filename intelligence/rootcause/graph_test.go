package rootcause

import (
	"sort"
	"testing"
)

func newTestGraph() *DependencyGraph {
	g := NewDependencyGraph()
	g.AddNode(&ServiceNode{Name: "frontend"})
	g.AddNode(&ServiceNode{Name: "api"})
	g.AddNode(&ServiceNode{Name: "db"})
	g.AddNode(&ServiceNode{Name: "cache"})
	return g
}

func TestAddNode_AddEdge(t *testing.T) {
	g := newTestGraph()

	if err := g.AddEdge("frontend", "api"); err != nil {
		t.Fatalf("AddEdge failed: %v", err)
	}
	if err := g.AddEdge("api", "db"); err != nil {
		t.Fatalf("AddEdge failed: %v", err)
	}

	nodes := g.Nodes()
	if len(nodes) != 4 {
		t.Errorf("expected 4 nodes, got %d", len(nodes))
	}
}

func TestAddEdge_MissingNode(t *testing.T) {
	g := NewDependencyGraph()
	g.AddNode(&ServiceNode{Name: "a"})

	if err := g.AddEdge("a", "missing"); err == nil {
		t.Error("expected error for missing target node")
	}
	if err := g.AddEdge("missing", "a"); err == nil {
		t.Error("expected error for missing source node")
	}
}

func TestDependencies(t *testing.T) {
	g := newTestGraph()
	_ = g.AddEdge("frontend", "api")
	_ = g.AddEdge("frontend", "cache")

	deps := g.Dependencies("frontend")
	sort.Strings(deps)
	if len(deps) != 2 {
		t.Fatalf("expected 2 deps, got %d", len(deps))
	}
	if deps[0] != "api" || deps[1] != "cache" {
		t.Errorf("unexpected deps: %v", deps)
	}
}

func TestDependents(t *testing.T) {
	g := newTestGraph()
	_ = g.AddEdge("frontend", "api")
	_ = g.AddEdge("api", "db")

	dependents := g.Dependents("api")
	if len(dependents) != 1 || dependents[0] != "frontend" {
		t.Errorf("expected [frontend], got %v", dependents)
	}

	dependents = g.Dependents("db")
	if len(dependents) != 1 || dependents[0] != "api" {
		t.Errorf("expected [api], got %v", dependents)
	}
}

func TestImpactAnalysis(t *testing.T) {
	g := newTestGraph()
	_ = g.AddEdge("frontend", "api")
	_ = g.AddEdge("api", "db")

	// If db fails, api and frontend are impacted.
	impacted := g.ImpactAnalysis("db")
	sort.Strings(impacted)
	if len(impacted) != 2 {
		t.Fatalf("expected 2 impacted, got %d: %v", len(impacted), impacted)
	}
	if impacted[0] != "api" || impacted[1] != "frontend" {
		t.Errorf("unexpected impacted: %v", impacted)
	}
}

func TestCausalChain_AllUnhealthy(t *testing.T) {
	g := newTestGraph()
	_ = g.AddEdge("frontend", "api")
	_ = g.AddEdge("api", "db")

	// All unhealthy.
	chain := g.CausalChain("frontend", func(s string) bool { return false })
	if len(chain) != 3 {
		t.Errorf("expected 3 in chain, got %d: %v", len(chain), chain)
	}
}

func TestCausalChain_AllHealthy(t *testing.T) {
	g := newTestGraph()
	_ = g.AddEdge("frontend", "api")
	_ = g.AddEdge("api", "db")

	// All healthy → empty chain.
	chain := g.CausalChain("frontend", func(s string) bool { return true })
	if len(chain) != 0 {
		t.Errorf("expected empty chain for all healthy, got %v", chain)
	}
}

func TestTopologicalSort_DAG(t *testing.T) {
	g := newTestGraph()
	_ = g.AddEdge("frontend", "api")
	_ = g.AddEdge("api", "db")
	_ = g.AddEdge("api", "cache")

	sorted, err := g.TopologicalSort()
	if err != nil {
		t.Fatalf("TopologicalSort failed: %v", err)
	}
	if len(sorted) != 4 {
		t.Fatalf("expected 4 sorted nodes, got %d", len(sorted))
	}

	// frontend should come before api, api before db and cache.
	indexOf := map[string]int{}
	for i, s := range sorted {
		indexOf[s] = i
	}
	if indexOf["frontend"] > indexOf["api"] {
		t.Error("frontend should come before api")
	}
	if indexOf["api"] > indexOf["db"] {
		t.Error("api should come before db")
	}
}

func TestTopologicalSort_CycleDetection(t *testing.T) {
	g := NewDependencyGraph()
	g.AddNode(&ServiceNode{Name: "a"})
	g.AddNode(&ServiceNode{Name: "b"})
	g.AddNode(&ServiceNode{Name: "c"})
	_ = g.AddEdge("a", "b")
	_ = g.AddEdge("b", "c")
	_ = g.AddEdge("c", "a")

	_, err := g.TopologicalSort()
	if err == nil {
		t.Error("expected cycle detection error")
	}
}

func TestRemoveEdge(t *testing.T) {
	g := newTestGraph()
	_ = g.AddEdge("frontend", "api")

	deps := g.Dependencies("frontend")
	if len(deps) != 1 {
		t.Fatal("edge should exist before removal")
	}

	g.RemoveEdge("frontend", "api")
	deps = g.Dependencies("frontend")
	if len(deps) != 0 {
		t.Error("edge should be removed")
	}
}
