package context

import "sync"

// Handler is a function that receives a Context value.
type Handler func(Context)

type entry struct {
	id int
	fn Handler
}

// Bus is a workflow-scoped pub/sub bus with "last writer wins" semantics.
// Emit dispatches synchronously to all registered handlers.
type Bus struct {
	mu     sync.RWMutex
	subs   map[string][]entry
	nextID int
}

// NewBus creates a new Bus.
func NewBus() *Bus {
	return &Bus{subs: make(map[string][]entry)}
}

// Subscribe registers h to receive contexts of the given type name.
// Returns an unsubscribe function.
func (b *Bus) Subscribe(typeName string, h Handler) func() {
	b.mu.Lock()
	id := b.nextID
	b.nextID++
	b.subs[typeName] = append(b.subs[typeName], entry{id: id, fn: h})
	b.mu.Unlock()

	return func() {
		b.mu.Lock()
		defer b.mu.Unlock()
		s := b.subs[typeName]
		for i, e := range s {
			if e.id == id {
				b.subs[typeName] = append(s[:i], s[i+1:]...)
				return
			}
		}
	}
}

// Emit dispatches ctx to all handlers registered for its concrete type.
// Safe to call from any goroutine.
func (b *Bus) Emit(ctx Context) {
	typeName := ctx.contextType()
	b.mu.RLock()
	s := make([]entry, len(b.subs[typeName]))
	copy(s, b.subs[typeName])
	b.mu.RUnlock()

	for _, e := range s {
		e.fn(ctx)
	}
}

// Subscribe is a generic helper that avoids string literals at call sites.
// T must be a type defined in this package (SessionContext or SQLContext).
func Subscribe[T Context](b *Bus, h func(T)) func() {
	var zero T
	typeName := zero.contextType()
	return b.Subscribe(typeName, func(c Context) {
		if v, ok := c.(T); ok {
			h(v)
		}
	})
}
