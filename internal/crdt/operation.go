package crdt

import "encoding/json"

// OpType enumerates the mutations that become replicated operations in Phase 3.
// They map 1:1 onto the Phase 1 graph methods and the REST routes.
type OpType string

const (
	OpCreateNode OpType = "create_node"
	OpRenameNode OpType = "rename_node"
	OpMoveNode   OpType = "move_node"
	OpDeleteNode OpType = "delete_node"
	OpCreateEdge OpType = "create_edge"
	OpDeleteEdge OpType = "delete_edge"
)

// HLCTimestamp is a Hybrid Logical Clock value. It orders LWW-register writes
// (title, position) WITHOUT depending on synchronized wall clocks: a replica
// advances its HLC to max(local, remote)+ on receive, so a cause never
// out-ranks its effect, and ReplicaID is the final deterministic tiebreak.
//
// Phase 2: definition only. The Now/Update/Compare logic lands in Phase 3
// (crdt/hlc.go).
type HLCTimestamp struct {
	Physical  int64  `json:"physical"`
	Logical   int64  `json:"logical"`
	ReplicaID string `json:"replicaId"`
}

// Operation is a single replicated mutation. Every change a replica makes
// becomes one of these, appended to its operation log and (Phase 4) broadcast
// to peers.
//
// Phase 2: the type is defined and the log field exists on Replica, but no
// operations are generated yet — the log stays empty until Phase 3 wires the
// graph methods to emit them.
type Operation struct {
	ID        string          `json:"id"`        // unique → exactly-once application
	ReplicaID string          `json:"replicaId"` // origin replica
	Type      OpType          `json:"type"`
	Payload   json.RawMessage `json:"payload"`

	// Causal metadata. VectorClock drives anti-entropy + concurrency detection
	// (NOT merge); HLC drives LWW ordering. See SYSTEM_DESIGN §9.
	VectorClock VectorClock  `json:"vectorClock"`
	HLC         HLCTimestamp `json:"hlc"`
}
