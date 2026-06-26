package transport

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/aryan-mishra/sentinel-sync/internal/replica"
)

// node wires a replica + manager behind an httptest server exposing /ws.
type node struct {
	rep    *replica.Replica
	mgr    *Manager
	server *httptest.Server
	cancel context.CancelFunc
}

func startNode(id string, base int64) *node {
	clock := int64(base)
	rep := replica.New(id, nil, func() int64 { clock++; return clock })
	mgr := NewManager(rep)

	mux := http.NewServeMux()
	mux.HandleFunc("/ws", mgr.HandleWS)
	srv := httptest.NewServer(mux)

	return &node{rep: rep, mgr: mgr, server: srv}
}

// connect points n at the given peers (by their httptest URLs) and starts the
// transport. Must be called after all servers exist so addresses are known.
func (n *node) connect(peers ...*node) {
	var pl []replica.Peer
	for _, p := range peers {
		pl = append(pl, replica.Peer{ID: p.rep.ID, Address: p.server.URL})
	}
	// Rebuild the replica's peer list (New took nil). The manager reads
	// replica.Peers in Start, so set it before starting.
	n.rep.Peers = pl

	ctx, cancel := context.WithCancel(context.Background())
	n.cancel = cancel
	go n.mgr.Start(ctx)
}

func (n *node) stop() {
	if n.cancel != nil {
		n.cancel()
	}
	n.server.Close()
}

// eventually polls cond until true or the deadline passes.
func eventually(t *testing.T, d time.Duration, cond func() bool) bool {
	t.Helper()
	deadline := time.Now().Add(d)
	for time.Now().Before(deadline) {
		if cond() {
			return true
		}
		time.Sleep(20 * time.Millisecond)
	}
	return cond()
}

// Live convergence over real WebSocket connections: an op created on A appears on
// B via broadcast.
func TestLiveBroadcastConverges(t *testing.T) {
	a := startNode("a", 1000)
	b := startNode("b", 2000)
	defer a.stop()
	defer b.stop()

	a.connect(b)
	b.connect(a)

	// Wait for the mesh to come up, then write on A.
	if !eventually(t, 2*time.Second, func() bool { return len(a.mgr.conns()) > 0 && len(b.mgr.conns()) > 0 }) {
		t.Fatal("peers did not connect")
	}
	a.rep.CreateNode("1", "Email", 0, 0)
	a.rep.CreateNode("2", "AI", 0, 0)
	a.rep.CreateEdge("e1", "1", "2")

	if !eventually(t, 2*time.Second, func() bool { return a.rep.Hash() == b.rep.Hash() }) {
		t.Fatalf("broadcast did not converge:\n a=%s\n b=%s", a.rep.Hash(), b.rep.Hash())
	}
}

// Anti-entropy recovers state created BEFORE a peer connected (the catch-up path
// a reconnecting/crashed replica relies on).
func TestAntiEntropyCatchUp(t *testing.T) {
	a := startNode("a", 1000)
	b := startNode("b", 2000)
	defer a.stop()
	defer b.stop()

	// A has state before B is anywhere near it.
	a.rep.CreateNode("1", "preexisting", 0, 0)
	a.rep.CreateNode("2", "alsohere", 0, 0)

	// Now connect — broadcast won't carry the old ops; only anti-entropy will.
	a.connect(b)
	b.connect(a)

	if !eventually(t, 5*time.Second, func() bool { return a.rep.Hash() == b.rep.Hash() }) {
		t.Fatalf("anti-entropy did not catch B up:\n a=%s (%d)\n b=%s (%d)",
			a.rep.Hash(), nodes(a), b.rep.Hash(), nodes(b))
	}
}

func nodes(n *node) int { c, _ := n.rep.Counts(); return c }
