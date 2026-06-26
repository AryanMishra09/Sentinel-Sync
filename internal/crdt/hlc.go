package crdt

import (
	"sync"
	"time"
)

// HLC is a Hybrid Logical Clock. It produces timestamps that track wall time
// closely (good for display and human reasoning) while remaining causally
// consistent: a cause never out-ranks its effect on a replica that has observed
// it, regardless of physical clock skew between machines.
//
// This is the corrected design from SYSTEM_DESIGN §17 — LWW registers (title,
// position) are ordered by HLC, never a raw wall clock. A raw clock would let the
// fastest machine win every concurrent rename.
type HLC struct {
	mu        sync.Mutex
	replicaID string
	physical  int64 // last observed/issued physical time (millis)
	logical   int64 // tiebreaker within the same physical millisecond
	now       func() int64
}

// NewHLC returns a clock for replicaID. now supplies wall-clock millis; pass nil
// for the real clock (tests inject a deterministic one).
func NewHLC(replicaID string, now func() int64) *HLC {
	if now == nil {
		now = func() int64 { return time.Now().UnixMilli() }
	}
	return &HLC{replicaID: replicaID, now: now}
}

// Now issues a timestamp for a locally generated event.
func (h *HLC) Now() HLCTimestamp {
	h.mu.Lock()
	defer h.mu.Unlock()

	pt := h.now()
	if pt > h.physical {
		h.physical = pt
		h.logical = 0
	} else {
		// Wall clock didn't advance (or went backward) — bump the logical part
		// so timestamps stay strictly increasing on this replica.
		h.logical++
	}
	return HLCTimestamp{Physical: h.physical, Logical: h.logical, ReplicaID: h.replicaID}
}

// Update advances this clock on receipt of a remote timestamp and returns a
// fresh local timestamp that dominates both. This is the standard HLC receive
// rule; it guarantees the local clock never trails a peer it has heard from.
func (h *HLC) Update(remote HLCTimestamp) HLCTimestamp {
	h.mu.Lock()
	defer h.mu.Unlock()

	pt := h.now()
	oldP := h.physical
	newP := max3(oldP, remote.Physical, pt)

	switch {
	case newP == oldP && newP == remote.Physical:
		h.logical = maxI64(h.logical, remote.Logical) + 1
	case newP == oldP:
		h.logical++
	case newP == remote.Physical:
		h.logical = remote.Logical + 1
	default:
		h.logical = 0
	}
	h.physical = newP
	return HLCTimestamp{Physical: h.physical, Logical: h.logical, ReplicaID: h.replicaID}
}

// After reports whether a strictly outranks b in (physical, logical, replicaID)
// order. This total order is what makes the LWW register deterministic — every
// replica resolves the same winner, with ReplicaID as the final tiebreak.
func (a HLCTimestamp) After(b HLCTimestamp) bool {
	if a.Physical != b.Physical {
		return a.Physical > b.Physical
	}
	if a.Logical != b.Logical {
		return a.Logical > b.Logical
	}
	return a.ReplicaID > b.ReplicaID
}

func max3(a, b, c int64) int64 { return maxI64(maxI64(a, b), c) }

func maxI64(a, b int64) int64 {
	if a > b {
		return a
	}
	return b
}
