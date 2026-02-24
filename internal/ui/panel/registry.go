package panel

import "sync"

// Entry describes a panel type available in the command palette.
type Entry struct {
	TypeName    string
	Description string
	Factory     Factory
}

// Registry is a thread-safe collection of panel factories.
type Registry struct {
	mu      sync.RWMutex
	entries []Entry
}

// Global is the application-wide panel registry populated by panel init() calls.
var Global = &Registry{}

// Register adds an entry to the registry. Called from panel init() functions.
func (r *Registry) Register(e Entry) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.entries = append(r.entries, e)
}

// All returns a snapshot of all registered entries.
func (r *Registry) All() []Entry {
	r.mu.RLock()
	defer r.mu.RUnlock()
	out := make([]Entry, len(r.entries))
	copy(out, r.entries)
	return out
}

// Get looks up an entry by type name.
func (r *Registry) Get(typeName string) (Entry, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	for _, e := range r.entries {
		if e.TypeName == typeName {
			return e, true
		}
	}
	return Entry{}, false
}
