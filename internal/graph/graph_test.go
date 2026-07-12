package graph

import (
	"testing"

	"github.com/msinclair25/cailab/internal/scenario"
)

func TestFindPathIsShortestAndDeterministic(t *testing.T) {
	t.Parallel()
	nodes := []scenario.Node{{ID: "a"}, {ID: "b"}, {ID: "c"}, {ID: "d"}}
	edges := []scenario.Relationship{
		{ID: "z", From: "a", To: "c"},
		{ID: "b-d", From: "b", To: "d"},
		{ID: "a-b", From: "a", To: "b"},
		{ID: "c-d", From: "c", To: "d"},
	}
	g, err := New(nodes, edges)
	if err != nil {
		t.Fatal(err)
	}
	path, ok := g.FindPath("a", "d")
	if !ok {
		t.Fatal("FindPath() did not find a path")
	}
	want := []string{"a", "b", "d"}
	if len(path.Nodes) != len(want) {
		t.Fatalf("path nodes = %v, want %v", path.Nodes, want)
	}
	for i := range want {
		if path.Nodes[i] != want[i] {
			t.Fatalf("path nodes = %v, want %v", path.Nodes, want)
		}
	}
}

func TestNewRejectsUnknownEndpoint(t *testing.T) {
	t.Parallel()
	_, err := New([]scenario.Node{{ID: "a"}}, []scenario.Relationship{{ID: "bad", From: "a", To: "missing"}})
	if err == nil {
		t.Fatal("New() error = nil, want unknown target error")
	}
}
