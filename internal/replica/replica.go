// Package replica is one node in the SentinelSync cluster.
//
// Phase 3: the replica is the operation engine. Every mutation becomes an
// Operation that advances the vector clock and HLC, is appended to the log, and
// is applied to the CRDT graph state. Operations from peers arrive via Ingest
// (the same apply path, minus local clock generation). There is still no
// networking — Phase 4 adds the transport that calls Ingest on peers.
package replica

import (
	"encoding/json"
	"fmt"
	"sync"

	"github.com/aryan-mishra/sentinel-sync/internal/crdt"
	"github.com/aryan-mishra/sentinel-sync/internal/graph"
)

// mustMarshal serializes an operation payload. The payload types are fixed
// structs that always marshal cleanly, so a failure is a programming error.
func mustMarshal(v any) json.RawMessage {
	b, err := json.Marshal(v)
	if err != nil {
		panic(fmt.Sprintf("marshal payload: %v", err))
	}
	return b
}

// Peer is another replica this one knows about.
type Peer struct {
	ID      string `json:"id"`
	Address string `json:"address"`
}

// Replica is the per-process unit of the cluster. Every replica is equal — no
// leader, no coordinator.
type Replica struct {
	ID    string
	Peers []Peer

	state *graph.State
	hlc   *crdt.HLC

	mu      sync.Mutex // guards clock, oplog, applied
	clock   crdt.VectorClock
	oplog   []crdt.Operation
	applied map[string]bool
}

// New builds a replica with an empty CRDT graph and a zeroed vector clock seeded
// with itself and all known peers. now supplies wall-clock millis for the HLC;
// pass nil for the real clock.
func New(id string, peers []Peer, now func() int64) *Replica {
	ids := make([]string, 0, len(peers)+1)
	ids = append(ids, id)
	for _, p := range peers {
		ids = append(ids, p.ID)
	}
	return &Replica{
		ID:      id,
		Peers:   peers,
		state:   graph.NewState(),
		hlc:     crdt.NewHLC(id, now),
		clock:   crdt.New(ids...),
		applied: make(map[string]bool),
	}
}

// --- Operation generation (local mutations) --------------------------------

// emit builds, records, and applies a locally generated operation. build
// receives the unique OR-Set tag for this operation (add operations embed it;
// others ignore it).
func (r *Replica) emit(typ crdt.OpType, build func(crdt.Tag) any) crdt.Operation {
	r.mu.Lock()
	defer r.mu.Unlock()

	r.clock.Increment(r.ID)
	counter := r.clock[r.ID]
	ts := r.hlc.Now()
	tag := crdt.Tag{ReplicaID: r.ID, Counter: counter}

	payload := mustMarshal(build(tag))
	op := crdt.Operation{
		ID:          fmt.Sprintf("%s-%d", r.ID, counter),
		ReplicaID:   r.ID,
		Type:        typ,
		Payload:     payload,
		VectorClock: r.clock.Snapshot(),
		HLC:         ts,
	}
	r.applied[op.ID] = true
	r.oplog = append(r.oplog, op)
	_ = r.state.Apply(op) // local payloads are always well-formed
	return op
}

// CreateNode adds a node. Errors locally if the node already exists.
func (r *Replica) CreateNode(id, title string, x, y float64) (crdt.Operation, error) {
	if r.state.HasNode(id) {
		return crdt.Operation{}, graph.ErrNodeExists
	}
	return r.emit(crdt.OpCreateNode, func(tag crdt.Tag) any {
		return graph.CreateNodePayload{ID: id, Title: title, X: x, Y: y, Tag: tag}
	}), nil
}

// RenameNode sets a node's title (LWW). Errors locally if missing.
func (r *Replica) RenameNode(id, title string) (crdt.Operation, error) {
	if !r.state.HasNode(id) {
		return crdt.Operation{}, graph.ErrNodeNotFound
	}
	return r.emit(crdt.OpRenameNode, func(crdt.Tag) any {
		return graph.RenameNodePayload{ID: id, Title: title}
	}), nil
}

