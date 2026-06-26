package graph

// Edge is a directed connection from one node to another.
//
// An edge references two nodes by ID. In Phase 1 (single replica) we enforce
// the invariant eagerly: an edge can only be created if both endpoints exist,
// and deleting a node cascades to its edges. This keeps the single-replica graph
// always-consistent.
//
// That eager enforcement does NOT survive into the distributed phases. Once
// nodes and edges are independent OR-Sets, a concurrent "add edge" + "delete
// node" can leave a dangling edge (SYSTEM_DESIGN §13a). At that point the
// invariant moves from creation-time validation to read-time materialization:
// the edge OR-Set stays pure, and dangling edges are filtered out when the graph
// is materialized for rendering/hashing. Phase 1 establishes the happy path;
// Phase 3 makes it converge under concurrency.
type Edge struct {
	ID     string `json:"id"`
	Source string `json:"source"`
	Target string `json:"target"`
}
