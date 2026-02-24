package workflow

import (
	"fmt"

	"github.com/rivo/tview"
)

// Manager owns the tab bar and the Pages widget that holds each workflow page.
type Manager struct {
	app       *tview.Application
	workflows []*Workflow
	active    int
	tabBar    *tview.TextView
	pages     *tview.Pages
	root      *tview.Flex
}

// NewManager creates a Manager with an empty tab bar and no workflows.
func NewManager(app *tview.Application) *Manager {
	m := &Manager{
		app:    app,
		active: -1,
		tabBar: tview.NewTextView().SetDynamicColors(true),
		pages:  tview.NewPages(),
	}
	m.root = tview.NewFlex().
		SetDirection(tview.FlexRow).
		AddItem(m.tabBar, 1, 0, false).
		AddItem(m.pages, 0, 1, true)
	return m
}

// AddWorkflow registers w and activates it if it is the first workflow.
func (m *Manager) AddWorkflow(w *Workflow) {
	w.SetPages(m.pages)
	m.workflows = append(m.workflows, w)
	m.renderTabBar()
	if m.active == -1 {
		m.SwitchTo(0)
	}
}

// SwitchTo stops the currently active workflow and starts the one at index.
func (m *Manager) SwitchTo(index int) {
	if index < 0 || index >= len(m.workflows) {
		return
	}
	if m.active >= 0 && m.active < len(m.workflows) {
		m.workflows[m.active].Stop()
	}
	m.active = index
	w := m.workflows[index]
	m.pages.SwitchToPage(w.pageKey)
	w.Start()
	m.renderTabBar()
	if len(w.focusOrder) > 0 {
		m.app.SetFocus(w.focusOrder[0])
	}
}

// SwitchNext moves to the next workflow tab, wrapping around.
func (m *Manager) SwitchNext() {
	if len(m.workflows) == 0 {
		return
	}
	m.SwitchTo((m.active + 1) % len(m.workflows))
}

// SwitchPrev moves to the previous workflow tab, wrapping around.
func (m *Manager) SwitchPrev() {
	if len(m.workflows) == 0 {
		return
	}
	next := m.active - 1
	if next < 0 {
		next = len(m.workflows) - 1
	}
	m.SwitchTo(next)
}

// ActiveWorkflow returns the currently visible workflow, or nil.
func (m *Manager) ActiveWorkflow() *Workflow {
	if m.active < 0 || m.active >= len(m.workflows) {
		return nil
	}
	return m.workflows[m.active]
}

// RootPrimitive returns the top-level primitive (tabBar + pages) to set as root.
func (m *Manager) RootPrimitive() tview.Primitive {
	return m.root
}

// Count returns the number of registered workflows.
func (m *Manager) Count() int {
	return len(m.workflows)
}

// renderTabBar refreshes the tab bar text, highlighting the active tab.
func (m *Manager) renderTabBar() {
	text := " "
	for i, w := range m.workflows {
		if i == m.active {
			text += fmt.Sprintf("[black:white:b] %s [-:-:-]  ", w.Name)
		} else {
			text += fmt.Sprintf("[::d] %s [-:-:-]  ", w.Name)
		}
	}
	m.tabBar.SetText(text)
}
