package plugin

import "sync"

type Registry struct {
	mu      sync.RWMutex
	plugins map[string]Plugin
}

func NewRegistry() *Registry {
	return &Registry{plugins: make(map[string]Plugin)}
}

func (r *Registry) Register(p Plugin) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.plugins[p.Name()] = p
}

func (r *Registry) Get(name string) (Plugin, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	p, ok := r.plugins[name]
	return p, ok
}

