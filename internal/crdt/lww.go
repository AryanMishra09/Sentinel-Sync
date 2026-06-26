package crdt

// LWWRegister is a Last-Write-Wins register ordered by Hybrid Logical Clock.
// It holds a single value whose conflicts resolve deterministically: the write
// with the greater HLC timestamp wins, with ReplicaID as the final tiebreak
// (see HLCTimestamp.After).
//
// Used for node Title and Position (SYSTEM_DESIGN §17–18). This is the *scoped*
// LWW the blueprint endorses — a single field that genuinely cannot keep two
// concurrent values — not the document-level LWW that loses data and was
// rejected.
type LWWRegister[T any] struct {
	Value T            `json:"value"`
	TS    HLCTimestamp `json:"ts"`
}

// Set applies a write. It takes effect only if ts strictly outranks the current
// timestamp, so applying the same writes in any order converges to the same
// value — the property the convergence checker verifies. Returns true if the
// value changed.
func (r *LWWRegister[T]) Set(value T, ts HLCTimestamp) bool {
	if ts.After(r.TS) {
		r.Value = value
		r.TS = ts
		return true
	}
	return false
}
