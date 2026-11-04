package layout

import (
	"testing"

	"github.com/rivo/tview"
)

func leaf(p tview.Primitive) *Node { return &Node{Primitive: p} }

func boxes(n int) []tview.Primitive {
	out := make([]tview.Primitive, n)
	for i := range out {
		out[i] = tview.NewBox()
	}
	return out
}

func assertOrder(t *testing.T, root *Node, want ...tview.Primitive) {
	t.Helper()
	got := FocusOrder(root)
	if len(got) != len(want) {
		t.Fatalf("focus order has %d entries, want %d", len(got), len(want))
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("focus order[%d] mismatch", i)
		}
	}
}

func TestAddChildClampsProportion(t *testing.T) {
	n := &Node{}
	n.AddChild(leaf(tview.NewBox()), 0)
	n.AddChild(leaf(tview.NewBox()), -3)
	n.AddChild(leaf(tview.NewBox()), 4)
	want := []int{1, 1, 4}
	for i, w := range want {
		if n.Proportions[i] != w {
			t.Errorf("Proportions[%d] = %d, want %d", i, n.Proportions[i], w)
		}
	}
}

func TestInsertNearTarget_SameDirectionAppendsSiblingAfterTarget(t *testing.T) {
	b := boxes(3)
	root := &Node{Direction: Horizontal}
	root.AddChild(leaf(b[0]), 1)
	root.AddChild(leaf(b[1]), 3)

	if !InsertNearTarget(root, b[0], leaf(b[2]), Horizontal) {
		t.Fatal("target not found")
	}
	assertOrder(t, root, b[0], b[2], b[1])
	if got := root.Proportions; len(got) != 3 || got[0] != 1 || got[1] != 1 || got[2] != 3 {
		t.Errorf("proportions = %v, want [1 1 3]", got)
	}
}

func TestInsertNearTarget_OtherDirectionCreatesSplit(t *testing.T) {
	b := boxes(2)
	root := &Node{Direction: Horizontal}
	root.AddChild(leaf(b[0]), 1)

	if !InsertNearTarget(root, b[0], leaf(b[1]), Vertical) {
		t.Fatal("target not found")
	}
	if len(root.Children) != 1 {
		t.Fatalf("root has %d children, want 1 split", len(root.Children))
	}
	split := root.Children[0]
	if split.IsLeaf() || split.Direction != Vertical || len(split.Children) != 2 {
		t.Fatalf("expected vertical split with 2 children, got %+v", split)
	}
	assertOrder(t, root, b[0], b[1])
}

func TestInsertNearTarget_FindsNestedTargetAndReportsMissing(t *testing.T) {
	b := boxes(4)
	root := &Node{Direction: Horizontal}
	root.AddChild(leaf(b[0]), 1)
	InsertNearTarget(root, b[0], leaf(b[1]), Vertical)

	if !InsertNearTarget(root, b[1], leaf(b[2]), Vertical) {
		t.Fatal("nested target not found")
	}
	assertOrder(t, root, b[0], b[1], b[2])

	if InsertNearTarget(root, tview.NewBox(), leaf(b[3]), Vertical) {
		t.Error("InsertNearTarget returned true for unknown target")
	}
}

func TestRemoveChildAndCollapse(t *testing.T) {
	b := boxes(3)
	root := &Node{Direction: Horizontal}
	root.AddChild(leaf(b[0]), 1)
	InsertNearTarget(root, b[0], leaf(b[1]), Vertical) // root -> [split(b0,b1)]
	root.AddChild(leaf(b[2]), 2)                       // root -> [split(b0,b1), b2]

	if !root.RemoveChild(b[1]) {
		t.Fatal("RemoveChild did not find nested leaf")
	}
	Collapse(root)
	// The single-child split must be unwrapped into a plain leaf.
	if !root.Children[0].IsLeaf() || root.Children[0].Primitive != b[0] {
		t.Errorf("single-child split was not unwrapped")
	}
	assertOrder(t, root, b[0], b[2])

	root.RemoveChild(b[0])
	root.RemoveChild(b[2])
	if root.RemoveChild(b[2]) {
		t.Error("RemoveChild returned true for already removed leaf")
	}
	if len(root.Children) != 0 || len(root.Proportions) != 0 {
		t.Errorf("children/proportions not emptied: %d/%d", len(root.Children), len(root.Proportions))
	}
}

func TestCollapseDropsEmptySplits(t *testing.T) {
	b := boxes(1)
	root := &Node{Direction: Horizontal}
	root.AddChild(&Node{Direction: Vertical}, 1) // empty split
	root.AddChild(leaf(b[0]), 1)
	Collapse(root)
	if len(root.Children) != 1 || len(root.Proportions) != 1 {
		t.Fatalf("empty split not dropped: %d children", len(root.Children))
	}
	assertOrder(t, root, b[0])
}

func TestResizeChildClampsAndIgnoresBadIndex(t *testing.T) {
	n := &Node{}
	n.AddChild(leaf(tview.NewBox()), 2)
	n.ResizeChild(0, 3)
	if n.Proportions[0] != 5 {
		t.Errorf("after +3: %d, want 5", n.Proportions[0])
	}
	n.ResizeChild(0, -10)
	if n.Proportions[0] != 1 {
		t.Errorf("after -10: %d, want clamped 1", n.Proportions[0])
	}
	n.ResizeChild(5, 1)  // must not panic
	n.ResizeChild(-1, 1) // must not panic
}

func TestBuild(t *testing.T) {
	b := boxes(2)
	if Build(leaf(b[0])) != b[0] {
		t.Error("Build(leaf) should return the leaf primitive itself")
	}
	if _, ok := Build(&Node{}).(*tview.Box); !ok {
		t.Error("Build(empty split) should return a placeholder Box")
	}

	root := &Node{Direction: Vertical}
	root.AddChild(leaf(b[0]), 1)
	root.AddChild(leaf(b[1]), 1)
	flex, ok := Build(root).(*tview.Flex)
	if !ok {
		t.Fatal("Build(split) should return a Flex")
	}
	if flex.GetItemCount() != 2 || flex.GetItem(0) != b[0] || flex.GetItem(1) != b[1] {
		t.Error("Flex items do not match children")
	}
}

func TestFindParent(t *testing.T) {
	b := boxes(2)
	root := &Node{Direction: Horizontal}
	root.AddChild(leaf(b[0]), 1)
	InsertNearTarget(root, b[0], leaf(b[1]), Vertical)

	parent, idx := FindParent(root, b[1])
	if parent != root.Children[0] || idx != 1 {
		t.Errorf("FindParent = (%p, %d), want (split, 1)", parent, idx)
	}
	if p, i := FindParent(root, tview.NewBox()); p != nil || i != -1 {
		t.Errorf("FindParent(unknown) = (%v, %d), want (nil, -1)", p, i)
	}
}
