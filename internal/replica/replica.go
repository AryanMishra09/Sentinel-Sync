// Package replica is one node in the SentinelSync cluster.
//
// Phase 2 scope: a Replica bundles a replica's identity, its peer list, its
// graph, and the (still-empty) causal-metadata scaffolding — a vector clock and
// an operation log. There is NO synchronization between replicas yet. Each
// process is an independent island that happens to know its peers' addresses.
// Phase 3 makes the graph mutations emit operations into the log and advance the
// clock; Phase 4 makes replicas exchange them.
package replica

import (
	"sync"

	"github.com/aryan-mishra/sentinel-sync/internal/crdt"
	"github.com/aryan-mishra/sentinel-sync/internal/graph"
)

// Peer is another replica this one knows about.
type Peer struct {
	ID      string `json:"id"`
	Address string `json:"address"`
}

// Replica is the per-process unit of the cluster. Every replica is equal — there
// is no leader and no coordinator (that is the whole point of a CRDT system).
type Replica struct {
	ID    string
	Peers []Peer

	Graph *graph.Graph

	// Causal-metadata scaffolding — defined now, exercised in Phase 3.
	mu    sync.RWMutex
	clock crdt.VectorClock
	oplog []crdt.Operation
}

// New builds a replica with an empty graph and a zeroed vector clock seeded with
// itself and all known peers.
func New(id string, peers []Peer) *Replica {
	ids := make([]string, 0, len(peers)+1)
	ids = append(ids, id)
	for _, p := range peers {
		ids = append(ids, p.ID)
	}
	return &Replica{
		ID:    id,
		Peers: peers,
		Graph: graph.New(),
		clock: crdt.New(ids...),
		oplog: make([]crdt.Operation, 0),
	}
}

// Clock returns a snapshot of this replica's vector clock (safe to serialize).
func (r *Replica) Clock() crdt.VectorClock {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.clock.Snapshot()
}

// OpLogLen reports how many operations this replica has recorded. Zero in
// Phase 2; grows once Phase 3 wires mutations to emit operations.
func (r *Replica) OpLogLen() int {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return len(r.oplog)
}
