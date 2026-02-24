package panels

import (
	"fmt"
	"strings"

	"github.com/mdoeren/otop/internal/db"
	"github.com/mdoeren/otop/internal/models"
	uictx "github.com/mdoeren/otop/internal/ui/context"
	"github.com/mdoeren/otop/internal/ui/panel"
	"github.com/rivo/tview"
)

// SQLDetailPanel shows the execution plan and runtime statistics for a SQL ID.
// It is driven by SessionContext and SQLContext events from the bus.
type SQLDetailPanel struct {
	app  *tview.Application
	db   *db.DB
	text *tview.TextView
}

func newSQLDetailPanel(app *tview.Application, database *db.DB) panel.Panel {
	p := &SQLDetailPanel{
		app:  app,
		db:   database,
		text: tview.NewTextView().SetDynamicColors(true).SetScrollable(true),
	}
	p.text.SetTitle(" SQL Detail ").SetBorder(true)
	return p
}

func (p *SQLDetailPanel) Name() string               { return "SQLDetail" }
func (p *SQLDetailPanel) Primitive() tview.Primitive { return p.text }
func (p *SQLDetailPanel) Subscriptions() []string    { return []string{"SessionContext", "SQLContext"} }
func (p *SQLDetailPanel) Refresh()                   {}
func (p *SQLDetailPanel) Mount()                     {}
func (p *SQLDetailPanel) Unmount()                   {}

// OnContext is called on the tview main goroutine; it spawns a goroutine for DB I/O.
func (p *SQLDetailPanel) OnContext(ctx uictx.Context) {
	switch c := ctx.(type) {
	case uictx.SQLContext:
		go p.fetchAndRender(c.SQLID, c.SQLText)
	case uictx.SessionContext:
		if c.Session.SQLID != "" {
			go p.fetchAndRender(c.Session.SQLID, c.Session.SQLText)
		}
	}
}

func (p *SQLDetailPanel) fetchAndRender(sqlID, sqlText string) {
	plan, _ := p.db.GetExecutionPlan(sqlID)
	stats, _ := p.db.GetSQLStats(sqlID)
	p.app.QueueUpdateDraw(func() {
		p.render(sqlID, sqlText, plan, stats)
	})
}

func (p *SQLDetailPanel) render(sqlID, sqlText string, plan []models.PlanRow, stats *models.SQLStats) {
	var sb strings.Builder

	fmt.Fprintf(&sb, "[yellow]SQL ID:[-] %s\n\n", sqlID)
	if sqlText != "" {
		fmt.Fprintf(&sb, "[yellow]SQL Text:[-]\n%s\n\n", sqlText)
	}

	if stats != nil {
		fmt.Fprintf(&sb, "[yellow]Statistics:[-]\n")
		fmt.Fprintf(&sb, "  Executions:    %d\n", stats.Executions)
		fmt.Fprintf(&sb, "  Elapsed (µs):  %d\n", stats.ElapsedTimeMicros)
		fmt.Fprintf(&sb, "  CPU (µs):      %d\n", stats.CPUTimeMicros)
		fmt.Fprintf(&sb, "  Buffer Gets:   %d\n", stats.BufferGets)
		fmt.Fprintf(&sb, "  Disk Reads:    %d\n", stats.DiskReads)
		fmt.Fprintf(&sb, "  Rows:          %d\n\n", stats.Rows)
	}

	if len(plan) > 0 {
		fmt.Fprintf(&sb, "[yellow]Execution Plan:[-]\n")
		for _, row := range plan {
			indent := strings.Repeat("  ", row.Depth)
			op := row.Operation
			if row.Options != "" {
				op += " " + row.Options
			}
			if row.ObjectName != "" {
				op += " [" + row.ObjectName + "]"
			}
			fmt.Fprintf(&sb, "  %s%s\n", indent, op)
		}
	}

	p.text.SetText(sb.String())
}

func init() {
	panel.Global.Register(panel.Entry{
		TypeName:    "SQLDetail",
		Description: "Execution plan and runtime statistics for a SQL statement",
		Factory:     newSQLDetailPanel,
	})
}
