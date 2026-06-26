// Package graph implements the Phase 1 local workflow-graph engine.
//
// Phase 1 scope (see docs/IMPLEMENTATION_PLAN.md): a single replica, no
// networking, no CRDT, no convergence. Operations mutate the graph in place
// under a lock. This is the local state model that every later phase replicates
// and makes convergent.
package graph

import (
	"errors"
	"sync"
	"time"
)

// Sentinel errors returned by the engine. The API layer maps these to HTTP
// status codes; callers should compare with errors.Is.
var (
	ErrNodeNotFound   = errors.New("node not found")
	ErrNodeExists     = errors.New("node already exists")
	ErrEdgeNotFound   = errors.New("edge not found")
	ErrEdgeExists     = errors.New("edge already exists")
	ErrEndpointMissing = errors.New("edge endpoint node does not exist")
)

// Graph is the in-memory workflow graph for one replica.
//
// Concurrency: every public method takes the lock. We use a single sync.RWMutex
// because writes (Create/Delete/Rename/Move) mutate the maps and reads
// (Snapshot) must see a consistent view. Reads are genuinely read-only here —
// unlike SentinelCache's LRU Get, materializing the graph has no side effects —
// so RWMutex (many readers, one writer) is the right choice.
type Graph struct {
	mu    sync.RWMutex
	nodes map[string]*Node
	edges map[string]*Edge

	// now is injected for testability. Production uses time.Now; tests can
	// supply a deterministic clock. Phase 3 replaces this with an HLC.
	now func() time.Time
}

// New returns an empty graph.
func New() *Graph {
	return &Graph{
		nodes: make(map[string]*Node),
		edges: make(map[string]*Edge),
		now:   time.Now,
	}
}

// --- Node operations -------------------------------------------------------

// CreateNode adds a new node. The ID must be unique.
func (g *Graph) CreateNode(id, title string, x, y float64) (*Node, error) {
	g.mu.Lock()
	defer g.mu.Unlock()

	if _, ok := g.nodes[id]; ok {
		return nil, ErrNodeExists
	}
	n := &Node{
		ID:        id,
		Title:     title,
		X:         x,
		Y:         y,
		CreatedAt: g.now().UnixMilli(),
	}
	g.nodes[id] = n
	return n.clone(), nil
}

// RenameNode sets a node's title. In Phase 3 this becomes an LWW-register write.
func (g *Graph) RenameNode(id, title string) (*Node, error) {
	g.mu.Lock()
	defer g.mu.Unlock()

	n, ok := g.nodes[id]
	if !ok {
		return nil, ErrNodeNotFound
	}
	n.Title = title
	return n.clone(), nil
}

// MoveNode sets a node's position. In Phase 3 this becomes an LWW-register write
// (and is lossy under concurrency by design — see SYSTEM_DESIGN §18).
func (g *Graph) MoveNode(id string, x, y float64) (*Node, error) {
	g.mu.Lock()
	defer g.mu.Unlock()

	n, ok := g.nodes[id]
	if !ok {
		return nil, ErrNodeNotFound
	}
	n.X, n.Y = x, y
	return n.clone(), nil
}

// DeleteNode removes a node and cascades to every edge touching it.
//
// The cascade is the single-replica shortcut. Under CRDTs this exact scenario
// (delete node while a concurrent op adds an edge to it) is the dangling-edge
// problem, solved later at materialization rather than by cascading deletes.
func (g *Graph) DeleteNode(id string) error {
	g.mu.Lock()
	defer g.mu.Unlock()

	if _, ok := g.nodes[id]; !ok {
		return ErrNodeNotFound
	}
	delete(g.nodes, id)

	for eid, e := range g.edges {
		if e.Source == id || e.Target == id {
			delete(g.edges, eid)
		}
	}
	return nil
}

// --- Edge operations -------------------------------------------------------

// CreateEdge connects two existing nodes. Both endpoints must exist (Phase 1
// eager invariant; see edge.go for how this changes under CRDTs).
func (g *Graph) CreateEdge(id, source, target string) (*Edge, error) {
	g.mu.Lock()
	defer g.mu.Unlock()

	if _, ok := g.edges[id]; ok {
		return nil, ErrEdgeExists
	}
	if _, ok := g.nodes[source]; !ok {
		return nil, ErrEndpointMissing
	}
	if _, ok := g.nodes[target]; !ok {
		return nil, ErrEndpointMissing
	}
	e := &Edge{ID: id, Source: source, Target: target}
	g.edges[id] = e
	return e.clone(), nil
}

// DeleteEdge removes an edge by ID.
func (g *Graph) DeleteEdge(id string) error {
	g.mu.Lock()
	defer g.mu.Unlock()

	if _, ok := g.edges[id]; !ok {
		return ErrEdgeNotFound
	}
	delete(g.edges, id)
	return nil
}

// --- Read ------------------------------------------------------------------

// Snapshot is the immutable view of a Snapshot returned to the API.
type Snapshot struct {
	Nodes []*Node `json:"nodes"`
	Edges []*Edge `json:"edges"`
}

// Snapshot returns a deep copy of the current graph. Copies (not the live
// pointers) are returned so callers can never mutate engine state without going
// through a locked method. This is the Phase 1 stand-in for what becomes the
// "materialize" step in Phase 3.
func (g *Graph) Snapshot() Snapshot {
	g.mu.RLock()
	defer g.mu.RUnlock()

	s := Snapshot{
		Nodes: make([]*Node, 0, len(g.nodes)),
		Edges: make([]*Edge, 0, len(g.edges)),
	}
	for _, n := range g.nodes {
		s.Nodes = append(s.Nodes, n.clone())
	}
	for _, e := range g.edges {
		s.Edges = append(s.Edges, e.clone())
	}
	return s
}

// Counts returns node and edge counts. Cheap status endpoint for the dashboard.
func (g *Graph) Counts() (nodes, edges int) {
	g.mu.RLock()
	defer g.mu.RUnlock()
	return len(g.nodes), len(g.edges)
}

// --- helpers ---------------------------------------------------------------

func (n *Node) clone() *Node {
	c := *n
	return &c
}

func (e *Edge) clone() *Edge {
	c := *e
	return &c
}
