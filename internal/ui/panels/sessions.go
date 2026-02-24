package panels

import (
	"fmt"

	"github.com/gdamore/tcell/v2"
	"github.com/mdoeren/otop/internal/db"
	"github.com/mdoeren/otop/internal/models"
	uictx "github.com/mdoeren/otop/internal/ui/context"
	"github.com/mdoeren/otop/internal/ui/panel"
	"github.com/rivo/tview"
)

// SessionListPanel displays active Oracle sessions in a selectable table.
// Selecting a row emits SessionContext and SQLContext to the workflow bus.
type SessionListPanel struct {
	app      *tview.Application
	db       *db.DB
	table    *tview.Table
	emitFn   func(uictx.Context)
	statusFn func(error)
	sessions []models.Session
}

func newSessionListPanel(app *tview.Application, database *db.DB) panel.Panel {
	p := &SessionListPanel{
		app:   app,
		db:    database,
		table: tview.NewTable().SetBorders(false).SetSelectable(true, false),
	}
	p.table.SetTitle(" Sessions ").SetBorder(true)
	p.table.SetSelectedFunc(func(row, _ int) {
		// row 0 is the header
		idx := row - 1
		if idx < 0 || idx >= len(p.sessions) {
			return
		}
		s := p.sessions[idx]
		if p.emitFn == nil {
			return
		}
		p.emitFn(uictx.SessionContext{Session: s})
		if s.SQLID != "" {
			p.emitFn(uictx.SQLContext{SQLID: s.SQLID, SQLText: s.SQLText})
		}
	})
	return p
}

func (p *SessionListPanel) Name() string                      { return "SessionList" }
func (p *SessionListPanel) Primitive() tview.Primitive        { return p.table }
func (p *SessionListPanel) Subscriptions() []string           { return nil }
func (p *SessionListPanel) OnContext(_ uictx.Context)         {}
func (p *SessionListPanel) SetEmitFn(fn func(uictx.Context))  { p.emitFn = fn }
func (p *SessionListPanel) SetStatusFn(fn func(error))        { p.statusFn = fn }

func (p *SessionListPanel) Mount() {
	p.table.SetCell(0, 0, tview.NewTableCell("[gray]Loading…[-]").SetSelectable(false))
	go p.loadSessions()
}

func (p *SessionListPanel) Unmount() {}

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
	})
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
		sqlText := s.SQLText
		if len(sqlText) > 50 {
			sqlText = sqlText[:50] + "…"
		}
		color := tcell.ColorDefault
		if s.Status == "ACTIVE" {
			color = tcell.ColorGreen
		}
		p.table.SetCell(row, 0, tview.NewTableCell(fmt.Sprintf("%d", s.SID)).SetTextColor(color))
		p.table.SetCell(row, 1, tview.NewTableCell(s.Username).SetTextColor(color))
		p.table.SetCell(row, 2, tview.NewTableCell(s.Status).SetTextColor(color))
		p.table.SetCell(row, 3, tview.NewTableCell(s.SQLID).SetTextColor(color))
		p.table.SetCell(row, 4, tview.NewTableCell(s.WaitEvent).SetTextColor(color).SetExpansion(1))
		p.table.SetCell(row, 5, tview.NewTableCell(sqlText).SetTextColor(color).SetExpansion(2))
	}
}

func init() {
	panel.Global.Register(panel.Entry{
		TypeName:    "SessionList",
		Description: "Active Oracle sessions with SQL and wait info",
		Factory:     newSessionListPanel,
	})
}
