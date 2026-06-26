package graph

import (
	"errors"
	"sync"
	"testing"
)

func TestCreateAndSnapshot(t *testing.T) {
	g := New()
	if _, err := g.CreateNode("1", "Email", 0, 0); err != nil {
		t.Fatalf("create: %v", err)
	}
	if _, err := g.CreateNode("1", "dup", 0, 0); !errors.Is(err, ErrNodeExists) {
		t.Fatalf("want ErrNodeExists, got %v", err)
	}
	s := g.Snapshot()
	if len(s.Nodes) != 1 || s.Nodes[0].Title != "Email" {
		t.Fatalf("unexpected snapshot: %+v", s)
	}
}

func TestRenameAndMove(t *testing.T) {
	g := New()
	g.CreateNode("1", "Email", 0, 0)

	if _, err := g.RenameNode("1", "AI Processor"); err != nil {
		t.Fatalf("rename: %v", err)
	}
	if _, err := g.MoveNode("1", 400, 200); err != nil {
		t.Fatalf("move: %v", err)
	}
	if _, err := g.RenameNode("missing", "x"); !errors.Is(err, ErrNodeNotFound) {
		t.Fatalf("want ErrNodeNotFound, got %v", err)
	}

	n := g.Snapshot().Nodes[0]
	if n.Title != "AI Processor" || n.X != 400 || n.Y != 200 {
		t.Fatalf("unexpected node: %+v", n)
	}
}

func TestEdgeRequiresEndpoints(t *testing.T) {
	g := New()
	g.CreateNode("1", "Email", 0, 0)

	if _, err := g.CreateEdge("e1", "1", "2"); !errors.Is(err, ErrEndpointMissing) {
		t.Fatalf("want ErrEndpointMissing, got %v", err)
	}
	g.CreateNode("2", "Slack", 0, 0)
	if _, err := g.CreateEdge("e1", "1", "2"); err != nil {
		t.Fatalf("create edge: %v", err)
	}
}

// Deleting a node must cascade to its edges (Phase 1 eager invariant).
func TestDeleteNodeCascadesEdges(t *testing.T) {
	g := New()
	g.CreateNode("1", "Email", 0, 0)
	g.CreateNode("2", "Slack", 0, 0)
	g.CreateEdge("e1", "1", "2")

	if err := g.DeleteNode("1"); err != nil {
		t.Fatalf("delete: %v", err)
	}
	s := g.Snapshot()
	if len(s.Edges) != 0 {
		t.Fatalf("edge should have been cascaded, got %d", len(s.Edges))
	}
}

// The engine must be safe under concurrent writers (run with -race).
func TestConcurrentWrites(t *testing.T) {
	g := New()
	var wg sync.WaitGroup
	for i := range 50 {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			id := string(rune('A' + i%26))
			g.CreateNode(id, "n", 0, 0)
			g.RenameNode(id, "renamed")
			g.MoveNode(id, float64(i), float64(i))
			g.Snapshot()
		}(i)
	}
	wg.Wait()
}
