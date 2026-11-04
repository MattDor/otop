package panels

import (
	"errors"
	"strings"
	"sync"
	"testing"
	"time"
	"unicode/utf8"

	"github.com/gdamore/tcell/v2"
	"github.com/mdoeren/otop/internal/models"
	uictx "github.com/mdoeren/otop/internal/ui/context"
)

type fakeSessions struct {
	mu       sync.Mutex
	sessions []models.Session
	err      error
}

func (f *fakeSessions) GetActiveSessions() ([]models.Session, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	return append([]models.Session(nil), f.sessions...), f.err
}

func (f *fakeSessions) set(s []models.Session, err error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.sessions, f.err = s, err
}

// recorder collects emitted contexts. It is only touched on the main goroutine.
type recorder struct{ events []uictx.Context }

func (r *recorder) emit(c uictx.Context) { r.events = append(r.events, c) }

// sessionSIDs returns the SIDs of all emitted SessionContexts, in order.
func (r *recorder) sessionSIDs() []int {
	var out []int
	for _, e := range r.events {
		if sc, ok := e.(uictx.SessionContext); ok {
			out = append(out, sc.Session.SID)
		}
	}
	return out
}

func withDebounce(t *testing.T, d time.Duration) {
	t.Helper()
	old := selectDebounce
	selectDebounce = d
	t.Cleanup(func() { selectDebounce = old })
}

var threeSessions = []models.Session{
	{SID: 1, Serial: 10, Username: "SCOTT", Status: "ACTIVE", SQLID: "sql1", SQLText: "select 1 from dual"},
	{SID: 2, Serial: 20, Username: "HR", Status: "INACTIVE"},
	{SID: 3, Serial: 30, Username: "APP", Status: "ACTIVE", SQLID: "sql3", SQLText: "update t set x = 1"},
}

// setupSessionList creates a mounted session list on a running app and
// waits until the initial load has been rendered.
func setupSessionList(t *testing.T, sessions []models.Session) (*SessionListPanel, *fakeSessions, *recorder, *tviewApp) {
	t.Helper()
	src := &fakeSessions{sessions: sessions}
	rec := &recorder{}
	p := newSessionList(nil, src)
	app, screen := startApp(t, p.table)
	p.app = app
	p.SetEmitFn(rec.emit)
	onMain(t, app, p.Mount)
	eventually(t, app, func() bool { return len(p.sessions) == len(sessions) }, "initial load")
	return p, src, rec, &tviewApp{app, screen}
}

func TestSessionList_InitialLoadSelectsFirstSessionAndEmitsDetails(t *testing.T) {
	withDebounce(t, 10*time.Millisecond)
	p, _, rec, a := setupSessionList(t, threeSessions)

	eventually(t, a.app, func() bool { return len(rec.events) == 2 }, "two events emitted")
	onMain(t, a.app, func() {
		row, _ := p.table.GetSelection()
		if row != 1 {
			t.Errorf("selected row = %d, want 1", row)
		}
		sc, ok := rec.events[0].(uictx.SessionContext)
		if !ok || sc.Session.SID != 1 {
			t.Errorf("first event = %#v, want SessionContext for SID 1", rec.events[0])
		}
		qc, ok := rec.events[1].(uictx.SQLContext)
		if !ok || qc.SQLID != "sql1" || qc.SQLText != "select 1 from dual" {
			t.Errorf("second event = %#v, want SQLContext for sql1", rec.events[1])
		}
	})
}

