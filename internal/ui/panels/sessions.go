package panels

import (
	"fmt"
	"time"

	"github.com/gdamore/tcell/v2"
	"github.com/mdoeren/otop/internal/db"
	"github.com/mdoeren/otop/internal/models"
	uictx "github.com/mdoeren/otop/internal/ui/context"
	"github.com/mdoeren/otop/internal/ui/panel"
	"github.com/rivo/tview"
)

// sessionSource is the subset of *db.DB the session list needs.
type sessionSource interface {
	GetActiveSessions() ([]models.Session, error)
}

// SessionListPanel displays active Oracle sessions in a selectable table.
// Selecting a row emits SessionContext and SQLContext to the workflow bus.
type SessionListPanel struct {
	app      *tview.Application
	db       sessionSource
	table    *tview.Table
	emitFn   func(uictx.Context)
	statusFn func(error)
	sessions []models.Session
	debounce *time.Timer // pending cursor-move emit; main goroutine only
	shown    sessionKey  // session whose details were last emitted
}

func newSessionListPanel(app *tview.Application, database *db.DB) panel.Panel {
	return newSessionList(app, database)
}

func newSessionList(app *tview.Application, src sessionSource) *SessionListPanel {
	p := &SessionListPanel{
		app:   app,
		db:    src,
		table: tview.NewTable().SetBorders(false).SetSelectable(true, false),
	}
	p.table.SetTitle(" Sessions ").SetBorder(true)
	// Enter shows details immediately.
	p.table.SetSelectedFunc(func(row, _ int) {
		p.cancelDebounce()
		p.emitRow(row, true)
	})
	// Moving the cursor shows details after a short pause, so scrolling
	// through the list doesn't fire a DB query for every row passed.
	p.table.SetSelectionChangedFunc(func(row, _ int) {
		p.cancelDebounce()
		p.debounce = time.AfterFunc(selectDebounce, func() {
			p.app.QueueUpdateDraw(func() { p.emitRow(row, false) })
		})
	})
	return p
}

// selectDebounce is how long the cursor must rest on a row before its
// details are loaded. A variable so tests can shorten it.
var selectDebounce = 300 * time.Millisecond

func (p *SessionListPanel) cancelDebounce() {
	if p.debounce != nil {
		p.debounce.Stop()
		p.debounce = nil
	}
}

// emitRow publishes the session at the given table row. Must run on the tview
// main goroutine. Unless force is set, nothing is emitted if that session is
// already the one being shown (e.g. after a periodic refresh).
func (p *SessionListPanel) emitRow(row int, force bool) {
	// row 0 is the header
	idx := row - 1
	if idx < 0 || idx >= len(p.sessions) || p.emitFn == nil {
		return
	}
	s := p.sessions[idx]
	key := sessionKey{s.SID, s.Serial}
	if !force && p.shown == key {
		return
	}
	p.shown = key
	p.emitFn(uictx.SessionContext{Session: s})
	if s.SQLID != "" {
		p.emitFn(uictx.SQLContext{SQLID: s.SQLID, SQLText: s.SQLText})
	}
}

// sessionKey uniquely identifies a session (SIDs are reused across sessions).
type sessionKey struct{ sid, serial int }

func (p *SessionListPanel) Name() string                     { return "SessionList" }
func (p *SessionListPanel) Primitive() tview.Primitive       { return p.table }
func (p *SessionListPanel) Subscriptions() []string          { return nil }
func (p *SessionListPanel) OnContext(_ uictx.Context)        {}
func (p *SessionListPanel) SetEmitFn(fn func(uictx.Context)) { p.emitFn = fn }
func (p *SessionListPanel) SetStatusFn(fn func(error))       { p.statusFn = fn }

func (p *SessionListPanel) Mount() {
	p.table.SetCell(0, 0, tview.NewTableCell("[gray]Loading…[-]").SetSelectable(false))
	go p.loadSessions()
}

func (p *SessionListPanel) Unmount() { p.cancelDebounce() }

func (p *SessionListPanel) Refresh() {
	go p.loadSessions()
}

func (p *SessionListPanel) loadSessions() {
	sessions, err := p.db.GetActiveSessions()
	if err != nil {
		if p.statusFn != nil {
			p.statusFn(err)
		}
		return
	}
	p.app.QueueUpdateDraw(func() {
		p.sessions = sessions
		p.renderTable()
		p.syncSelection()
	})
}

// syncSelection runs after a reload: it puts the cursor on the first session
// if none is selected yet, and otherwise updates the details if a different
// session ended up under the cursor (the list order can change on refresh).
func (p *SessionListPanel) syncSelection() {
	if len(p.sessions) == 0 {
		return
	}
	row, _ := p.table.GetSelection()
	if row < 1 {
		p.table.Select(1, 0) // fires SelectionChangedFunc → debounced emit
		return
	}
	if row > len(p.sessions) {
		p.table.Select(len(p.sessions), 0)
		return
	}
	p.emitRow(row, false)
}

func (p *SessionListPanel) renderTable() {
	p.table.Clear()

	headers := []string{"SID", "Username", "Status", "SQL ID", "Wait Event", "SQL Text"}
	for col, h := range headers {
		p.table.SetCell(0, col,
			tview.NewTableCell(h).
				SetTextColor(tcell.ColorYellow).
				SetSelectable(false).
				SetExpansion(1))
	}

	for i, s := range p.sessions {
		row := i + 1
		sqlText := truncate(s.SQLText, 50)
		color := tcell.ColorDefault
		if s.Status == "ACTIVE" {
			color = tcell.ColorGreen
		}
		p.table.SetCell(row, 0, tview.NewTableCell(fmt.Sprintf("%d", s.SID)).SetTextColor(color))
		p.table.SetCell(row, 1, tview.NewTableCell(s.Username).SetTextColor(color))
		p.table.SetCell(row, 2, tview.NewTableCell(s.Status).SetTextColor(color))
		p.table.SetCell(row, 3, tview.NewTableCell(s.SQLID).SetTextColor(color))
		p.table.SetCell(row, 4, tview.NewTableCell(tview.Escape(s.WaitEvent)).SetTextColor(color).SetExpansion(1))
		p.table.SetCell(row, 5, tview.NewTableCell(tview.Escape(sqlText)).SetTextColor(color).SetExpansion(2))
	}
}

func init() {
	panel.Global.Register(panel.Entry{
		TypeName:    "SessionList",
		Description: "Active Oracle sessions with SQL and wait info",
		Factory:     newSessionListPanel,
	})
}

// truncate shortens s to at most n runes, appending "…" if it was cut.
// It never splits a multi-byte character.
func truncate(s string, n int) string {
	r := []rune(s)
	if len(r) <= n {
		return s
	}
	return string(r[:n]) + "…"
}
