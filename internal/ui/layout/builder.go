package layout

import "github.com/rivo/tview"

// Build converts a Node tree into a tview.Primitive tree.
// Leaf nodes return their Primitive directly.
// Internal nodes become *tview.Flex containers.
// The tree is rebuilt from scratch on every call (O(n_panels), negligible).
func Build(root *Node) tview.Primitive {
	if root.IsLeaf() {
		return root.Primitive
	}
	if len(root.Children) == 0 {
		return tview.NewBox() // empty placeholder
	}

	var flex *tview.Flex
	if root.Direction == Horizontal {
		flex = tview.NewFlex().SetDirection(tview.FlexColumn)
	} else {
		flex = tview.NewFlex().SetDirection(tview.FlexRow)
	}

	for i, child := range root.Children {
		proportion := 1
		if i < len(root.Proportions) {
			proportion = root.Proportions[i]
		}
		flex.AddItem(Build(child), 0, proportion, false)
	}
	return flex
}

// FocusOrder returns all leaf primitives in depth-first order.
// This slice drives Tab/Shift+Tab focus cycling.
func FocusOrder(root *Node) []tview.Primitive {
	if root.IsLeaf() {
		return []tview.Primitive{root.Primitive}
	}
	var out []tview.Primitive
	for _, child := range root.Children {
		out = append(out, FocusOrder(child)...)
	}
	return out
}

// FindParent returns the parent node and child index of the leaf whose
// Primitive == target. Returns (nil, -1) if not found.
func FindParent(root *Node, target tview.Primitive) (*Node, int) {
	for i, child := range root.Children {
		if child.IsLeaf() && child.Primitive == target {
			return root, i
		}
		if !child.IsLeaf() {
			if p, idx := FindParent(child, target); p != nil {
				return p, idx
			}
		}
	}
	return nil, -1
}

// InsertNearTarget inserts newLeaf adjacent to target in the tree.
// If the parent split direction matches dir, newLeaf is appended as a sibling.
// Otherwise, target is replaced by a new split node containing [target, newLeaf].
func InsertNearTarget(root *Node, target tview.Primitive, newLeaf *Node, dir Direction) bool {
	for i, child := range root.Children {
		if child.IsLeaf() && child.Primitive == target {
			if root.Direction == dir {
				// Insert after target
				newChildren := make([]*Node, 0, len(root.Children)+1)
				newProps := make([]int, 0, len(root.Proportions)+1)
				newChildren = append(newChildren, root.Children[:i+1]...)
				newChildren = append(newChildren, newLeaf)
				newChildren = append(newChildren, root.Children[i+1:]...)
				newProps = append(newProps, root.Proportions[:i+1]...)
				newProps = append(newProps, 1)
				newProps = append(newProps, root.Proportions[i+1:]...)
				root.Children = newChildren
				root.Proportions = newProps
			} else {
				// Replace target leaf with a new split containing [target, newLeaf]
				split := &Node{Direction: dir}
				split.AddChild(child, 1)
				split.AddChild(newLeaf, 1)
				root.Children[i] = split
			}
			return true
		}
		if !child.IsLeaf() && InsertNearTarget(child, target, newLeaf, dir) {
			return true
		}
	}
	return false
}