func TestSessionList_SessionWithoutSQLEmitsOnlySessionContext(t *testing.T) {
	withDebounce(t, 10*time.Millisecond)
	p, _, rec, a := setupSessionList(t, threeSessions)
	eventually(t, a.app, func() bool { return len(rec.events) == 2 }, "initial emit")

	onMain(t, a.app, func() {
		rec.events = nil
		p.table.Select(2, 0) // SID 2 has no SQL_ID
	})
	eventually(t, a.app, func() bool { return len(rec.events) > 0 }, "emit for SID 2")
	time.Sleep(30 * time.Millisecond)
	onMain(t, a.app, func() {
		if len(rec.events) != 1 {
			t.Fatalf("got %d events, want 1: %#v", len(rec.events), rec.events)
		}
		if sc, ok := rec.events[0].(uictx.SessionContext); !ok || sc.Session.SID != 2 {
			t.Errorf("event = %#v, want SessionContext for SID 2", rec.events[0])
		}
	})
}

func TestSessionList_CursorMovesAreDebounced(t *testing.T) {
	withDebounce(t, 100*time.Millisecond)
	p, _, rec, a := setupSessionList(t, threeSessions)
	eventually(t, a.app, func() bool { return len(rec.events) == 2 }, "initial emit")

	onMain(t, a.app, func() {
		rec.events = nil
		// Scroll quickly past SID 2 to SID 3.
		p.table.Select(2, 0)
		p.table.Select(3, 0)
		if len(rec.events) != 0 {
			t.Errorf("emitted before debounce elapsed: %#v", rec.events)
		}
	})

	eventually(t, a.app, func() bool { return len(rec.sessionSIDs()) > 0 }, "debounced emit")
	time.Sleep(150 * time.Millisecond) // make sure no further (stale) timer fires
	onMain(t, a.app, func() {
		if got := rec.sessionSIDs(); len(got) != 1 || got[0] != 3 {
			t.Errorf("emitted sessions = %v, want [3] (SID 2 must be skipped)", got)
		}
	})
}

func TestSessionList_EnterEmitsImmediatelyAndRepeatedly(t *testing.T) {
	withDebounce(t, time.Hour) // only Enter can trigger an emit
	_, _, rec, a := setupSessionList(t, threeSessions)

	a.screen.InjectKey(tcell.KeyEnter, 0, tcell.ModNone)
	eventually(t, a.app, func() bool { return len(rec.sessionSIDs()) == 1 }, "emit on Enter")

	// Enter on the already-shown session reloads it.
	a.screen.InjectKey(tcell.KeyEnter, 0, tcell.ModNone)
	eventually(t, a.app, func() bool { return len(rec.sessionSIDs()) == 2 }, "second emit on Enter")

	onMain(t, a.app, func() {
		if got := rec.sessionSIDs(); got[0] != 1 || got[1] != 1 {
			t.Errorf("emitted sessions = %v, want [1 1]", got)
		}
	})
}

func TestSessionList_EnterCancelsPendingDebounce(t *testing.T) {
	withDebounce(t, 50*time.Millisecond)
	p, _, rec, a := setupSessionList(t, threeSessions)
	eventually(t, a.app, func() bool { return len(rec.events) == 2 }, "initial emit")

	onMain(t, a.app, func() {
		rec.events = nil
		p.table.Select(3, 0)
	})
	a.screen.InjectKey(tcell.KeyEnter, 0, tcell.ModNone)
	eventually(t, a.app, func() bool { return len(rec.sessionSIDs()) == 1 }, "emit on Enter")
	time.Sleep(100 * time.Millisecond)
	onMain(t, a.app, func() {
		if got := rec.sessionSIDs(); len(got) != 1 || got[0] != 3 {
			t.Errorf("emitted sessions = %v, want [3]", got)
		}
	})
}

func TestSessionList_RefreshWithSameSessionDoesNotReemit(t *testing.T) {
	withDebounce(t, 10*time.Millisecond)
	p, _, rec, a := setupSessionList(t, threeSessions)
	eventually(t, a.app, func() bool { return len(rec.events) == 2 }, "initial emit")

	p.loadSessions()                  // queues the re-render
	onMain(t, a.app, func() {})       // barrier: re-render has run
	time.Sleep(50 * time.Millisecond) // any (unexpected) debounce would fire
	onMain(t, a.app, func() {
		if len(rec.events) != 2 {
			t.Errorf("got %d events after refresh, want 2 (no re-emit)", len(rec.events))
		}
	})
}

