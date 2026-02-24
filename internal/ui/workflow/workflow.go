package workflow

import (
	"time"

	"github.com/mdoeren/otop/internal/db"
	uictx "github.com/mdoeren/otop/internal/ui/context"
	"github.com/mdoeren/otop/internal/ui/layout"
	"github.com/mdoeren/otop/internal/ui/panel"
	"github.com/rivo/tview"
)

// Workflow is a named tab unit that owns a layout tree, a context bus,
// and a periodic refresh ticker for its panels.
type Workflow struct {
	Name            string
	app             *tview.Application
	db              *db.DB
	bus             *uictx.Bus
	root            *layout.Node
	panels          []panel.Panel
	unsubs          map[panel.Panel][]func()
	focusOrder      []tview.Primitive
	focusIdx        int
	refreshInterval time.Duration
	stopCh          chan struct{}
	active          bool
	pages           *tview.Pages
	pageKey         string
}

// New creates a Workflow with the given name and refresh interval.
func New(name string, app *tview.Application, database *db.DB, refreshInterval time.Duration) *Workflow {
	return &Workflow{
		Name:            name,
		app:             app,
		db:              database,
		bus:             uictx.NewBus(),
		root:            &layout.Node{Direction: layout.Horizontal},
		unsubs:          make(map[panel.Panel][]func()),
		refreshInterval: refreshInterval,
		pageKey:         name,
	}
}

// AddPanel adds p to the workflow layout.
// If splitTarget is nil, the panel is appended to the root split.
// Otherwise it is inserted adjacent to splitTarget in the given direction.
func (w *Workflow) AddPanel(p panel.Panel, splitTarget tview.Primitive, dir layout.Direction) {
	leaf := &layout.Node{Primitive: p.Primitive(), PanelName: p.Name()}

	if len(w.root.Children) == 0 || splitTarget == nil {
		w.root.AddChild(leaf, 1)
	} else {
		if !layout.InsertNearTarget(w.root, splitTarget, leaf, dir) {
			// Fallback: append to root
			w.root.AddChild(leaf, 1)
		}
	}

	// Subscribe panel to its requested context types
	var unsubs []func()
	for _, typeName := range p.Subscriptions() {
		tn := typeName
		p_ := p
		unsub := w.bus.Subscribe(tn, func(ctx uictx.Context) {
			p_.OnContext(ctx)
		})
		unsubs = append(unsubs, unsub)
	}
	w.unsubs[p] = unsubs

	// Wire emit function if the panel can emit
	if e, ok := p.(panel.Emitter); ok {
		e.SetEmitFn(w.bus.Emit)
	}

	w.panels = append(w.panels, p)
	p.Mount()
	w.rebuild()

	// Focus the newly added panel
	w.app.SetFocus(p.Primitive())
	for i, prim := range w.focusOrder {
		if prim == p.Primitive() {
			w.focusIdx = i
			break
		}
	}
}

// RemovePanel removes p from the workflow layout and cancels its subscriptions.
func (w *Workflow) RemovePanel(p panel.Panel) {
	// Cancel subscriptions
	for _, unsub := range w.unsubs[p] {
		unsub()
	}
	delete(w.unsubs, p)

	// Remove from panels slice
	for i, existing := range w.panels {
		if existing == p {
			w.panels = append(w.panels[:i], w.panels[i+1:]...)
			break
		}
	}

	// Remove from layout tree
	w.root.RemoveChild(p.Primitive())
	layout.Collapse(w.root)

	p.Unmount()
	w.rebuild()

	// Shift focus if needed
	if len(w.focusOrder) > 0 {
		if w.focusIdx >= len(w.focusOrder) {
			w.focusIdx = len(w.focusOrder) - 1
		}
		w.app.SetFocus(w.focusOrder[w.focusIdx])
	}
}

// Emit publishes ctx to all panels subscribed to its type.
func (w *Workflow) Emit(ctx uictx.Context) {
	w.bus.Emit(ctx)
}

// FocusCycle moves focus to the next (or previous) panel.
func (w *Workflow) FocusCycle(reverse bool) {
	if len(w.focusOrder) == 0 {
		return
	}
	if reverse {
		w.focusIdx--
		if w.focusIdx < 0 {
			w.focusIdx = len(w.focusOrder) - 1
		}
	} else {
		w.focusIdx = (w.focusIdx + 1) % len(w.focusOrder)
	}
	w.app.SetFocus(w.focusOrder[w.focusIdx])
}

// ResizeFocused adjusts the proportion of the focused panel along dir.
func (w *Workflow) ResizeFocused(dir layout.Direction, delta int) {
	if w.focusIdx < 0 || w.focusIdx >= len(w.focusOrder) {
		return
	}
	target := w.focusOrder[w.focusIdx]
	resizeInTree(w.root, target, dir, delta)
	w.rebuild()
}

// FocusedPrimitive returns the currently focused primitive, or nil.
func (w *Workflow) FocusedPrimitive() tview.Primitive {
	if w.focusIdx >= 0 && w.focusIdx < len(w.focusOrder) {
		return w.focusOrder[w.focusIdx]
	}
	return nil
}

// Panels returns the current panel list (snapshot).
func (w *Workflow) Panels() []panel.Panel {
	out := make([]panel.Panel, len(w.panels))
	copy(out, w.panels)
	return out
}

// SetPages wires the workflow into the given tview.Pages (one page per workflow).
func (w *Workflow) SetPages(pages *tview.Pages) {
	w.pages = pages
	w.rebuild()
}

// Start activates the periodic refresh ticker. Called when the tab becomes active.
func (w *Workflow) Start() {
	if w.stopCh != nil {
		return
	}
	w.active = true
	w.stopCh = make(chan struct{})
	ticker := time.NewTicker(w.refreshInterval)
	go func() {
		defer ticker.Stop()
		for {
			select {
			case <-ticker.C:
				w.app.QueueUpdateDraw(func() {
					for _, p := range w.panels {
						p.Refresh()
					}
				})
			case <-w.stopCh:
				return
			}
		}
	}()
}

// Stop deactivates the refresh ticker. Called when switching away from the tab.
func (w *Workflow) Stop() {
	w.active = false
	if w.stopCh != nil {
		close(w.stopCh)
		w.stopCh = nil
	}
}

// rebuild reconstructs the tview.Flex tree from the layout node tree
// and updates the workflow's page in the manager's Pages widget.
func (w *Workflow) rebuild() {
	w.focusOrder = layout.FocusOrder(w.root)

	if w.pages == nil {
		return
	}
	w.pages.RemovePage(w.pageKey)
	primitive := layout.Build(w.root)
	w.pages.AddPage(w.pageKey, primitive, true, w.active)
	if w.active {
		w.pages.SwitchToPage(w.pageKey)
	}
}

// resizeInTree adjusts the proportion of the leaf matching target along dir.
func resizeInTree(node *layout.Node, target tview.Primitive, dir layout.Direction, delta int) bool {
	for i, child := range node.Children {
		if child.IsLeaf() && child.Primitive == target {
			if node.Direction == dir {
				node.ResizeChild(i, delta)
			}
			return true
		}
		if !child.IsLeaf() && resizeInTree(child, target, dir, delta) {
			return true
		}
	}
	return false
}
