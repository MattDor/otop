package layout

import "github.com/rivo/tview"

// Direction controls how a split node arranges its children.
type Direction int

const (
	Horizontal Direction = iota // children side-by-side (FlexColumn)
	Vertical                    // children stacked (FlexRow)
)

// Node is a tree element representing either a split pane (internal node)
// or a single panel (leaf node with a non-nil Primitive).
type Node struct {
	Direction   Direction
	Children    []*Node
	Proportions []int           // parallel with Children; each ≥ 1
	Primitive   tview.Primitive // non-nil for leaf nodes only
	PanelName   string          // informational; used for future serialization
}

// IsLeaf returns true when this node holds a terminal primitive.
func (n *Node) IsLeaf() bool {
	return n.Primitive != nil
}

// AddChild appends a child with the given proportion.
func (n *Node) AddChild(child *Node, proportion int) {
	if proportion < 1 {
		proportion = 1
	}
	n.Children = append(n.Children, child)
	n.Proportions = append(n.Proportions, proportion)
}

// RemoveChild finds and removes the leaf whose Primitive == p from this subtree.
// Returns true if the leaf was found and removed.
func (n *Node) RemoveChild(p tview.Primitive) bool {
	for i, child := range n.Children {
		if child.IsLeaf() && child.Primitive == p {
			n.Children = append(n.Children[:i], n.Children[i+1:]...)
			n.Proportions = append(n.Proportions[:i], n.Proportions[i+1:]...)
			return true
		}
		if !child.IsLeaf() && child.RemoveChild(p) {
			return true
		}
	}
	return false
}

// ResizeChild adjusts the proportion of the child at idx by delta.
// The proportion is clamped to a minimum of 1.
func (n *Node) ResizeChild(idx, delta int) {
	if idx < 0 || idx >= len(n.Proportions) {
		return
	}
	v := n.Proportions[idx] + delta
	if v < 1 {
		v = 1
	}
	n.Proportions[idx] = v
}

// Collapse walks n's subtree post-order and replaces any single-child split
// node with its sole child, and removes empty split nodes.
func Collapse(n *Node) {
	for i := len(n.Children) - 1; i >= 0; i-- {
		child := n.Children[i]
		if child.IsLeaf() {
			continue
		}
		Collapse(child)
		switch len(child.Children) {
		case 0:
			// Drop empty split node
			n.Children = append(n.Children[:i], n.Children[i+1:]...)
			n.Proportions = append(n.Proportions[:i], n.Proportions[i+1:]...)
		case 1:
			// Unwrap single-child split
			n.Children[i] = child.Children[0]
		}
	}
}
