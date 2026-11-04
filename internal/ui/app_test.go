package ui

import (
	"testing"
	"time"

	"github.com/gdamore/tcell/v2"

	"github.com/mdoeren/otop/internal/ui/panel"
	"github.com/rivo/tview"
)

func TestIsTextInput(t *testing.T) {
	cases := []struct {
		name string
		p    tview.Primitive
		want bool
	}{
		{"TextArea", tview.NewTextArea(), true},
		{"InputField", tview.NewInputField(), true},
		{"Table", tview.NewTable(), false},
		{"TextView", tview.NewTextView(), false},
		{"nil", nil, false},
	}
	for _, c := range cases {
		if got := isTextInput(c.p); got != c.want {
			t.Errorf("isTextInput(%s) = %v, want %v", c.name, got, c.want)
		}
	}
}

func TestDefaultPanelsAreRegistered(t *testing.T) {
	// NewApp seeds the layout with these; a typo would silently drop them.
	for _, name := range []string{"SessionList", "SQLDetail", "QueryEditor"} {
		if _, ok := panel.Global.Get(name); !ok {
			t.Errorf("panel %q is not registered", name)
		}
	}
}

// TestNewApp_KeyboardStartsOnSessionList guards against SetRoot stealing focus:
// arrow keys must reach the session table right after startup.
func TestNewApp_KeyboardStartsOnSessionList(t *testing.T) {
	a := NewApp(nil) // queries fail with ErrNotConnected; layout is real

	screen := tcell.NewSimulationScreen("UTF-8")
	if err := screen.Init(); err != nil {
		t.Fatal(err)
	}
	screen.SetSize(160, 40)
	a.tview.SetScreen(screen)

	done := make(chan struct{})
	go func() {
		defer close(done)
		if err := a.Run(); err != nil {
			t.Errorf("Run: %v", err)
		}
	}()
	t.Cleanup(func() {
		a.tview.Stop()
		select {
		case <-done:
		case <-time.After(2 * time.Second):
			t.Error("app did not shut down (main loop blocked?)")
		}
	})

	focused := func() tview.Primitive {
		ch := make(chan tview.Primitive, 1)
		a.tview.QueueUpdate(func() { ch <- a.tview.GetFocus() })
		select {
		case p := <-ch:
			return p
		case <-time.After(2 * time.Second):
			t.Fatal("timed out waiting for main goroutine")
			return nil
		}
	}

	table, ok := focused().(*tview.Table)
	if !ok {
		t.Fatalf("initial focus on %T, want *tview.Table (session list)", focused())
	}
	if table.GetTitle() != " Sessions " {
		t.Errorf("focused table title = %q, want \" Sessions \"", table.GetTitle())
	}

	// Tab moves to the detail panel and back.
	screen.InjectKey(tcell.KeyTab, 0, tcell.ModNone)
	deadline := time.Now().Add(2 * time.Second)
	for {
		if _, ok := focused().(*tview.TextView); ok {
			break
		}
		if time.Now().After(deadline) {
			t.Fatalf("after Tab focus on %T, want *tview.TextView (details)", focused())
		}
		time.Sleep(5 * time.Millisecond)
	}
}
