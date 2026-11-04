package panels

import (
	"fmt"
	"strings"
	"sync/atomic"

	"github.com/mdoeren/otop/internal/db"
	"github.com/mdoeren/otop/internal/models"
	uictx "github.com/mdoeren/otop/internal/ui/context"
	"github.com/mdoeren/otop/internal/ui/panel"
	"github.com/rivo/tview"
)

// sqlSource is the subset of *db.DB the detail panel needs.
type sqlSource interface {
	GetExecutionPlan(sqlID string) ([]models.PlanRow, error)
	GetSQLStats(sqlID string) (*models.SQLStats, error)
}

// SQLDetailPanel shows session details plus the execution plan and runtime
// statistics of the session's current SQL statement.
// It is driven by SessionContext and SQLContext events from the bus.
type SQLDetailPanel struct {
	app      *tview.Application
	db       sqlSource
	text     *tview.TextView
	statusFn func(error)

	// session is the currently displayed session (nil if detail was opened
	// from a bare SQLContext). Only touched on the tview main goroutine.
	session *models.Session
	// gen is bumped for every new request so stale fetches are discarded.
	gen atomic.Uint64
}

func newSQLDetailPanel(app *tview.Application, database *db.DB) panel.Panel {
	return newSQLDetail(app, database)
}

func newSQLDetail(app *tview.Application, src sqlSource) *SQLDetailPanel {
	p := &SQLDetailPanel{
		app:  app,
		db:   src,
		text: tview.NewTextView().SetDynamicColors(true).SetScrollable(true),
	}
	p.text.SetTitle(" Details ").SetBorder(true)
	p.text.SetText("[gray]Move the cursor to a session to show its details.[-]")
	return p
}

func (p *SQLDetailPanel) Name() string               { return "SQLDetail" }
func (p *SQLDetailPanel) Primitive() tview.Primitive { return p.text }
func (p *SQLDetailPanel) Subscriptions() []string    { return []string{"SessionContext", "SQLContext"} }
func (p *SQLDetailPanel) Refresh()                   {}
func (p *SQLDetailPanel) Mount()                     {}
func (p *SQLDetailPanel) Unmount()                   { p.gen.Add(1) }
func (p *SQLDetailPanel) SetStatusFn(fn func(error)) { p.statusFn = fn }

// OnContext is called on the tview main goroutine; it spawns a goroutine for DB I/O.
func (p *SQLDetailPanel) OnContext(ctx uictx.Context) {
	switch c := ctx.(type) {
	case uictx.SessionContext:
		s := c.Session
		p.session = &s
		p.load(s.SQLID, s.SQLText)
	case uictx.SQLContext:
		// The session list emits SQLContext right after SessionContext for the
		// same statement; skip it to avoid a duplicate fetch.
		if p.session != nil && p.session.SQLID == c.SQLID {
			return
		}
		p.session = nil
		p.load(c.SQLID, c.SQLText)
	}
}

func (p *SQLDetailPanel) load(sqlID, sqlText string) {
	gen := p.gen.Add(1)
	session := p.session
	if sqlID == "" {
		p.render(session, sqlID, sqlText, nil, nil)
		return
	}
	p.text.SetText("[gray]Loading…[-]")
	go p.fetchAndRender(gen, session, sqlID, sqlText)
}

func (p *SQLDetailPanel) fetchAndRender(gen uint64, session *models.Session, sqlID, sqlText string) {
	plan, err := p.db.GetExecutionPlan(sqlID)
	if err != nil && p.statusFn != nil {
		p.statusFn(err)
	}

	stats, err := p.db.GetSQLStats(sqlID)
	if err != nil && p.statusFn != nil {
		p.statusFn(err)
	}

	p.app.QueueUpdateDraw(func() {
		if p.gen.Load() != gen {
			return // a newer selection superseded this one
		}
		p.render(session, sqlID, sqlText, plan, stats)
	})
}

func (p *SQLDetailPanel) render(session *models.Session, sqlID, sqlText string, plan []models.PlanRow, stats *models.SQLStats) {
	var sb strings.Builder

	if s := session; s != nil {
		fmt.Fprintf(&sb, "[yellow]Session:[-]\n")
		fmt.Fprintf(&sb, "  SID,Serial#:   %d,%d\n", s.SID, s.Serial)
		fmt.Fprintf(&sb, "  Username:      %s\n", s.Username)
		fmt.Fprintf(&sb, "  Status:        %s\n", s.Status)
		fmt.Fprintf(&sb, "  Program:       %s\n", tview.Escape(s.Program))
		fmt.Fprintf(&sb, "  Machine:       %s\n", tview.Escape(s.Machine))
		if s.WaitEvent != "" {
			fmt.Fprintf(&sb, "  Wait Event:    %s (%.0f s)\n", tview.Escape(s.WaitEvent), s.WaitSeconds)
		}
		sb.WriteString("\n")
	}

	if sqlID == "" {
		sb.WriteString("[gray]No SQL statement currently associated with this session.[-]\n")
		p.text.SetText(sb.String()).ScrollToBeginning()
		return
	}

	fmt.Fprintf(&sb, "[yellow]SQL ID:[-] %s\n\n", sqlID)
	if sqlText == "" && stats != nil {
		sqlText = stats.SQLText
	}
	if sqlText != "" {
		fmt.Fprintf(&sb, "[yellow]SQL Text:[-]\n%s\n\n", tview.Escape(sqlText))
	}

	if stats != nil {
		fmt.Fprintf(&sb, "[yellow]Statistics (all child cursors):[-]\n")
		fmt.Fprintf(&sb, "  Executions:    %d\n", stats.Executions)
		fmt.Fprintf(&sb, "  Elapsed:       %.3f s\n", float64(stats.ElapsedTimeMicros)/1e6)
		fmt.Fprintf(&sb, "  CPU:           %.3f s\n", float64(stats.CPUTimeMicros)/1e6)
		if stats.Executions > 0 {
			fmt.Fprintf(&sb, "  Elapsed/exec:  %.3f ms\n", float64(stats.ElapsedTimeMicros)/1e3/float64(stats.Executions))
		}
		fmt.Fprintf(&sb, "  Buffer Gets:   %d\n", stats.BufferGets)
		fmt.Fprintf(&sb, "  Disk Reads:    %d\n", stats.DiskReads)
		fmt.Fprintf(&sb, "  Rows:          %d\n\n", stats.Rows)
	}

	if len(plan) > 0 {
		fmt.Fprintf(&sb, "[yellow]Execution Plan:[-]\n")
		fmt.Fprintf(&sb, "  [::b]%-4s %-60s %12s %12s %10s[::-]\n", "Id", "Operation", "Rows", "Bytes", "Cost")
		for _, row := range plan {
			op := strings.Repeat("  ", row.Depth) + row.Operation
			if row.Options != "" {
				op += " " + row.Options
			}
			if row.ObjectName != "" {
				op += " " + row.ObjectName
			}
			fmt.Fprintf(&sb, "  %-4d %-60s %12d %12d %10d\n",
				row.ID, tview.Escape(op), row.Cardinality, row.Bytes, row.Cost)
		}
	} else {
		sb.WriteString("[gray]No execution plan found in V$SQL_PLAN.[-]\n")
	}

	p.text.SetText(sb.String()).ScrollToBeginning()
}

func init() {
	panel.Global.Register(panel.Entry{
		TypeName:    "SQLDetail",
		Description: "Session details, execution plan and runtime statistics",
		Factory:     newSQLDetailPanel,
	})
}
