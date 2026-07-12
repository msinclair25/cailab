package graph

import (
	"fmt"
	"sort"

	"github.com/msinclair25/cailab/internal/scenario"
)

type Graph struct {
	nodes     map[string]scenario.Node
	adjacency map[string][]scenario.Relationship
}

type Path struct {
	Nodes []string                `json:"nodes"`
	Edges []scenario.Relationship `json:"edges"`
}

func New(nodes []scenario.Node, edges []scenario.Relationship) (*Graph, error) {
	g := &Graph{
		nodes:     make(map[string]scenario.Node, len(nodes)),
		adjacency: make(map[string][]scenario.Relationship, len(nodes)),
	}
	for _, node := range nodes {
		if _, exists := g.nodes[node.ID]; exists {
			return nil, fmt.Errorf("duplicate graph node %q", node.ID)
		}
		g.nodes[node.ID] = node
	}
	for _, edge := range edges {
		if _, ok := g.nodes[edge.From]; !ok {
			return nil, fmt.Errorf("edge %q references unknown source %q", edge.ID, edge.From)
		}
		if _, ok := g.nodes[edge.To]; !ok {
			return nil, fmt.Errorf("edge %q references unknown target %q", edge.ID, edge.To)
		}
		g.adjacency[edge.From] = append(g.adjacency[edge.From], edge)
	}
	for id := range g.adjacency {
		sort.Slice(g.adjacency[id], func(i, j int) bool {
			left, right := g.adjacency[id][i], g.adjacency[id][j]
			if left.To != right.To {
				return left.To < right.To
			}
			return left.ID < right.ID
		})
	}
	return g, nil
}

// FindPath returns a deterministic shortest directed path.
func (g *Graph) FindPath(from, to string) (Path, bool) {
	if _, ok := g.nodes[from]; !ok {
		return Path{}, false
	}
	if _, ok := g.nodes[to]; !ok {
		return Path{}, false
	}
	if from == to {
		return Path{Nodes: []string{from}}, true
	}

	visited := map[string]bool{from: true}
	previous := make(map[string]predecessor)
	queue := []string{from}

	for len(queue) > 0 {
		current := queue[0]
		queue = queue[1:]
		for _, edge := range g.adjacency[current] {
			if visited[edge.To] {
				continue
			}
			visited[edge.To] = true
			previous[edge.To] = predecessor{node: current, edge: edge}
			if edge.To == to {
				return buildPath(from, to, previous), true
			}
			queue = append(queue, edge.To)
		}
	}
	return Path{}, false
}

func buildPath(from, to string, previous map[string]predecessor) Path {
	nodes := []string{to}
	var edges []scenario.Relationship
	for current := to; current != from; {
		step := previous[current]
		edges = append(edges, step.edge)
		nodes = append(nodes, step.node)
		current = step.node
	}
	reverseStrings(nodes)
	for i, j := 0, len(edges)-1; i < j; i, j = i+1, j-1 {
		edges[i], edges[j] = edges[j], edges[i]
	}
	return Path{Nodes: nodes, Edges: edges}
}

type predecessor struct {
	node string
	edge scenario.Relationship
}

func reverseStrings(values []string) {
	for i, j := 0, len(values)-1; i < j; i, j = i+1, j-1 {
		values[i], values[j] = values[j], values[i]
	}
}
