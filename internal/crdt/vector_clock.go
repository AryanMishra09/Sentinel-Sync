// Package crdt holds the conflict-free replicated data types and the supporting
// causal-metadata types (vector clocks, HLC, operations).
//
// Phase 2 scope: only the TYPE DEFINITIONS exist, so the Replica struct is
// stable and /status can display causal metadata. The actual merge behavior —
// VectorClock.Increment/Merge/Compare, OR-Set, the HLC-ordered LWW register —
// is built in Phase 3. Nothing in Phase 2 mutates these; the op log stays empty
// and every clock reads zero. See docs/IMPLEMENTATION_PLAN.md, Phase 3.
package crdt

import (
	"fmt"
	"maps"
	"sort"
	"strings"
)

// VectorClock maps a replica ID to the number of operations that replica has
// originated, as observed by the owner of this clock. Comparing two vector
// clocks tells us which operations a peer is missing (anti-entropy, Phase 4)
// and whether two operations are causal or concurrent (Phase 3).
//
// Note: vector clocks are NOT used to merge state — OR-Set tags and the HLC do
// that. See SYSTEM_DESIGN §11.
type VectorClock map[string]uint64

// New returns an empty clock pre-seeded with every known replica at 0, so a
// freshly started replica reports a complete, comparable clock immediately.
func New(replicaIDs ...string) VectorClock {
	vc := make(VectorClock, len(replicaIDs))
	for _, id := range replicaIDs {
		vc[id] = 0
	}
	return vc
}

// Snapshot returns a copy safe to hand to the JSON encoder without exposing the
// live map.
func (vc VectorClock) Snapshot() VectorClock {
	out := make(VectorClock, len(vc))
	maps.Copy(out, vc)
	return out
}

// Increment bumps this replica's own component by one. Called once per locally
// generated operation.
func (vc VectorClock) Increment(replicaID string) {
	vc[replicaID]++
}

// Merge takes the element-wise maximum of two clocks into the receiver. This is
// how a replica folds in everything a peer has seen.
func (vc VectorClock) Merge(other VectorClock) {
	for id, v := range other {
		if v > vc[id] {
			vc[id] = v
		}
	}
}

// Ordering is the causal relationship between two events as told by their vector
// clocks.
type Ordering int

const (
	Before     Ordering = iota // receiver happened strictly before other
	After                      // receiver happened strictly after other
	Equal                      // identical clocks
	Concurrent                 // neither dominates — concurrent events
)

// Compare returns the causal relationship of the receiver to other. This is used
// for anti-entropy diffs (Phase 4) and for labeling concurrent vs causal edits
// in the timeline UI — NOT for merging state (OR-Set tags and the HLC do that).
func (vc VectorClock) Compare(other VectorClock) Ordering {
	var less, greater bool
	seen := make(map[string]struct{}, len(vc)+len(other))
	for id := range vc {
		seen[id] = struct{}{}
	}
	for id := range other {
		seen[id] = struct{}{}
	}
	for id := range seen {
		a, b := vc[id], other[id]
		if a < b {
			less = true
		}
		if a > b {
			greater = true
		}
	}
	switch {
	case less && greater:
		return Concurrent
	case less:
		return Before
	case greater:
		return After
	default:
		return Equal
	}
}

// String renders the clock deterministically, e.g. "a=0,b=0,c=0".
func (vc VectorClock) String() string {
	keys := make([]string, 0, len(vc))
	for k := range vc {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	parts := make([]string, 0, len(keys))
	for _, k := range keys {
		parts = append(parts, fmt.Sprintf("%s=%d", k, vc[k]))
	}
	return strings.Join(parts, ",")
}
