package graph

// Node is a graph node containing links to parents and child nodes
type Node struct {
	ID       string
	Parents  map[string]*Node
	Children map[string]*Node
}

// Graph is the graph that manages the nodes and which graph operations work on
type Graph struct {
	nodes map[string]*Node
}

// NewGraph initialises a new graph
func NewGraph() *Graph {
	return &Graph{
		nodes: make(map[string]*Node),
	}
}

// GetNodes returns the map of nodes in the graph
func (g Graph) GetNodes() map[string]*Node {
	return g.nodes
}

// GetNode returns a single node in the graph given the ID
func (g Graph) GetNode(id string) *Node {
	return g.nodes[id]
}
