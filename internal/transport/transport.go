// Package transport is the replica-to-replica WebSocket layer (Phase 4).
//
// It turns the Phase 3 CRDT engine — which only converged when operations were
// hand-fed — into a live, self-healing cluster. Two mechanisms work together
// (SYSTEM_DESIGN §8, §25):
//
//   - Broadcast: every locally generated operation is pushed to all peers
//     immediately. Fast path, best-effort.
//   - Anti-entropy: periodically each replica sends its vector clock to peers,
//     who reply with any operations it is missing. Reliability backstop — this
//     is what recovers from dropped messages, partitions, and crashes.
//
// Phase 5 adds a Chaos injector that can introduce latency, packet loss, and
// full soft-isolation on any replica at runtime via the /sim REST endpoints.
//
// Topology: a full mesh. Each replica DIALS every peer (outbound connections it
// broadcasts on) and ACCEPTS connections from peers (inbound). Operations are
// never relayed — a peer applies what it receives but does not re-broadcast it;
// dedup by operation ID makes any redundancy harmless and anti-entropy covers
// any peer a direct link missed.
package transport

import (
	"context"
	"log"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/aryan-mishra/sentinel-sync/internal/crdt"
	"github.com/aryan-mishra/sentinel-sync/internal/replica"
	"github.com/aryan-mishra/sentinel-sync/internal/simulation"
	"github.com/gorilla/websocket"
)

const (
	reconnectDelay = 2 * time.Second
	syncInterval   = 3 * time.Second
)

type msgType string

const (
	msgOp       msgType = "op"
	msgSyncReq  msgType = "sync_request"
	msgSyncResp msgType = "sync_response"
)

// message is the wire envelope. Only the field relevant to Type is populated.
type message struct {
	Type  msgType          `json:"type"`
	Op    *crdt.Operation  `json:"op,omitempty"`
	Clock crdt.VectorClock `json:"clock,omitempty"`
	Ops   []crdt.Operation `json:"ops,omitempty"`
}

// peerConn wraps a WebSocket connection with a write mutex — gorilla forbids
// concurrent writes, and broadcast, anti-entropy, and sync replies can all write
// to the same connection.
type peerConn struct {
	mu sync.Mutex
	ws *websocket.Conn
}

func (c *peerConn) write(v any) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.ws.WriteJSON(v)
}

// Manager owns one replica's transport.
type Manager struct {
	replica  *replica.Replica
	chaos    *simulation.Chaos
	upgrader websocket.Upgrader

	mu       sync.RWMutex
	outbound map[string]*peerConn // peerID -> our dialed connection
}

// NewManager builds a transport for r and registers itself as r's broadcaster.
func NewManager(r *replica.Replica, chaos *simulation.Chaos) *Manager {
	m := &Manager{
		replica:  r,
		chaos:    chaos,
		outbound: make(map[string]*peerConn),
		upgrader: websocket.Upgrader{
			CheckOrigin: func(*http.Request) bool { return true }, // intra-cluster only
		},
	}
	r.SetBroadcast(m.Broadcast)
	return m
}

// HandleWS is the inbound endpoint (mounted at /ws). A peer dials here; we read
// its operations and sync requests and reply on the same connection.
func (m *Manager) HandleWS(w http.ResponseWriter, r *http.Request) {
	ws, err := m.upgrader.Upgrade(w, r, nil)
	if err != nil {
		return
	}
	pc := &peerConn{ws: ws}
	m.readLoop(pc) // inbound conns aren't stored — we never broadcast on them
}

// Start dials every peer (with reconnect) and runs the anti-entropy loop. It
// blocks until ctx is cancelled.
func (m *Manager) Start(ctx context.Context) {
	for _, p := range m.replica.Peers {
		go m.dialLoop(ctx, p)
	}
	m.antiEntropyLoop(ctx)
}

// Broadcast pushes an operation to all connected peers. Chaos faults (isolation,
// packet loss, latency) are applied per-peer before writing. Best-effort: write
// failures just drop the connection (the dial loop reconnects, and anti-entropy
// recovers anything missed).
func (m *Manager) Broadcast(op crdt.Operation) {
	for _, pc := range m.conns() {
		if m.chaos.ShouldDrop() {
			continue
		}
		m.chaos.ApplyDelay()
		if err := pc.write(message{Type: msgOp, Op: &op}); err != nil {
			_ = pc.ws.Close()
		}
	}
}

