package statusbar

import (
	"fmt"
	"sync"
	"time"

	"github.com/rivo/tview"
)

// StatusBar is a 1-row status strip that shows transient info and error messages.
// It is safe to call from any goroutine, including the tview main goroutine.
type StatusBar struct {
	app     *tview.Application
	view    *tview.TextView
	mu      sync.Mutex
	timer   *time.Timer
	version int
}

// New creates a StatusBar backed by app for thread-safe UI updates.
func New(app *tview.Application) *StatusBar {
	v := tview.NewTextView().SetDynamicColors(true)
	return &StatusBar{app: app, view: v}
}

// Primitive returns the tview widget to add to a layout.
func (s *StatusBar) Primitive() tview.Primitive {
	return s.view
}

// Error shows msg in red and auto-clears after 10 s.
func (s *StatusBar) Error(msg string) {
	s.show(fmt.Sprintf("[red]%s[-]", msg), 10*time.Second)
}

// Info shows a formatted message in gray and auto-clears after 5 s.
func (s *StatusBar) Info(format string, args ...any) {
	s.show(fmt.Sprintf("[gray]"+format+"[-]", args...), 5*time.Second)
}

func (s *StatusBar) show(text string, ttl time.Duration) {
	s.mu.Lock()
	s.version++
	ver := s.version
	if s.timer != nil {
		s.timer.Stop()
	}
	s.timer = time.AfterFunc(ttl, func() { s.setIfCurrent(ver, "") })
	s.mu.Unlock()

	// QueueUpdateDraw blocks until the main loop has run the update, so
	// calling it directly from the main goroutine (e.g. from inside another
	// queued update or an input handler) would deadlock the whole UI.
	// Hand it off to a goroutine; setIfCurrent keeps the newest message
	// winning even if these goroutines run out of order.
	go s.setIfCurrent(ver, text)
}

// setIfCurrent sets the text only if no newer message was shown since ver.
// Must not be called on the main goroutine.
func (s *StatusBar) setIfCurrent(ver int, text string) {
	s.app.QueueUpdateDraw(func() {
		s.mu.Lock()
		current := ver == s.version
		s.mu.Unlock()
		if current {
			s.view.SetText(text)
		}
	})
}
