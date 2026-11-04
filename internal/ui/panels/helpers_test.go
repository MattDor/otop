package panels

import (
	"testing"
	"time"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
)

// startApp runs a tview application on a simulated screen so that
// QueueUpdateDraw callbacks are actually executed. root receives focus.
func startApp(t *testing.T, root tview.Primitive) (*tview.Application, tcell.SimulationScreen) {
	t.Helper()
	screen := tcell.NewSimulationScreen("UTF-8")
	if err := screen.Init(); err != nil {
		t.Fatalf("init screen: %v", err)
	}
	screen.SetSize(160, 40)

	app := tview.NewApplication().SetScreen(screen).SetRoot(root, true)
	done := make(chan struct{})
	go func() {
		defer close(done)
		if err := app.Run(); err != nil {
			t.Errorf("app.Run: %v", err)
		}
	}()
	t.Cleanup(func() {
		app.Stop()
		select {
		case <-done:
		case <-time.After(2 * time.Second):
			t.Error("app did not shut down (main loop blocked?)")
		}
	})
	return app, screen
}

// onMain runs fn on the tview main goroutine and waits for it. Because the
// update queue is FIFO, it also acts as a barrier for previously queued updates.
func onMain(t *testing.T, app *tview.Application, fn func()) {
	t.Helper()
	done := make(chan struct{})
	app.QueueUpdate(func() {
		fn()
		close(done)
	})
	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for main goroutine")
	}
}

// eventually polls cond (on the main goroutine) until it is true or times out.
func eventually(t *testing.T, app *tview.Application, cond func() bool, msg string) {
	t.Helper()
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		var ok bool
		onMain(t, app, func() { ok = cond() })
		if ok {
			return
		}
		time.Sleep(5 * time.Millisecond)
	}
	t.Fatalf("condition not met: %s", msg)
}

// tviewApp bundles a running app with its simulated screen (for key injection).
type tviewApp struct {
	app    *tview.Application
	screen tcell.SimulationScreen
}
