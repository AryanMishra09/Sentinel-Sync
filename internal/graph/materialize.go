package graph

// Snapshot is the materialized, plain-data view of the graph returned to clients
// and hashed for convergence. It is fully resolved: OR-Sets collapsed to present
// elements, LWW registers collapsed to winning values, dangling edges removed.
type Snapshot struct {
	Nodes []*Node `json:"nodes"`
	Edges []*Edge `json:"edges"`
}

// Snapshot materializes the CRDT state.
//
// This is the read boundary where the edge referential-integrity invariant is
// enforced (SYSTEM_DESIGN §13a): the edge OR-Set stays pure, but an edge is only
// materialized if both its endpoints are present nodes. A node deleted
// concurrently with an edge-add simply makes that edge disappear from the view —
// and reappear automatically if the node is re-added.
//
// Both element lists come out sorted by ID (ORSet.Elements sorts), so the output
// is canonical and directly hashable.
func (s *State) Snapshot() Snapshot {
	s.mu.RLock()
	defer s.mu.RUnlock()

	nodeIDs := s.nodes.Elements()
	snap := Snapshot{
		Nodes: make([]*Node, 0, len(nodeIDs)),
		Edges: make([]*Edge, 0),
	}

	for _, id := range nodeIDs {
		n := &Node{ID: id, CreatedAt: s.createdAt[id]}
		if reg := s.titles[id]; reg != nil {
			n.Title = reg.Value
		}
		if reg := s.positions[id]; reg != nil {
			n.X, n.Y = reg.Value.X, reg.Value.Y
		}
		snap.Nodes = append(snap.Nodes, n.clone())
	}

	for _, id := range s.edges.Elements() {
		e := s.endpoints[id]
		// Dangling-edge filter: both endpoints must be live nodes.
		if s.nodes.Contains(e.Source) && s.nodes.Contains(e.Target) {
			snap.Edges = append(snap.Edges, (&Edge{ID: id, Source: e.Source, Target: e.Target}).clone())
		}
	}
	return snap
}

// Counts returns the number of materialized (visible) nodes and edges.
func (s *State) Counts() (nodes, edges int) {
	snap := s.Snapshot()
	return len(snap.Nodes), len(snap.Edges)
}

// Node returns a single materialized node, if present.
func (s *State) Node(id string) (*Node, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if !s.nodes.Contains(id) {
		return nil, false
	}
	n := &Node{ID: id, CreatedAt: s.createdAt[id]}
	if reg := s.titles[id]; reg != nil {
		n.Title = reg.Value
	}
	if reg := s.positions[id]; reg != nil {
		n.X, n.Y = reg.Value.X, reg.Value.Y
	}
	return n.clone(), true
}

// TombstoneCount reports total OR-Set tombstones (nodes + edges) — the
// growth metric from SYSTEM_DESIGN §30.
func (s *State) TombstoneCount() int {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.nodes.TombstoneCount() + s.edges.TombstoneCount()
}
