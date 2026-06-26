package graph

// Node is a single vertex in the workflow graph.
//
// In Phase 1 this is a plain struct mutated in place. From Phase 3 onward the
// mutable fields split into CRDT types: presence (the node existing at all)
// becomes an OR-Set membership, while Title and Position become HLC-ordered LWW
// registers. The field layout here deliberately anticipates that split so the
// later refactor is mechanical, not structural.
type Node struct {
	ID    string `json:"id"`
	Title string `json:"title"`

	// Position. In Phase 3 (X, Y) become a single LWW register so two
	// concurrent drags resolve deterministically to one position.
	X float64 `json:"x"`
	Y float64 `json:"y"`

	// CreatedAt is wall-clock millis. It is metadata only — it is NEVER used
	// for conflict resolution (that is what the HLC is for, see SYSTEM_DESIGN
	// §17). Using it to break a rename tie would reintroduce the clock-skew bug.
	CreatedAt int64 `json:"createdAt"`
}

func (n *Node) clone() *Node {
	c := *n
	return &c
}
