package graph

// Node is a graph node containing links to parents and child nodes
type Node struct {
	ID       string
	Parents  map[string]*Node
	Children map[string]*Node
}
