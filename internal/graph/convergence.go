package graph

import (
	"crypto/sha256"
	"encoding/hex"
	"strconv"
)

// Hash returns a content hash of the materialized graph — the convergence oracle
// (SYSTEM_DESIGN §29a). Two replicas that have applied the same set of operations
// (in any order) produce the same hash; during a partition their hashes differ.
//
// The hash is taken over the canonical Snapshot (sorted nodes/edges, dangling
// edges already filtered). CreatedAt is deliberately excluded: it is display
// metadata, not part of logical state, and including it would couple the hash to
// non-convergent bookkeeping.
func (s *State) Hash() string {
	snap := s.Snapshot()

	h := sha256.New()
	for _, n := range snap.Nodes {
		writeField(h, "N")
		writeField(h, n.ID)
		writeField(h, n.Title)
		writeField(h, strconv.FormatFloat(n.X, 'f', -1, 64))
		writeField(h, strconv.FormatFloat(n.Y, 'f', -1, 64))
		h.Write([]byte{'\n'})
	}
	for _, e := range snap.Edges {
		writeField(h, "E")
		writeField(h, e.ID)
		writeField(h, e.Source)
		writeField(h, e.Target)
		h.Write([]byte{'\n'})
	}
	return hex.EncodeToString(h.Sum(nil))
}

// writeField writes a length-prefixed field so that, e.g., titles "ab"+"c" and
// "a"+"bc" can never collide into the same byte stream.
func writeField(h interface{ Write([]byte) (int, error) }, s string) {
	h.Write([]byte(strconv.Itoa(len(s))))
	h.Write([]byte{':'})
	h.Write([]byte(s))
	h.Write([]byte{'|'})
}
