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

	// Register the workflow first so the status bar is wired before panels
	// are added (panels pick up the status function in AddPanel).
	manager.AddWorkflow(w)

	// Seed with a SessionList panel and a detail panel below it.
	if entry, ok := panel.Global.Get("SessionList"); ok {
		sessionPanel := entry.Factory(tapp, database)
		w.AddPanel(sessionPanel, nil, layout.Horizontal)

		if detail, ok := panel.Global.Get("SQLDetail"); ok {
			w.AddPanel(detail.Factory(tapp, database), sessionPanel.Primitive(), layout.Vertical)
		}
		// AddPanel focuses the newest panel; start on the session list.
		w.Focus(sessionPanel)
	}

	// Register the workflow manager as the main page.
	rootPages.AddPage("main", manager.RootPrimitive(), true, true)

	// Create and register the command palette overlay (initially hidden).
	pal := palette.New(tapp, database, rootPages, manager)
	rootPages.AddPage("palette", pal.Primitive(), true, false)

	// Global keybindings.
	tapp.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
		switch {
		case event.Key() == tcell.KeyCtrlQ,
			event.Key() == tcell.KeyRune && event.Rune() == 'q' && !isTextInput(tapp.GetFocus()):
			tapp.Stop()
			return nil

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

// isTextInput reports whether p is a widget that consumes typed characters,
// in which case single-letter shortcuts must not be intercepted.
func isTextInput(p tview.Primitive) bool {
	switch p.(type) {
	case *tview.TextArea, *tview.InputField:
		return true
	}
	return false
}

// Run starts the TUI event loop.
func (a *App) Run() error {
	return a.tview.Run()
}
