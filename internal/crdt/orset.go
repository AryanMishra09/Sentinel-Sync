package crdt

import "sort"

// Tag uniquely identifies a single "add" of an element to an OR-Set. Because the
// pair (ReplicaID, Counter) is globally unique, two replicas adding the same
// element produce distinct tags — which is exactly what lets the set survive an
// add concurrent with a remove.
type Tag struct {
	ReplicaID string `json:"replicaId"`
	Counter   uint64 `json:"counter"`
}

// ORSet is an Observed-Remove Set with add-wins semantics, used for node and
// edge *presence* (SYSTEM_DESIGN §13).
//
// Each add records a unique tag. A remove records the tags it *observed* at the
// time as tombstones. An element is present if it has at least one add tag that
// has not been tombstoned. Therefore an add concurrent with a remove (the add's
// tag is new and unobserved by the remove) keeps the element alive — preventing
// the "zombie resurrection" bug of naive sets.
//
// Known limitation (SYSTEM_DESIGN §13): tombstones accumulate forever. There is
// no causal-stability GC in V1; that is a V2 enhancement, surfaced meanwhile as a
// metric.
type ORSet struct {
	adds    map[string]map[Tag]struct{}
	removes map[string]map[Tag]struct{}
}

// NewORSet returns an empty set.
func NewORSet() *ORSet {
	return &ORSet{
		adds:    make(map[string]map[Tag]struct{}),
		removes: make(map[string]map[Tag]struct{}),
	}
}

// AddTag records an add of elem under tag.
func (s *ORSet) AddTag(elem string, tag Tag) {
	if s.adds[elem] == nil {
		s.adds[elem] = make(map[Tag]struct{})
	}
	s.adds[elem][tag] = struct{}{}
}

// ObservedTags returns the currently-live add tags for elem (adds minus
// tombstones). A remove operation carries exactly this set so that only the
// observed adds are tombstoned — concurrent adds are spared.
func (s *ORSet) ObservedTags(elem string) []Tag {
	var out []Tag
	for tag := range s.adds[elem] {
		if _, removed := s.removes[elem][tag]; !removed {
			out = append(out, tag)
		}
	}
	return out
}

// RemoveTags tombstones the given tags for elem.
func (s *ORSet) RemoveTags(elem string, tags []Tag) {
	if len(tags) == 0 {
		return
	}
	if s.removes[elem] == nil {
		s.removes[elem] = make(map[Tag]struct{})
	}
	for _, tag := range tags {
		s.removes[elem][tag] = struct{}{}
	}
}

// Contains reports whether elem is present: it has at least one add tag that is
// not tombstoned.
func (s *ORSet) Contains(elem string) bool {
	for tag := range s.adds[elem] {
		if _, removed := s.removes[elem][tag]; !removed {
			return true
		}
	}
	return false
}

// Elements returns all present elements, sorted for deterministic iteration
// (the convergence hash depends on stable ordering).
func (s *ORSet) Elements() []string {
	var out []string
	for elem := range s.adds {
		if s.Contains(elem) {
			out = append(out, elem)
		}
	}
	sort.Strings(out)
	return out
}

// Merge folds another set into this one: union of adds and union of removes.
// This is the state-based merge used by anti-entropy (Phase 4). Because both
// components are sets and union is commutative/associative/idempotent, merge
// order never matters — the defining property of a CRDT.
func (s *ORSet) Merge(other *ORSet) {
	for elem, tags := range other.adds {
		for tag := range tags {
			s.AddTag(elem, tag)
		}
	}
	for elem, tags := range other.removes {
		for tag := range tags {
			if s.removes[elem] == nil {
				s.removes[elem] = make(map[Tag]struct{})
			}
			s.removes[elem][tag] = struct{}{}
		}
	}
}

// TombstoneCount returns the total number of tombstoned tags across all
// elements. Exposed for the growth metric (SYSTEM_DESIGN §30).
func (s *ORSet) TombstoneCount() int {
	n := 0
	for _, tags := range s.removes {
		n += len(tags)
	}
	return n
}
