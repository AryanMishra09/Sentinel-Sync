package simulation

import (
	"testing"
	"time"

	"github.com/aryan-mishra/sentinel-sync/internal/replica"
)

func newTestReplica(id string) *replica.Replica {
	n := int64(1000)
	return replica.New(id, nil, func() int64 { n++; return n })
}

// Start with 3 users at 20 ops/sec; after 300 ms we expect at least 10 ops.
func TestSimulatorOpsAccumulate(t *testing.T) {
	r := newTestReplica("a")
	sim := NewSimulator(r)

	sim.Start(3, 20.0)
	time.Sleep(300 * time.Millisecond)
	sim.Stop()

	stats := sim.Stats()
	if stats.Running {
		t.Fatal("simulator should be stopped after Stop()")
	}
	if stats.TotalOps < 5 {
		t.Fatalf("expected ≥5 ops in 300ms with 3 users @ 20 ops/sec, got %d", stats.TotalOps)
	}
}

// Start/Stop twice — should not panic or deadlock.
func TestSimulatorStartStopIdempotent(t *testing.T) {
	r := newTestReplica("a")
	sim := NewSimulator(r)

	sim.Start(2, 5.0)
	sim.Stop()
	sim.Stop() // second stop is a no-op

	sim.Start(1, 10.0) // restart is allowed
	time.Sleep(50 * time.Millisecond)
	sim.Stop()

	if sim.Stats().Running {
		t.Fatal("should be stopped")
	}
}

// Start with invalid params — should be a no-op, not panic.
func TestSimulatorInvalidParams(t *testing.T) {
	r := newTestReplica("a")
	sim := NewSimulator(r)

	sim.Start(0, 5.0)   // users <= 0
	sim.Start(5, 0.0)   // opsPerSec <= 0
	sim.Start(-1, -1.0) // both invalid

	if sim.Stats().Running {
		t.Fatal("invalid params should leave simulator stopped")
	}
}

// syncReplicas cross-feeds the op logs of two replicas — the same exchange the
// transport does live, using only public API so there is no import cycle.
func syncReplicas(a, b *replica.Replica) {
	aLog := a.OpLog()
	bLog := b.OpLog()
	for _, op := range aLog {
		b.Ingest(op)
	}
	for _, op := range bLog {
		a.Ingest(op)
	}
}

// Simulator-generated operations are valid CRDTs: two replicas fed the same
// ops converge to the same state hash regardless of generation order.
func TestSimulatorConvergence(t *testing.T) {
	a := newTestReplica("a")
	b := newTestReplica("b")

	sim := NewSimulator(a)
	sim.Start(5, 30.0) // 5 users @ 30 ops/sec
	time.Sleep(300 * time.Millisecond)
	sim.Stop()

	if a.OpLogLen() == 0 {
		t.Fatal("simulator generated no ops")
	}

	syncReplicas(a, b)

	if a.Hash() != b.Hash() {
		t.Fatalf("simulator ops did not converge:\n a=%s (%d ops)\n b=%s",
			a.Hash(), a.OpLogLen(), b.Hash())
	}
}

// Ops counter is cumulative across restarts (totalOps is not reset on Stop).
func TestSimulatorStatsCumulative(t *testing.T) {
	r := newTestReplica("a")
	sim := NewSimulator(r)

	sim.Start(2, 20.0)
	time.Sleep(150 * time.Millisecond)
	sim.Stop()
	after1 := sim.Stats().TotalOps

	sim.Start(2, 20.0)
	time.Sleep(150 * time.Millisecond)
	sim.Stop()
	after2 := sim.Stats().TotalOps

	if after2 <= after1 {
		t.Fatalf("totalOps should grow across restarts: first=%d second=%d", after1, after2)
	}
}
