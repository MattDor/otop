package panel

import (
	"github.com/mdoeren/otop/internal/db"
	uictx "github.com/mdoeren/otop/internal/ui/context"
	"github.com/rivo/tview"
)

// Panel is the lifecycle interface every UI panel must implement.
type Panel interface {
	// Name returns the panel's registered type name.
	Name() string

	// Primitive returns the tview widget for this panel.
	Primitive() tview.Primitive

	// Subscriptions returns the context type names this panel wants to receive.
	Subscriptions() []string

	// OnContext is called by the bus when a subscribed context is emitted.
	// Must not block; spawn a goroutine for I/O and use QueueUpdateDraw.
	OnContext(ctx uictx.Context)

	// Refresh is called periodically inside QueueUpdateDraw.
	Refresh()

	// Mount is called once after the panel is added to a workflow.
	Mount()

	// Unmount is called before the panel is removed; cancel goroutines here.
	Unmount()
}

// Emitter is an optional interface. If a Panel also implements Emitter, the
// workflow wires up the emit function so the panel can publish context.
type Emitter interface {
	SetEmitFn(fn func(uictx.Context))
}

// Reporter is an optional interface. If a Panel also implements Reporter, the
// workflow wires up a status function so the panel can surface DB errors.
type Reporter interface {
	SetStatusFn(fn func(error))
}

// Factory creates a new Panel instance.
type Factory func(app *tview.Application, db *db.DB) Panel
