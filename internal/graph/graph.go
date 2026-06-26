// Package graph holds the workflow-graph state and its materialized view.
//
// Phase 3 turns the state into a CRDT. Presence of nodes and edges is an OR-Set;
// each node's Title and Position is an HLC-ordered LWW register. State mutates
// ONLY by applying operations (State.Apply), so the same operations applied in
// any order on any replica converge to identical materialized state — verified
// by the convergence hash (convergence.go).
//
// node.go / edge.go define the *materialized* (plain) types returned to clients;
// materialize.go resolves the CRDTs into them (filtering dangling edges);
// convergence.go hashes that materialized view.
package graph

import (
	"encoding/json"
	"errors"
	"sync"

	"github.com/aryan-mishra/sentinel-sync/internal/crdt"
)

// Sentinel errors for the local-convenience checks the API performs before
// generating an operation. They never fire on the merge path (Apply) — a replica
// must accept any operation a peer sends.
var (
	ErrNodeNotFound = errors.New("node not found")
	ErrNodeExists   = errors.New("node already exists")
	ErrEdgeNotFound = errors.New("edge not found")
	ErrEdgeExists   = errors.New("edge already exists")
)

// Position is a node's coordinate, stored as a single LWW register so concurrent
// moves resolve to one position deterministically.
type Position struct {
	X float64 `json:"x"`
	Y float64 `json:"y"`
}

// State is the CRDT-backed graph for one replica.
type State struct {
	mu        sync.RWMutex
	nodes     *crdt.ORSet // node presence
	edges     *crdt.ORSet // edge presence
	titles    map[string]*crdt.LWWRegister[string]
	positions map[string]*crdt.LWWRegister[Position]
	endpoints map[string]endpoint // edgeID -> source/target (immutable per edge)
	createdAt map[string]int64    // nodeID -> first-create physical time (metadata)
}

type endpoint struct {
	Source string
	Target string
}

// NewState returns an empty CRDT graph.
func NewState() *State {
	return &State{
		nodes:     crdt.NewORSet(),
		edges:     crdt.NewORSet(),
		titles:    make(map[string]*crdt.LWWRegister[string]),
		positions: make(map[string]*crdt.LWWRegister[Position]),
		endpoints: make(map[string]endpoint),
		createdAt: make(map[string]int64),
	}
}

// --- Operation payloads ----------------------------------------------------
// These are marshaled into Operation.Payload by the replica and unmarshaled by
// Apply. They live in this package because Apply is what interprets them; crdt
// only carries the raw bytes (so crdt never imports graph).

// CreateNodePayload carries a node creation, including its unique OR-Set tag and
// the initial title/position (applied at the operation's HLC).
type CreateNodePayload struct {
	ID    string    `json:"id"`
	Title string    `json:"title"`
	X     float64   `json:"x"`
	Y     float64   `json:"y"`
	Tag   crdt.Tag  `json:"tag"`
}

// RenameNodePayload carries a title write.
type RenameNodePayload struct {
	ID    string `json:"id"`
	Title string `json:"title"`
}

// MoveNodePayload carries a position write.
type MoveNodePayload struct {
	ID string  `json:"id"`
	X  float64 `json:"x"`
	Y  float64 `json:"y"`
}

// DeleteNodePayload carries the observed add-tags to tombstone (add-wins remove).
type DeleteNodePayload struct {
	ID          string     `json:"id"`
	RemovedTags []crdt.Tag `json:"removedTags"`
}

// CreateEdgePayload carries an edge creation. No endpoint validation — a dangling
// edge is allowed and filtered at materialization (SYSTEM_DESIGN §13a).
type CreateEdgePayload struct {
	ID     string   `json:"id"`
	Source string   `json:"source"`
	Target string   `json:"target"`
	Tag    crdt.Tag `json:"tag"`
}

// DeleteEdgePayload carries the observed add-tags to tombstone.
type DeleteEdgePayload struct {
	ID          string     `json:"id"`
	RemovedTags []crdt.Tag `json:"removedTags"`
}

// --- Apply -----------------------------------------------------------------

// Apply mutates the state by an operation. It is the ONLY way state changes, and
// it is deterministic and order-independent: OR-Set adds/removes are set unions,
// LWW writes keep the max-HLC value. op.HLC is the timestamp for any LWW write in
// this operation.
func (s *State) Apply(op crdt.Operation) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	switch op.Type {
	case crdt.OpCreateNode:
		var p CreateNodePayload
		if err := json.Unmarshal(op.Payload, &p); err != nil {
			return err
		}
		s.nodes.AddTag(p.ID, p.Tag)
		s.ensureNode(p.ID)
		s.titles[p.ID].Set(p.Title, op.HLC)
		s.positions[p.ID].Set(Position{X: p.X, Y: p.Y}, op.HLC)
		if cur, ok := s.createdAt[p.ID]; !ok || op.HLC.Physical < cur {
			s.createdAt[p.ID] = op.HLC.Physical
		}

	case crdt.OpRenameNode:
		var p RenameNodePayload
		if err := json.Unmarshal(op.Payload, &p); err != nil {
			return err
		}
		s.ensureNode(p.ID)
		s.titles[p.ID].Set(p.Title, op.HLC)

	case crdt.OpMoveNode:
		var p MoveNodePayload
		if err := json.Unmarshal(op.Payload, &p); err != nil {
			return err
		}
		s.ensureNode(p.ID)
		s.positions[p.ID].Set(Position{X: p.X, Y: p.Y}, op.HLC)

	case crdt.OpDeleteNode:
		var p DeleteNodePayload
		if err := json.Unmarshal(op.Payload, &p); err != nil {
			return err
		}
		s.nodes.RemoveTags(p.ID, p.RemovedTags)

	case crdt.OpCreateEdge:
		var p CreateEdgePayload
		if err := json.Unmarshal(op.Payload, &p); err != nil {
			return err
		}
		s.edges.AddTag(p.ID, p.Tag)
		if _, ok := s.endpoints[p.ID]; !ok {
			s.endpoints[p.ID] = endpoint{Source: p.Source, Target: p.Target}
		}

	case crdt.OpDeleteEdge:
		var p DeleteEdgePayload
		if err := json.Unmarshal(op.Payload, &p); err != nil {
			return err
		}
		s.edges.RemoveTags(p.ID, p.RemovedTags)

	default:
		return errors.New("unknown operation type: " + string(op.Type))
	}
	return nil
}

// ensureNode lazily creates the LWW registers for a node id. Caller holds the lock.
func (s *State) ensureNode(id string) {
	if s.titles[id] == nil {
		s.titles[id] = &crdt.LWWRegister[string]{}
	}
	if s.positions[id] == nil {
		s.positions[id] = &crdt.LWWRegister[Position]{}
	}
}

// --- Local-convenience reads (used by the API before emitting ops) ---------

// HasNode reports whether a node is currently present.
func (s *State) HasNode(id string) bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.nodes.Contains(id)
}

// HasEdge reports whether an edge is currently present (in the OR-Set; may still
// be dangling once materialized).
func (s *State) HasEdge(id string) bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.edges.Contains(id)
}

// ObservedNodeTags returns the live add-tags for a node, for a delete operation.
func (s *State) ObservedNodeTags(id string) []crdt.Tag {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.nodes.ObservedTags(id)
}

// ObservedEdgeTags returns the live add-tags for an edge, for a delete operation.
func (s *State) ObservedEdgeTags(id string) []crdt.Tag {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.edges.ObservedTags(id)
}
