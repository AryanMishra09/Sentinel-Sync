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
