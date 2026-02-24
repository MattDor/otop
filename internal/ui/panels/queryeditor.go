package panels

import (
	"github.com/mdoeren/otop/internal/db"
	uictx "github.com/mdoeren/otop/internal/ui/context"
	"github.com/mdoeren/otop/internal/ui/panel"
	"github.com/rivo/tview"
)

// QueryEditorPanel is a stub panel that pre-populates with SQL text from SQLContext.
// Future: execute queries, show results.
type QueryEditorPanel struct {
	app    *tview.Application
	db     *db.DB
	editor *tview.TextArea
}

func newQueryEditorPanel(app *tview.Application, database *db.DB) panel.Panel {
	p := &QueryEditorPanel{
		app:    app,
		db:     database,
		editor: tview.NewTextArea(),
	}
	p.editor.SetTitle(" Query Editor ").SetBorder(true)
	return p
}

func (p *QueryEditorPanel) Name() string               { return "QueryEditor" }
func (p *QueryEditorPanel) Primitive() tview.Primitive { return p.editor }
func (p *QueryEditorPanel) Subscriptions() []string    { return []string{"SQLContext"} }
func (p *QueryEditorPanel) Refresh()                   {}
func (p *QueryEditorPanel) Mount()                     {}
func (p *QueryEditorPanel) Unmount()                   {}

func (p *QueryEditorPanel) OnContext(ctx uictx.Context) {
	if c, ok := ctx.(uictx.SQLContext); ok {
		p.editor.SetText(c.SQLText, true)
	}
}

func init() {
	panel.Global.Register(panel.Entry{
		TypeName:    "QueryEditor",
		Description: "SQL query editor (pre-populated from selected statement)",
		Factory:     newQueryEditorPanel,
	})
}
