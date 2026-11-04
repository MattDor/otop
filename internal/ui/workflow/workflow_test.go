package workflow

import (
	"testing"
	"time"

	uictx "github.com/mdoeren/otop/internal/ui/context"
	"github.com/mdoeren/otop/internal/ui/layout"
	"github.com/mdoeren/otop/internal/ui/panel"
	"github.com/rivo/tview"
)

// fakePanel implements panel.Panel, panel.Emitter and panel.Reporter.
type fakePanel struct {
	name     string
	box      *tview.Box
	subs     []string
	received []uictx.Context
	emitFn   func(uictx.Context)
	statusFn func(error)
	mounted  int
	unmount  int
}

func newFake(name string, subs ...string) *fakePanel {
	return &fakePanel{name: name, box: tview.NewBox(), subs: subs}
}

func (f *fakePanel) Name() string                     { return f.name }
func (f *fakePanel) Primitive() tview.Primitive       { return f.box }
func (f *fakePanel) Subscriptions() []string          { return f.subs }
func (f *fakePanel) OnContext(c uictx.Context)        { f.received = append(f.received, c) }
func (f *fakePanel) Refresh()                         {}
func (f *fakePanel) Mount()                           { f.mounted++ }
func (f *fakePanel) Unmount()                         { f.unmount++ }
func (f *fakePanel) SetEmitFn(fn func(uictx.Context)) { f.emitFn = fn }
func (f *fakePanel) SetStatusFn(fn func(error))       { f.statusFn = fn }

var _ panel.Panel = (*fakePanel)(nil)

// newTestWorkflow returns a workflow registered with a manager (so it has
// pages and a status bar). The app is never run.
func newTestWorkflow(t *testing.T) (*Workflow, *Manager, *tview.Application) {
	t.Helper()
	app := tview.NewApplication()
	m := NewManager(app)
	w := New("Test", app, nil, time.Hour)
	m.AddWorkflow(w)
	t.Cleanup(w.Stop)
	return w, m, app
}

func TestAddPanel_WiresLifecycleAndFocus(t *testing.T) {
	w, _, app := newTestWorkflow(t)
	a := newFake("A")
	b := newFake("B")

	w.AddPanel(a, nil, layout.Horizontal)
	w.AddPanel(b, a.Primitive(), layout.Vertical)

	for _, p := range []*fakePanel{a, b} {
		if p.mounted != 1 {
			t.Errorf("%s mounted %d times, want 1", p.name, p.mounted)
		}
		if p.emitFn == nil {
			t.Errorf("%s: emit function not wired", p.name)
		}
		if p.statusFn == nil {
			t.Errorf("%s: status function not wired", p.name)
		}
	}
	if app.GetFocus() != b.Primitive() || w.FocusedPrimitive() != b.Primitive() {
		t.Error("newly added panel should be focused")
	}
	if got := len(w.Panels()); got != 2 {
		t.Errorf("Panels() = %d, want 2", got)
	}
}

func TestSetStatusBar_WiresPanelsAddedEarlier(t *testing.T) {
	app := tview.NewApplication()
	w := New("Early", app, nil, time.Hour)
	p := newFake("A")
	w.AddPanel(p, nil, layout.Horizontal) // before AddWorkflow

	m := NewManager(app)
	m.AddWorkflow(w)
	t.Cleanup(w.Stop)

	if p.statusFn == nil {
		t.Error("panel added before AddWorkflow did not get a status function")
	}
}

func TestEmit_ReachesOnlySubscribedPanels(t *testing.T) {
	w, _, _ := newTestWorkflow(t)
	sqlOnly := newFake("SQL", "SQLContext")
	both := newFake("Both", "SessionContext", "SQLContext")
	none := newFake("None")
	w.AddPanel(sqlOnly, nil, layout.Horizontal)
	w.AddPanel(both, nil, layout.Horizontal)
	w.AddPanel(none, nil, layout.Horizontal)

	// Emitting through a panel's wired function is how panels talk to each other.
	none.emitFn(uictx.SessionContext{})
	w.Emit(uictx.SQLContext{SQLID: "x"})

	if len(sqlOnly.received) != 1 || len(both.received) != 2 || len(none.received) != 0 {
		t.Errorf("received: sqlOnly=%d both=%d none=%d, want 1 2 0",
			len(sqlOnly.received), len(both.received), len(none.received))
	}
}

func TestRemovePanel_UnsubscribesUnmountsAndMovesFocus(t *testing.T) {
	w, _, app := newTestWorkflow(t)
	a := newFake("A", "SQLContext")
	b := newFake("B", "SQLContext")
	w.AddPanel(a, nil, layout.Horizontal)
	w.AddPanel(b, a.Primitive(), layout.Vertical)

	w.RemovePanel(b)

	if b.unmount != 1 {
		t.Errorf("B unmounted %d times, want 1", b.unmount)
	}
	w.Emit(uictx.SQLContext{})
	if len(b.received) != 0 {
		t.Error("removed panel still receives context")
	}
	if len(a.received) != 1 {
		t.Error("remaining panel stopped receiving context")
	}
	if got := w.Panels(); len(got) != 1 || got[0] != panel.Panel(a) {
		t.Errorf("Panels() after remove = %v", got)
	}
	if app.GetFocus() != a.Primitive() {
		t.Error("focus should move to the remaining panel")
	}
}

func TestFocusAndFocusCycle(t *testing.T) {
	w, _, app := newTestWorkflow(t)
	a, b, c := newFake("A"), newFake("B"), newFake("C")
	w.AddPanel(a, nil, layout.Horizontal)
	w.AddPanel(b, nil, layout.Horizontal)
	w.AddPanel(c, nil, layout.Horizontal)

	w.Focus(a)
	if app.GetFocus() != a.Primitive() || w.FocusedPrimitive() != a.Primitive() {
		t.Fatal("Focus(a) did not focus A")
	}

	steps := []struct {
		reverse bool
		want    *fakePanel
	}{
		{false, b}, {false, c}, {false, a}, // forward wraps
		{true, c}, {true, b}, // backward wraps
	}
	for i, s := range steps {
		w.FocusCycle(s.reverse)
		if app.GetFocus() != s.want.Primitive() {
			t.Errorf("step %d: focus on wrong panel, want %s", i, s.want.name)
		}
	}
}

func TestManager_SwitchWorkflows(t *testing.T) {
	app := tview.NewApplication()
	m := NewManager(app)
	w1 := New("One", app, nil, time.Hour)
	w2 := New("Two", app, nil, time.Hour)
	m.AddWorkflow(w1)
	m.AddWorkflow(w2)
	t.Cleanup(func() { w1.Stop(); w2.Stop() })

	if m.ActiveWorkflow() != w1 {
		t.Fatal("first workflow should be active")
	}
	m.SwitchNext()
	if m.ActiveWorkflow() != w2 || w1.active || !w2.active {
		t.Error("SwitchNext should activate w2 and stop w1")
	}
	m.SwitchNext()
	if m.ActiveWorkflow() != w1 {
		t.Error("SwitchNext should wrap to w1")
	}
	m.SwitchPrev()
	if m.ActiveWorkflow() != w2 {
		t.Error("SwitchPrev should wrap to w2")
	}
	m.SwitchTo(7) // out of range: ignored
	if m.ActiveWorkflow() != w2 || m.Count() != 2 {
		t.Error("SwitchTo(out of range) changed state")
	}
}
