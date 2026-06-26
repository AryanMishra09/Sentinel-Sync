package graph

import (
	"encoding/json"
	"testing"

	"github.com/aryan-mishra/sentinel-sync/internal/crdt"
)

// ts builds an HLC timestamp for tests.
func ts(replicaID string, physical, logical int64) crdt.HLCTimestamp {
	return crdt.HLCTimestamp{Physical: physical, Logical: logical, ReplicaID: replicaID}
}

func mustJSON(v any) json.RawMessage {
	b, err := json.Marshal(v)
	if err != nil {
		panic(err)
	}
	return b
}

// op builds an operation with an explicit HLC and payload. State.Apply does not
// dedup (that is the replica's job), so the op ID is irrelevant here.
func op(typ crdt.OpType, hlc crdt.HLCTimestamp, payload any) crdt.Operation {
	return crdt.Operation{Type: typ, Payload: mustJSON(payload), HLC: hlc}
}

func TestApplyCreateMaterializes(t *testing.T) {
	s := NewState()
	tag := crdt.Tag{ReplicaID: "a", Counter: 1}
	if err := s.Apply(op(crdt.OpCreateNode, ts("a", 1, 0), CreateNodePayload{ID: "1", Title: "Email", Tag: tag})); err != nil {
		t.Fatal(err)
	}
	snap := s.Snapshot()
	if len(snap.Nodes) != 1 || snap.Nodes[0].Title != "Email" {
		t.Fatalf("unexpected snapshot: %+v", snap)
	}
}

// LWW: the higher HLC wins regardless of apply order.
func TestRenameLWWHigherHLCWins(t *testing.T) {
	build := func(order int) string {
		s := NewState()
		s.Apply(op(crdt.OpCreateNode, ts("a", 1, 0), CreateNodePayload{ID: "1", Title: "orig", Tag: crdt.Tag{ReplicaID: "a", Counter: 1}}))
		hi := op(crdt.OpRenameNode, ts("b", 10, 0), RenameNodePayload{ID: "1", Title: "winner"})
		lo := op(crdt.OpRenameNode, ts("a", 5, 0), RenameNodePayload{ID: "1", Title: "loser"})
		if order == 0 {
			s.Apply(hi)
			s.Apply(lo)
		} else {
			s.Apply(lo)
			s.Apply(hi)
		}
		return s.Snapshot().Nodes[0].Title
	}
	if a, b := build(0), build(1); a != "winner" || b != "winner" {
		t.Fatalf("LWW not order-independent: %q vs %q", a, b)
	}
}

// Add-wins: a create concurrent with a delete (delete only saw the first tag)
// keeps the node alive.
func TestORSetAddWins(t *testing.T) {
	s := NewState()
	s.Apply(op(crdt.OpCreateNode, ts("a", 1, 0), CreateNodePayload{ID: "1", Title: "x", Tag: crdt.Tag{ReplicaID: "a", Counter: 1}}))
	// Concurrent add on replica b with a different tag.
	s.Apply(op(crdt.OpCreateNode, ts("b", 1, 0), CreateNodePayload{ID: "1", Title: "x", Tag: crdt.Tag{ReplicaID: "b", Counter: 1}}))
	// Delete that only observed replica a's tag.
	s.Apply(op(crdt.OpDeleteNode, ts("a", 2, 0), DeleteNodePayload{ID: "1", RemovedTags: []crdt.Tag{{ReplicaID: "a", Counter: 1}}}))

	if !s.HasNode("1") {
		t.Fatal("node should survive: concurrent add was not observed by the delete (add-wins)")
	}
}

// Dangling edge: an edge whose endpoint is deleted disappears from the view, and
// returns if the endpoint is re-added.
func TestDanglingEdgeFilteredAndRestored(t *testing.T) {
	s := NewState()
	s.Apply(op(crdt.OpCreateNode, ts("a", 1, 0), CreateNodePayload{ID: "1", Tag: crdt.Tag{ReplicaID: "a", Counter: 1}}))
	s.Apply(op(crdt.OpCreateNode, ts("a", 2, 0), CreateNodePayload{ID: "2", Tag: crdt.Tag{ReplicaID: "a", Counter: 2}}))
	s.Apply(op(crdt.OpCreateEdge, ts("a", 3, 0), CreateEdgePayload{ID: "e1", Source: "1", Target: "2", Tag: crdt.Tag{ReplicaID: "a", Counter: 3}}))

	if len(s.Snapshot().Edges) != 1 {
		t.Fatal("edge should be visible")
	}
	// Delete node 2 -> edge dangles -> filtered.
	s.Apply(op(crdt.OpDeleteNode, ts("a", 4, 0), DeleteNodePayload{ID: "2", RemovedTags: []crdt.Tag{{ReplicaID: "a", Counter: 2}}}))
	if len(s.Snapshot().Edges) != 0 {
		t.Fatal("dangling edge should be filtered after endpoint deletion")
	}
	// Re-add node 2 -> edge reappears (edge OR-Set was never mutated).
	s.Apply(op(crdt.OpCreateNode, ts("a", 5, 0), CreateNodePayload{ID: "2", Tag: crdt.Tag{ReplicaID: "a", Counter: 5}}))
	if len(s.Snapshot().Edges) != 1 {
		t.Fatal("edge should reappear when endpoint is re-added")
	}
}

// Convergence: the same operations applied in two different orders produce the
// same state hash. This is the core CRDT guarantee.
func TestConvergenceOrderIndependent(t *testing.T) {
	ops := []crdt.Operation{
		op(crdt.OpCreateNode, ts("a", 1, 0), CreateNodePayload{ID: "1", Title: "A", Tag: crdt.Tag{ReplicaID: "a", Counter: 1}}),
		op(crdt.OpCreateNode, ts("b", 2, 0), CreateNodePayload{ID: "2", Title: "B", Tag: crdt.Tag{ReplicaID: "b", Counter: 1}}),
		op(crdt.OpRenameNode, ts("c", 9, 0), RenameNodePayload{ID: "1", Title: "renamed"}),
		op(crdt.OpMoveNode, ts("a", 4, 0), MoveNodePayload{ID: "2", X: 5, Y: 6}),
		op(crdt.OpCreateEdge, ts("b", 5, 0), CreateEdgePayload{ID: "e1", Source: "1", Target: "2", Tag: crdt.Tag{ReplicaID: "b", Counter: 2}}),
	}

	s1 := NewState()
	for _, o := range ops {
		s1.Apply(o)
	}
	// Apply in reverse order on a fresh state.
	s2 := NewState()
	for i := len(ops) - 1; i >= 0; i-- {
		s2.Apply(ops[i])
	}

	if s1.Hash() != s2.Hash() {
		t.Fatalf("hashes diverge across apply orders:\n forward=%s\n reverse=%s", s1.Hash(), s2.Hash())
	}
}
