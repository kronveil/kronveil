package engine

import (
	"fmt"
	"sync"
)

// Registry manages dynamic registration of collectors, intelligence modules, and integrations.
type Registry struct {
	mu            sync.RWMutex
	collectors    map[string]Collector
	modules       map[string]IntelligenceModule
	integrations  map[string]Integration
	notifiers     map[string]Notifier
}

// NewRegistry creates a new component registry.
func NewRegistry() *Registry {
	return &Registry{
		collectors:   make(map[string]Collector),
		modules:      make(map[string]IntelligenceModule),
		integrations: make(map[string]Integration),
		notifiers:    make(map[string]Notifier),
	}
}

// RegisterCollector adds a collector to the registry.
func (r *Registry) RegisterCollector(c Collector) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if _, exists := r.collectors[c.Name()]; exists {
		return fmt.Errorf("collector %q already registered", c.Name())
	}
	r.collectors[c.Name()] = c
	return nil
}

// RegisterModule adds an intelligence module to the registry.
func (r *Registry) RegisterModule(m IntelligenceModule) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if _, exists := r.modules[m.Name()]; exists {
		return fmt.Errorf("module %q already registered", m.Name())
	}
	r.modules[m.Name()] = m
	return nil
}

// RegisterIntegration adds an integration to the registry.
func (r *Registry) RegisterIntegration(i Integration) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if _, exists := r.integrations[i.Name()]; exists {
		return fmt.Errorf("integration %q already registered", i.Name())
	}
	r.integrations[i.Name()] = i
	return nil
}

// RegisterNotifier adds a notifier to the registry.
func (r *Registry) RegisterNotifier(n Notifier) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	name := fmt.Sprintf("%T", n)
	if _, exists := r.notifiers[name]; exists {
		return fmt.Errorf("notifier %q already registered", name)
	}
	r.notifiers[name] = n
	return nil
}

// Collectors returns all registered collectors.
func (r *Registry) Collectors() []Collector {
	r.mu.RLock()
	defer r.mu.RUnlock()
	result := make([]Collector, 0, len(r.collectors))
	for _, c := range r.collectors {
		result = append(result, c)
	}
	return result
}

// Modules returns all registered intelligence modules.
func (r *Registry) Modules() []IntelligenceModule {
	r.mu.RLock()
	defer r.mu.RUnlock()
	result := make([]IntelligenceModule, 0, len(r.modules))
	for _, m := range r.modules {
		result = append(result, m)
	}
	return result
}

// Integrations returns all registered integrations.
func (r *Registry) Integrations() []Integration {
	r.mu.RLock()
	defer r.mu.RUnlock()
	result := make([]Integration, 0, len(r.integrations))
	for _, i := range r.integrations {
		result = append(result, i)
	}
	return result
}

// Notifiers returns all registered notifiers.
func (r *Registry) Notifiers() []Notifier {
	r.mu.RLock()
	defer r.mu.RUnlock()
	result := make([]Notifier, 0, len(r.notifiers))
	for _, n := range r.notifiers {
		result = append(result, n)
	}
	return result
}

// GetCollector returns a collector by name.
func (r *Registry) GetCollector(name string) (Collector, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	c, ok := r.collectors[name]
	return c, ok
}

// GetModule returns a module by name.
func (r *Registry) GetModule(name string) (IntelligenceModule, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	m, ok := r.modules[name]
	return m, ok
}

// Health returns health status for all registered components.
func (r *Registry) Health() []ComponentHealth {
	r.mu.RLock()
	defer r.mu.RUnlock()
	var health []ComponentHealth
	for _, c := range r.collectors {
		health = append(health, c.Health())
	}
	for _, m := range r.modules {
		health = append(health, m.Health())
	}
	for _, i := range r.integrations {
		health = append(health, i.Health())
	}
	return health
}
