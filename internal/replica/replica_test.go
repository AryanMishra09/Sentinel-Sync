package replica

import (
	"testing"

	"github.com/aryan-mishra/sentinel-sync/internal/crdt"
)

// counterClock returns a monotonic wall-clock stub starting at base. Distinct
// bases let tests make HLC winners predictable; convergence holds regardless.
func counterClock(base int64) func() int64 {
	n := base
	return func() int64 { n++; return n }
}

func newReplica(id string, base int64, peers ...Peer) *Replica {
	return New(id, peers, counterClock(base))
}

// syncAll cross-feeds every replica's operations into every other replica — a
// full, lossless sync (what Phase 4's transport will do over the network). Logs
// are captured first so the feed set is fixed even as Ingest grows each log.
func syncAll(reps ...*Replica) {
	logs := make([][]crdt.Operation, len(reps))
	for i, r := range reps {
		logs[i] = r.OpLog()
	}
	for i, r := range reps {
		for j, l := range logs {
			if i == j {
				continue
			}
			for _, o := range l {
				r.Ingest(o)
			}
		}
	}
}

func countNodes(r *Replica) int { n, _ := r.Counts(); return n }

// The headline Phase 3 guarantee: independent replicas that exchange their
// operations converge to an identical state hash.
func TestTwoReplicasConverge(t *testing.T) {
	a := newReplica("a", 1000)
	b := newReplica("b", 2000)

	a.CreateNode("1", "Email", 0, 0)
	a.CreateNode("2", "AI", 100, 100)
	a.CreateEdge("e1", "1", "2")
	b.CreateNode("3", "Slack", 200, 200)
	b.RenameNode("3", "Notify")

	if a.Hash() == b.Hash() {
		t.Fatal("replicas should differ before sync")
	}
	syncAll(a, b)

	if a.Hash() != b.Hash() {
		t.Fatalf("did not converge:\n a=%s (%d nodes)\n b=%s (%d nodes)",
			a.Hash(), countNodes(a), b.Hash(), countNodes(b))
	}
	if countNodes(a) != 3 {
		t.Fatalf("expected 3 nodes after merge, got %d", countNodes(a))
	}
}

// Concurrent renames of the same node converge to one HLC-determined winner.
func TestConcurrentRenameConverges(t *testing.T) {
	a := newReplica("a", 1000)
	b := newReplica("b", 2000)

	a.CreateNode("1", "orig", 0, 0)
	syncAll(a, b) // both now know node 1

	a.RenameNode("1", "from-a")
	b.RenameNode("1", "from-b")
	syncAll(a, b)

	if a.Hash() != b.Hash() {
		t.Fatalf("concurrent rename did not converge: a=%s b=%s", a.Hash(), b.Hash())
	}
	na, _ := a.Node("1")
	if na.Title != "from-a" && na.Title != "from-b" {
		t.Fatalf("title should be one of the writes, got %q", na.Title)
	}
}

// Concurrent create (same id, different tags) and delete: the delete only
// observes its own tag, so the concurrent add survives on both replicas
// (add-wins) and they converge.
func TestConcurrentCreateDeleteAddWins(t *testing.T) {
	a := newReplica("a", 1000)
	b := newReplica("b", 2000)

	// Both create node "1" before either has synced — distinct OR-Set tags.
	a.CreateNode("1", "x", 0, 0)
	b.CreateNode("1", "x", 0, 0)
	// a deletes, observing only its own tag.
	a.DeleteNode("1")

	syncAll(a, b)

	_, okA := a.Node("1")
	_, okB := b.Node("1")
	if !okA || !okB {
		t.Fatal("node should survive (add-wins): b's concurrent add was not deleted")
	}
	if a.Hash() != b.Hash() {
		t.Fatalf("add-wins did not converge: a=%s b=%s", a.Hash(), b.Hash())
	}
}

// Ingest is idempotent: applying the same operation twice changes nothing.
func TestIngestIdempotent(t *testing.T) {
	a := newReplica("a", 1000)
	b := newReplica("b", 2000)
	a.CreateNode("1", "Email", 0, 0)

	ops := a.OpLog()
	b.Ingest(ops[0])
	h1, n1 := b.Hash(), b.OpLogLen()
	b.Ingest(ops[0]) // duplicate
	if b.Hash() != h1 || b.OpLogLen() != n1 {
		t.Fatalf("duplicate ingest changed state: hash %s->%s, len %d->%d",
			h1, b.Hash(), n1, b.OpLogLen())
	}
}
