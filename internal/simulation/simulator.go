// Simulator runs a pool of virtual users against a single replica (Phase 6).
//
// Each user fires random graph mutations (create/rename/move/delete) at a
// configured rate. The simulator is how we stress-test convergence without
// opening browser tabs: 100 users across 3 replicas with packet loss active,
// and the convergence checker (stateHash) is the oracle that proves every
// replica ends up identical after quiescence.
package simulation

import (
	"context"
	"fmt"
	"math/rand/v2"
	"sync"
	"sync/atomic"
	"time"

	"github.com/aryan-mishra/sentinel-sync/internal/replica"
)

// maxNodesPerUser caps how many nodes each virtual user will accumulate before
// it stops creating new ones and sticks to rename/move/delete.
const maxNodesPerUser = 20

// SimStats is returned by Stats() and embedded in /status.
type SimStats struct {
	Running   bool    `json:"running"`
	Users     int     `json:"users"`
	OpsPerSec float64 `json:"opsPerSec"`
	TotalOps  int64   `json:"totalOps"`
}

// Simulator manages a pool of virtual users that continuously mutate a replica.
// All exported methods are safe for concurrent use.
type Simulator struct {
	replica *replica.Replica

	mu        sync.Mutex
	cancel    context.CancelFunc
	wg        sync.WaitGroup // tracks live user goroutines
	users     int
	opsPerSec float64
	running   bool

	totalOps atomic.Int64
}

// NewSimulator returns a stopped simulator bound to r.
func NewSimulator(r *replica.Replica) *Simulator {
	return &Simulator{replica: r}
}

// Start launches n virtual users each firing at opsPerSec operations per second.
// If the simulator is already running, it is stopped first (waiting for all
// goroutines to drain) before the new run begins. This ensures no ops leak
// across runs when the caller immediately reads the oplog after Stop.
func (s *Simulator) Start(users int, opsPerSec float64) {
	if users <= 0 || opsPerSec <= 0 {
		return
	}
	s.Stop() // drain any previous run before starting a new one
	s.mu.Lock()
	s.users = users
	s.opsPerSec = opsPerSec
	ctx, cancel := context.WithCancel(context.Background())
	s.cancel = cancel
	s.running = true
	s.wg.Add(users)
	for i := range users {
		go func(i int) {
			defer s.wg.Done()
			s.runUser(ctx, fmt.Sprintf("u%d", i), opsPerSec)
		}(i)
	}
	s.mu.Unlock()
}

// Stop cancels all virtual users and waits for their goroutines to exit.
// After Stop returns, no more ops will be appended to the replica — safe to
// snapshot the oplog immediately. Safe to call when already stopped.
func (s *Simulator) Stop() {
	s.mu.Lock()
	if s.cancel != nil {
		s.cancel()
		s.cancel = nil
	}
	s.running = false
	s.mu.Unlock()
	s.wg.Wait() // block until every goroutine has returned
}

// Stats returns a point-in-time snapshot of simulator state.
func (s *Simulator) Stats() SimStats {
	s.mu.Lock()
	defer s.mu.Unlock()
	return SimStats{
		Running:   s.running,
		Users:     s.users,
		OpsPerSec: s.opsPerSec,
		TotalOps:  s.totalOps.Load(),
	}
}

// --- internals -------------------------------------------------------------

func (s *Simulator) runUser(ctx context.Context, id string, opsPerSec float64) {
	interval := time.Duration(float64(time.Second) / opsPerSec)
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	// Each user manages its own node ID namespace so creates never collide.
	var nodes []string
	counter := 0

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			s.doOp(id, &nodes, &counter)
		}
	}
}

// doOp picks and executes one random graph mutation. Errors are swallowed —
// a concurrent delete making a node disappear from our list is harmless; the
// failed op simply doesn't count toward totalOps.
func (s *Simulator) doOp(userID string, nodes *[]string, counter *int) {
	r := s.replica

	// Bias toward creates until the per-user cap is reached.
	if len(*nodes) == 0 || (len(*nodes) < maxNodesPerUser && rand.IntN(4) != 0) {
		*counter++
		id := fmt.Sprintf("%s-n%d", userID, *counter)
		title := fmt.Sprintf("%s-node-%d", userID, *counter)
		x, y := rand.Float64()*800, rand.Float64()*600
		if _, err := r.CreateNode(id, title, x, y); err == nil {
			*nodes = append(*nodes, id)
			s.totalOps.Add(1)
		}
		return
	}

	// Pick a random node owned by this user.
	idx := rand.IntN(len(*nodes))
	nodeID := (*nodes)[idx]

	switch rand.IntN(3) {
	case 0: // rename
		title := fmt.Sprintf("%s-r%d", userID, *counter)
		if _, err := r.RenameNode(nodeID, title); err == nil {
			s.totalOps.Add(1)
		}
	case 1: // move
		x, y := rand.Float64()*800, rand.Float64()*600
		if _, err := r.MoveNode(nodeID, x, y); err == nil {
			s.totalOps.Add(1)
		}
	case 2: // delete
		if _, err := r.DeleteNode(nodeID); err == nil {
			*nodes = append((*nodes)[:idx], (*nodes)[idx+1:]...)
			s.totalOps.Add(1)
		}
	}
}
