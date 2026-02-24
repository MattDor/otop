package palette

import (
	"github.com/mdoeren/otop/internal/db"
	"github.com/mdoeren/otop/internal/ui/layout"
	"github.com/mdoeren/otop/internal/ui/panel"
	"github.com/mdoeren/otop/internal/ui/workflow"
	"github.com/rivo/tview"
)

// Palette is a command palette modal overlay for opening new panels.
// It lives as a permanent (but initially hidden) page in the root tview.Pages.
type Palette struct {
	app        *tview.Application
	db         *db.DB
	rootPages  *tview.Pages
	manager    *workflow.Manager
	list       *tview.List
	overlay    tview.Primitive
	priorFocus tview.Primitive
}

// New creates a Palette. rootPages is the application-level Pages widget
// so the palette can be shown as a full-screen overlay.
func New(app *tview.Application, database *db.DB, rootPages *tview.Pages, manager *workflow.Manager) *Palette {
	p := &Palette{
		app:       app,
		db:        database,
		rootPages: rootPages,
		manager:   manager,
		list:      tview.NewList(),
	}

	p.list.
		SetBorder(true).
		SetTitle(" Open Panel (Enter to select, Esc to cancel) ")
	p.list.SetDoneFunc(func() { p.Hide() })

	// Center the list with proportional spacers
	inner := tview.NewFlex().
		SetDirection(tview.FlexRow).
		AddItem(nil, 0, 1, false).
		AddItem(p.list, 0, 3, true).
		AddItem(nil, 0, 1, false)

	p.overlay = tview.NewFlex().
		AddItem(nil, 0, 1, false).
		AddItem(inner, 60, 1, true).
		AddItem(nil, 0, 1, false)

	return p
}

// Primitive returns the overlay primitive to be registered as a Pages page.
func (p *Palette) Primitive() tview.Primitive {
	return p.overlay
}

// Show populates the list and reveals the palette overlay.
func (p *Palette) Show() {
	p.priorFocus = p.app.GetFocus()

	p.list.Clear()
	for _, entry := range panel.Global.All() {
		entry := entry // capture
		p.list.AddItem(entry.TypeName, entry.Description, 0, func() {
			p.Hide()
			p.openPanel(entry)
		})
	}

	p.rootPages.ShowPage("palette")
	p.app.SetFocus(p.list)
}

// Hide dismisses the palette and restores the previous focus.
func (p *Palette) Hide() {
	p.rootPages.HidePage("palette")
	if p.priorFocus != nil {
		p.app.SetFocus(p.priorFocus)
	}
}

// openPanel creates a new instance of the given panel type and adds it to the
// active workflow, splitting the currently focused panel vertically.
func (p *Palette) openPanel(entry panel.Entry) {
	w := p.manager.ActiveWorkflow()
	if w == nil {
		return
	}
	newPanel := entry.Factory(p.app, p.db)
	target := w.FocusedPrimitive()
	w.AddPanel(newPanel, target, layout.Vertical)
}
