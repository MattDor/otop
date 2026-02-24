package ui

import (
	"time"

	"github.com/gdamore/tcell/v2"
	"github.com/mdoeren/otop/internal/db"
	"github.com/mdoeren/otop/internal/ui/layout"
	"github.com/mdoeren/otop/internal/ui/palette"
	"github.com/mdoeren/otop/internal/ui/panel"
	"github.com/mdoeren/otop/internal/ui/workflow"

	// Blank import triggers all panel init() registrations.
	_ "github.com/mdoeren/otop/internal/ui/panels"

	"github.com/rivo/tview"
)

const refreshInterval = 5 * time.Second

// App is the top-level TUI application.
type App struct {
	tview *tview.Application
}

// NewApp creates and wires up the extensible panel-based TUI.
// main.go calls this with the open database handle.
func NewApp(database *db.DB) *App {
	tapp := tview.NewApplication()

	// Root Pages: holds the workflow manager UI and the palette overlay.
	rootPages := tview.NewPages()

	manager := workflow.NewManager(tapp)

	// Create the default "Sessions" workflow.
	w := workflow.New("Sessions", tapp, database, refreshInterval)

	// Seed with a SessionList panel.
	if entry, ok := panel.Global.Get("SessionList"); ok {
		sessionPanel := entry.Factory(tapp, database)
		w.AddPanel(sessionPanel, nil, layout.Horizontal)
	}

	manager.AddWorkflow(w)

	// Register the workflow manager as the main page.
	rootPages.AddPage("main", manager.RootPrimitive(), true, true)

	// Create and register the command palette overlay (initially hidden).
	pal := palette.New(tapp, database, rootPages, manager)
	rootPages.AddPage("palette", pal.Primitive(), true, false)

	// Global keybindings.
	tapp.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
		switch {
		case event.Key() == tcell.KeyCtrlP:
			pal.Show()
			return nil

		case event.Key() == tcell.KeyCtrlW:
			aw := manager.ActiveWorkflow()
			if aw == nil {
				return event
			}
			// Find which panel owns the focused primitive.
			focused := tapp.GetFocus()
			for _, p := range aw.Panels() {
				if p.Primitive() == focused {
					aw.RemovePanel(p)
					return nil
				}
			}
			return event

		case event.Key() == tcell.KeyRight && event.Modifiers()&tcell.ModCtrl != 0:
			manager.SwitchNext()
			return nil

		case event.Key() == tcell.KeyLeft && event.Modifiers()&tcell.ModCtrl != 0:
			manager.SwitchPrev()
			return nil

		case event.Key() == tcell.KeyTab:
			if aw := manager.ActiveWorkflow(); aw != nil {
				aw.FocusCycle(false)
				return nil
			}

		case event.Key() == tcell.KeyBacktab:
			if aw := manager.ActiveWorkflow(); aw != nil {
				aw.FocusCycle(true)
				return nil
			}

		// Alt+Arrow: resize focused panel
		case event.Key() == tcell.KeyRight && event.Modifiers()&tcell.ModAlt != 0:
			if aw := manager.ActiveWorkflow(); aw != nil {
				aw.ResizeFocused(layout.Horizontal, 1)
				return nil
			}
		case event.Key() == tcell.KeyLeft && event.Modifiers()&tcell.ModAlt != 0:
			if aw := manager.ActiveWorkflow(); aw != nil {
				aw.ResizeFocused(layout.Horizontal, -1)
				return nil
			}
		case event.Key() == tcell.KeyDown && event.Modifiers()&tcell.ModAlt != 0:
			if aw := manager.ActiveWorkflow(); aw != nil {
				aw.ResizeFocused(layout.Vertical, 1)
				return nil
			}
		case event.Key() == tcell.KeyUp && event.Modifiers()&tcell.ModAlt != 0:
			if aw := manager.ActiveWorkflow(); aw != nil {
				aw.ResizeFocused(layout.Vertical, -1)
				return nil
			}
		}
		return event
	})

	tapp.SetRoot(rootPages, true)
	return &App{tview: tapp}
}

// Run starts the TUI event loop.
func (a *App) Run() error {
	return a.tview.Run()
}
