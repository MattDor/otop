package ui

import (
	"testing"

	"github.com/mdoeren/otop/internal/ui/panel"
	"github.com/rivo/tview"
)

func TestIsTextInput(t *testing.T) {
	cases := []struct {
		name string
		p    tview.Primitive
		want bool
	}{
		{"TextArea", tview.NewTextArea(), true},
		{"InputField", tview.NewInputField(), true},
		{"Table", tview.NewTable(), false},
		{"TextView", tview.NewTextView(), false},
		{"nil", nil, false},
	}
	for _, c := range cases {
		if got := isTextInput(c.p); got != c.want {
			t.Errorf("isTextInput(%s) = %v, want %v", c.name, got, c.want)
		}
	}
}

func TestDefaultPanelsAreRegistered(t *testing.T) {
	// NewApp seeds the layout with these; a typo would silently drop them.
	for _, name := range []string{"SessionList", "SQLDetail", "QueryEditor"} {
		if _, ok := panel.Global.Get(name); !ok {
			t.Errorf("panel %q is not registered", name)
		}
	}
}