func TestSessionList_RefreshReorderUpdatesDetails(t *testing.T) {
	withDebounce(t, 10*time.Millisecond)
	p, src, rec, a := setupSessionList(t, threeSessions)
	eventually(t, a.app, func() bool { return len(rec.events) == 2 }, "initial emit")

	// SID 1 drops out; SID 3 moves under the cursor (row 1).
	src.set([]models.Session{threeSessions[2], threeSessions[1]}, nil)
	p.loadSessions()
	onMain(t, a.app, func() {
		got := rec.sessionSIDs()
		if len(got) != 2 || got[1] != 3 {
			t.Errorf("emitted sessions = %v, want [1 3]", got)
		}
	})
}

func TestSessionList_RefreshShrinkingListClampsCursor(t *testing.T) {
	withDebounce(t, 10*time.Millisecond)
	p, src, rec, a := setupSessionList(t, threeSessions)
	eventually(t, a.app, func() bool { return len(rec.events) == 2 }, "initial emit")
	onMain(t, a.app, func() { p.table.Select(3, 0) })
	eventually(t, a.app, func() bool { return len(rec.sessionSIDs()) == 2 }, "emit for SID 3")

	src.set(threeSessions[:1], nil)
	p.loadSessions()
	onMain(t, a.app, func() {
		if row, _ := p.table.GetSelection(); row != 1 {
			t.Errorf("selected row = %d, want 1", row)
		}
	})
	eventually(t, a.app, func() bool {
		got := rec.sessionSIDs()
		return got[len(got)-1] == 1
	}, "details switch to remaining session")
}

func TestSessionList_LoadErrorIsReportedAndKeepsData(t *testing.T) {
	withDebounce(t, time.Hour)
	p, src, _, a := setupSessionList(t, threeSessions)

	var mu sync.Mutex
	var reported error
	p.SetStatusFn(func(err error) {
		mu.Lock()
		reported = err
		mu.Unlock()
	})
	want := errors.New("ORA-03113: end-of-file on communication channel")
	src.set(nil, want)
	p.loadSessions()

	mu.Lock()
	got := reported
	mu.Unlock()
	if !errors.Is(got, want) {
		t.Errorf("reported error = %v, want %v", got, want)
	}
	onMain(t, a.app, func() {
		if len(p.sessions) != 3 {
			t.Errorf("sessions = %d, want previous 3 to be kept", len(p.sessions))
		}
	})
}

func TestSessionList_UnmountCancelsPendingDebounce(t *testing.T) {
	withDebounce(t, 50*time.Millisecond)
	p, _, rec, a := setupSessionList(t, threeSessions)
	eventually(t, a.app, func() bool { return len(rec.events) == 2 }, "initial emit")

	onMain(t, a.app, func() {
		rec.events = nil
		p.table.Select(3, 0)
		p.Unmount()
	})
	time.Sleep(120 * time.Millisecond)
	onMain(t, a.app, func() {
		if len(rec.events) != 0 {
			t.Errorf("emitted after Unmount: %#v", rec.events)
		}
	})
}

func TestSessionList_RenderTruncatesLongSQLOnRuneBoundary(t *testing.T) {
	withDebounce(t, time.Hour)
	long := strings.Repeat("ä", 60) // 2 bytes per rune
	p, _, _, a := setupSessionList(t, []models.Session{{SID: 1, Serial: 1, SQLText: long}})

	onMain(t, a.app, func() {
		text := p.table.GetCell(1, 5).Text
		if !utf8.ValidString(text) {
			t.Fatalf("truncated SQL text is not valid UTF-8: %q", text)
		}
		if want := strings.Repeat("ä", 50) + "…"; text != want {
			t.Errorf("SQL cell = %q, want %q", text, want)
		}
	})
}
