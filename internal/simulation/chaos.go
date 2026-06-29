// Package simulation is the network fault injector (Phase 5).
//
// A Chaos value sits between the replica and its transport. Three independent
// knobs control outgoing behaviour:
//
//   - Latency: sleep before each outgoing message (simulates a slow WAN link).
//   - LossRate: drop each outgoing message with probability p (simulates packet
//     loss on the replica's outbound link).
//   - Isolated: block ALL outgoing sends AND drop ALL incoming messages (simulates
//     a full network partition — the node is alive but cut off).
//
// Loss and latency affect only outgoing traffic; isolation is bidirectional.
// When isolation is lifted the next anti-entropy tick carries the replica back
// to full convergence with no special recovery code.
package simulation

import (
	"math/rand/v2"
	"sync"
	"time"
)

// Chaos is the per-replica network fault injector. All methods are safe for
// concurrent use from the transport's broadcast and read goroutines.
type Chaos struct {
	mu       sync.RWMutex
	latency  time.Duration
	lossRate float64 // [0.0, 1.0]
	isolated bool
}

// NewChaos returns a no-op Chaos (all faults disabled).
func NewChaos() *Chaos { return &Chaos{} }

// SetLatency configures outgoing message delay. Zero disables latency.
func (c *Chaos) SetLatency(d time.Duration) {
	c.mu.Lock()
	c.latency = d
	c.mu.Unlock()
}

// SetLossRate sets the probability [0.0–1.0] that each outgoing message is
// silently dropped. 0.0 = no loss; 1.0 = total loss.
func (c *Chaos) SetLossRate(r float64) {
	c.mu.Lock()
	c.lossRate = r
	c.mu.Unlock()
}

// SetIsolated enables or clears the soft-partition flag. While isolated the
// replica sends nothing and discards all incoming messages (both directions
// blocked). Lifting isolation triggers recovery on the next anti-entropy tick.
func (c *Chaos) SetIsolated(v bool) {
	c.mu.Lock()
	c.isolated = v
	c.mu.Unlock()
}

// ShouldDrop returns true if an outgoing message should be dropped.
// Isolation always drops; packet loss drops probabilistically.
func (c *Chaos) ShouldDrop() bool {
	c.mu.RLock()
	defer c.mu.RUnlock()
	if c.isolated {
		return true
	}
	return c.lossRate > 0 && rand.Float64() < c.lossRate
}

// ApplyDelay sleeps for the configured latency. Called by the transport before
// each outgoing write so the replica's send goroutine blocks, not the caller.
func (c *Chaos) ApplyDelay() {
	c.mu.RLock()
	lat := c.latency
	c.mu.RUnlock()
	if lat > 0 {
		time.Sleep(lat)
	}
}

// IsIsolated reports whether the replica is soft-partitioned. The transport's
// read loop checks this to discard incoming messages while isolated.
func (c *Chaos) IsIsolated() bool {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.isolated
}

// Snapshot captures current fault settings for the /status endpoint.
type Snapshot struct {
	LatencyMs int64   `json:"latencyMs"`
	LossRate  float64 `json:"lossRate"`
	Isolated  bool    `json:"isolated"`
}

// Snapshot returns a read-only view of the current settings.
func (c *Chaos) Snapshot() Snapshot {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return Snapshot{
		LatencyMs: int64(c.latency.Milliseconds()),
		LossRate:  c.lossRate,
		Isolated:  c.isolated,
	}
}
