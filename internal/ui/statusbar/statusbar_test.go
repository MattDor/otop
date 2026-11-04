package statusbar

import (
	"strings"
	"testing"
	"time"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
)

func runApp(t *testing.T, root tview.Primitive) *tview.Application {
	t.Helper()
	screen := tcell.NewSimulationScreen("UTF-8")
	if err := screen.Init(); err != nil {
		t.Fatal(err)
	}
	app := tview.NewApplication().SetScreen(screen).SetRoot(root, true)
	done := make(chan struct{})
	go func() { defer close(done); _ = app.Run() }()
	t.Cleanup(func() {
		app.Stop()
		select {
		case <-done:
		case <-time.After(2 * time.Second):
			t.Error("app did not shut down (main loop blocked?)")
		}
	})
	return app
}

// onMain runs fn on the main goroutine; fails if the main loop is stuck.
func onMain(t *testing.T, app *tview.Application, fn func()) {
	t.Helper()
	done := make(chan struct{})
	go app.QueueUpdate(func() { fn(); close(done) })
	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("main loop is blocked (deadlock?)")
	}
}

func waitText(t *testing.T, app *tview.Application, s *StatusBar, want string) {
	t.Helper()
	deadline := time.Now().Add(2 * time.Second)
	var got string
	for time.Now().Before(deadline) {
		onMain(t, app, func() { got = s.view.GetText(true) })
		if strings.Contains(got, want) {
			return
		}
		time.Sleep(5 * time.Millisecond)
	}
	t.Fatalf("status text = %q, want it to contain %q", got, want)
}

func TestStatusBar_CallFromMainGoroutineDoesNotDeadlock(t *testing.T) {
	sb := New(nil)
	app := runApp(t, sb.Primitive())
	sb.app = app

	// This is what the refresh ticker does inside QueueUpdateDraw.
	onMain(t, app, func() { sb.Info("Refreshed %s", "12:00:00") })
	waitText(t, app, sb, "Refreshed 12:00:00")

	// The main loop must still be responsive afterwards.
	onMain(t, app, func() {})
}

func TestStatusBar_NewestMessageWins(t *testing.T) {
	sb := New(nil)
	app := runApp(t, sb.Primitive())
	sb.app = app

	for i := 0; i < 20; i++ {
		onMain(t, app, func() {
			sb.Info("info")
			sb.Error("ORA-12345 newest")
		})
		waitText(t, app, sb, "ORA-12345 newest")
		time.Sleep(10 * time.Millisecond) // let any stale goroutine land
		var got string
		onMain(t, app, func() { got = sb.view.GetText(true) })
		if got != "ORA-12345 newest" {
			t.Fatalf("iteration %d: status = %q, older message overwrote newer", i, got)
		}
	}
}

func TestStatusBar_ClearsAfterTTLButNotNewerMessage(t *testing.T) {
	sb := New(nil)
	app := runApp(t, sb.Primitive())
	sb.app = app

	sb.show("first", 30*time.Millisecond)
	sb.show("second", time.Hour)
	time.Sleep(80 * time.Millisecond) // first's TTL has passed
	waitText(t, app, sb, "second")

	sb.show("short", 20*time.Millisecond)
	deadline := time.Now().Add(2 * time.Second)
	for {
		var got string
		onMain(t, app, func() { got = sb.view.GetText(true) })
		if got == "" {
			return
		}
		if time.Now().After(deadline) {
			t.Fatalf("status not cleared after TTL, still %q", got)
		}
		time.Sleep(5 * time.Millisecond)
	}
}