// MoveNode sets a node's position (LWW). Errors locally if missing.
func (r *Replica) MoveNode(id string, x, y float64) (crdt.Operation, error) {
	if !r.state.HasNode(id) {
		return crdt.Operation{}, graph.ErrNodeNotFound
	}
	return r.emit(crdt.OpMoveNode, func(crdt.Tag) any {
		return graph.MoveNodePayload{ID: id, X: x, Y: y}
	}), nil
}

// DeleteNode removes a node (add-wins OR-Set remove over observed tags). Errors
// locally if missing. Edges to this node are NOT cascaded — they become dangling
// and are filtered at materialization (SYSTEM_DESIGN §13a).
func (r *Replica) DeleteNode(id string) (crdt.Operation, error) {
	if !r.state.HasNode(id) {
		return crdt.Operation{}, graph.ErrNodeNotFound
	}
	tags := r.state.ObservedNodeTags(id)
	return r.emit(crdt.OpDeleteNode, func(crdt.Tag) any {
		return graph.DeleteNodePayload{ID: id, RemovedTags: tags}
	}), nil
}

// CreateEdge adds an edge. No endpoint validation — a dangling edge is allowed
// and filtered at materialization. Errors locally only if the edge id is taken.
func (r *Replica) CreateEdge(id, source, target string) (crdt.Operation, error) {
	if r.state.HasEdge(id) {
		return crdt.Operation{}, graph.ErrEdgeExists
	}
	return r.emit(crdt.OpCreateEdge, func(tag crdt.Tag) any {
		return graph.CreateEdgePayload{ID: id, Source: source, Target: target, Tag: tag}
	}), nil
}

// DeleteEdge removes an edge. Errors locally if missing.
func (r *Replica) DeleteEdge(id string) (crdt.Operation, error) {
	if !r.state.HasEdge(id) {
		return crdt.Operation{}, graph.ErrEdgeNotFound
	}
	tags := r.state.ObservedEdgeTags(id)
	return r.emit(crdt.OpDeleteEdge, func(crdt.Tag) any {
		return graph.DeleteEdgePayload{ID: id, RemovedTags: tags}
	}), nil
}

// --- Operation ingestion (from peers / replay) -----------------------------

// Ingest applies an operation that originated elsewhere. It is idempotent
// (deduplicated by operation ID), merges the operation's vector clock, and
// advances the HLC. This is the path Phase 4's replication and anti-entropy use.
func (r *Replica) Ingest(op crdt.Operation) {
	r.mu.Lock()
	defer r.mu.Unlock()

	if r.applied[op.ID] {
		return
	}
	r.applied[op.ID] = true
	r.clock.Merge(op.VectorClock)
	r.hlc.Update(op.HLC)
	r.oplog = append(r.oplog, op)
	_ = r.state.Apply(op)
}

// --- Reads -----------------------------------------------------------------

// Snapshot returns the materialized graph.
func (r *Replica) Snapshot() graph.Snapshot { return r.state.Snapshot() }

// Node returns a single materialized node, if present.
func (r *Replica) Node(id string) (*graph.Node, bool) { return r.state.Node(id) }

// Counts returns visible node/edge counts.
func (r *Replica) Counts() (nodes, edges int) { return r.state.Counts() }

// Hash returns the convergence hash of the materialized state.
func (r *Replica) Hash() string { return r.state.Hash() }

// TombstoneCount returns total OR-Set tombstones (growth metric).
func (r *Replica) TombstoneCount() int { return r.state.TombstoneCount() }

// Clock returns a snapshot of this replica's vector clock.
func (r *Replica) Clock() crdt.VectorClock {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.clock.Snapshot()
}

// OpLogLen reports how many operations this replica has recorded.
func (r *Replica) OpLogLen() int {
	r.mu.Lock()
	defer r.mu.Unlock()
	return len(r.oplog)
}

// OpLog returns a copy of the operation log (used by tests and, in Phase 4, by
// anti-entropy to ship missing operations to peers).
func (r *Replica) OpLog() []crdt.Operation {
	r.mu.Lock()
	defer r.mu.Unlock()
	out := make([]crdt.Operation, len(r.oplog))
	copy(out, r.oplog)
	return out
}