// --- internals -------------------------------------------------------------

func (m *Manager) dialLoop(ctx context.Context, p replica.Peer) {
	url := wsURL(p.Address)
	for ctx.Err() == nil {
		ws, _, err := websocket.DefaultDialer.DialContext(ctx, url, nil)
		if err != nil {
			sleepCtx(ctx, reconnectDelay)
			continue
		}
		pc := &peerConn{ws: ws}
		m.setOutbound(p.ID, pc)
		log.Printf("[%s] connected to peer %s", m.replica.ID, p.ID)

		// Immediately reconcile on (re)connect — this is what makes a replica
		// catch up after a partition or crash. Skip the send if currently dropping.
		if !m.chaos.ShouldDrop() {
			m.chaos.ApplyDelay()
			_ = pc.write(message{Type: msgSyncReq, Clock: m.replica.Clock()})
		}

		m.readLoop(pc) // blocks until the connection drops
		m.clearOutbound(p.ID, pc)
		log.Printf("[%s] lost connection to peer %s", m.replica.ID, p.ID)
		sleepCtx(ctx, reconnectDelay)
	}
}

func (m *Manager) readLoop(pc *peerConn) {
	defer pc.ws.Close()
	for {
		var msg message
		if err := pc.ws.ReadJSON(&msg); err != nil {
			return
		}
		// When isolated, discard all incoming messages — the replica is
		// soft-partitioned (both directions blocked).
		if m.chaos.IsIsolated() {
			continue
		}
		switch msg.Type {
		case msgOp:
			if msg.Op != nil {
				m.replica.Ingest(*msg.Op)
			}
		case msgSyncReq:
			missing := m.replica.MissingFor(msg.Clock)
			if len(missing) > 0 {
				_ = pc.write(message{Type: msgSyncResp, Ops: missing})
			}
		case msgSyncResp:
			for _, op := range msg.Ops {
				m.replica.Ingest(op)
			}
		}
	}
}

func (m *Manager) antiEntropyLoop(ctx context.Context) {
	ticker := time.NewTicker(syncInterval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			// Skip the tick entirely while isolated — no point sending a clock
			// when all outgoing traffic is dropped.
			if m.chaos.IsIsolated() {
				continue
			}
			clock := m.replica.Clock()
			for _, pc := range m.conns() {
				if m.chaos.ShouldDrop() {
					continue
				}
				m.chaos.ApplyDelay()
				_ = pc.write(message{Type: msgSyncReq, Clock: clock})
			}
		}
	}
}

func (m *Manager) conns() []*peerConn {
	m.mu.RLock()
	defer m.mu.RUnlock()
	out := make([]*peerConn, 0, len(m.outbound))
	for _, pc := range m.outbound {
		out = append(out, pc)
	}
	return out
}

func (m *Manager) setOutbound(id string, pc *peerConn) {
	m.mu.Lock()
	m.outbound[id] = pc
	m.mu.Unlock()
}

// clearOutbound removes pc only if it is still the current connection for id, so
// a stale dropped connection can't evict a freshly reconnected one.
func (m *Manager) clearOutbound(id string, pc *peerConn) {
	m.mu.Lock()
	if m.outbound[id] == pc {
		delete(m.outbound, id)
	}
	m.mu.Unlock()
}

// wsURL converts a peer's http(s) address into its ws(s) /ws endpoint.
func wsURL(addr string) string {
	switch {
	case strings.HasPrefix(addr, "https://"):
		addr = "wss://" + strings.TrimPrefix(addr, "https://")
	case strings.HasPrefix(addr, "http://"):
		addr = "ws://" + strings.TrimPrefix(addr, "http://")
	}
	return strings.TrimRight(addr, "/") + "/ws"
}

func sleepCtx(ctx context.Context, d time.Duration) {
	select {
	case <-ctx.Done():
	case <-time.After(d):
	}
}
