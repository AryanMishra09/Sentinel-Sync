# SentinelSync

**A distributed state synchronization engine built with CRDTs and eventual consistency.**

Multiple replicas edit the same workflow graph concurrently and converge to identical state — no leader, no coordinator — even under latency, packet loss, and network partitions. Every convergence claim is provable: equal `stateHash` values across replicas is a mathematical proof of identical state.

It is the consistency-focused companion to [SentinelCache](../sentinel-cache) (availability, failure detection, leader election).

---

## Features

| Feature | What it does |
|---|---|
| **OR-Set CRDT** | Node and edge presence uses an Observed-Remove Set (add-wins on concurrent create+delete) |
| **HLC-ordered LWW registers** | Node title and position are Last-Write-Wins, ordered by Hybrid Logical Clock — not raw wall time |
| **Vector clocks** | Every operation advances a per-origin sequence; replicas track the *contiguous* prefix so gaps are detected, not skipped |
| **Gap-aware anti-entropy** | Periodic vector-clock diffs replay any ops a peer missed — the reliability backstop when broadcast drops |
| **WebSocket replication** | Every local mutation broadcasts to peers instantly; anti-entropy heals anything broadcast missed |
| **Network fault injection** | REST-controlled latency, packet loss rate, and soft partition — all hot-swappable at runtime |
| **Virtual users** | Configurable goroutine pool fires random graph mutations at a target ops/sec — load generation without an external tool |
| **Convergence hash** | SHA-256 of the canonicalized materialized state — equal hash = provably identical replicas |
| **React dashboard** | Live graph (React Flow), per-replica status + controls, convergence banner, operation timeline |
| **Convergence chart** | SVG line chart of per-replica node counts over time — divergence windows highlighted in red |
| **Time travel / replay** | Scrub backwards through the op log — the graph reconstructs itself op-by-op |
| **Docker Compose** | One command starts three replicas + the dashboard |

---

## Why I Built This

I wanted to understand how collaborative editing tools (Google Docs, Figma, Notion) make concurrent edits from multiple users converge to the same state — without a central server serializing every write.

The answer is CRDTs (Conflict-free Replicated Data Types). SentinelSync implements them from scratch:

1. What happens when replica A and replica B both delete the same node simultaneously?
2. What happens when a replica is partitioned for 30 seconds and then reconnects?
3. How do you *prove* two replicas are identical without comparing every byte?

SentinelSync answers all three — and makes the answers observable in a browser.

---

## Demo

Start the cluster:

```bash
docker compose up --build
```

Four services start: `replica-a` (port 8080), `replica-b` (8081), `replica-c` (8082), and the dashboard (port 3000).

Open the dashboard:

```
http://localhost:3000
```

Start a simulation and watch the graph fill with nodes:

```bash
curl -s -X POST localhost:8080/sim/users/start \
  -H 'Content-Type: application/json' \
  -d '{"users":10,"opsPerSec":5}'
```

Partition replica B:

```bash
curl -s -X POST localhost:8081/sim/isolate
```

Watch the dashboard banner flip from **✓ Converged** to **⚠ Diverged**. B's node count freezes; A and C continue growing.

Recover B:

```bash
curl -s -X POST localhost:8081/sim/recover
```

Within 3 seconds — one anti-entropy tick — B receives all missing operations and the banner returns to **✓ Converged**. All three `stateHash` values are equal.

No manual intervention. No data loss.

---

## Quick Start

**Prerequisites:** Docker, Docker Compose, `curl`. Optional: [`jq`](https://stedolan.github.io/jq/) for pretty JSON.

```bash
git clone https://github.com/AryanMishra09/sentinel-sync
cd sentinel-sync
docker compose up --build
```

Three replicas + the dashboard start. Wait ~5 seconds for all replicas to peer-connect, then open `http://localhost:3000` or follow the walkthrough below.

---

## Hands-On Walkthrough

Everything step by step. Copy-paste the commands — no thinking required.

### 0. Verify the cluster is healthy

```bash
curl -s localhost:8080/status | jq '{replicaId, nodes, edges, stateHash}'
curl -s localhost:8081/status | jq '{replicaId, nodes, edges, stateHash}'
curl -s localhost:8082/status | jq '{replicaId, nodes, edges, stateHash}'
```

Expected — all hashes equal, all counts zero:

```json
{ "replicaId": "replica-a", "nodes": 0, "edges": 0, "stateHash": "e3b0c442..." }
{ "replicaId": "replica-b", "nodes": 0, "edges": 0, "stateHash": "e3b0c442..." }
{ "replicaId": "replica-c", "nodes": 0, "edges": 0, "stateHash": "e3b0c442..." }
```

Same hash on all three = provably identical empty state.

---

### 1. Basic create / rename / move / delete

Write a node to replica A. All three replicas receive it automatically via WebSocket broadcast.

```bash
# Create a node on replica A
curl -s -X POST localhost:8080/node \
  -H 'Content-Type: application/json' \
  -d '{"id":"n1","title":"Start","x":100,"y":200}' | jq .
```

```json
{ "id": "n1", "title": "Start", "x": 100, "y": 200 }
```

```bash
# Read it from replica C — it replicated automatically
curl -s localhost:8082/graph | jq '.nodes'
```

```json
[{ "id": "n1", "title": "Start", "x": 100, "y": 200 }]
```

```bash
# Rename it from replica B (LWW — most recent HLC timestamp wins)
curl -s -X PATCH localhost:8081/node/n1/title \
  -H 'Content-Type: application/json' \
  -d '{"title":"Ingestion"}' | jq .

# Move it from replica C
curl -s -X PATCH localhost:8082/node/n1/position \
  -H 'Content-Type: application/json' \
  -d '{"x":300,"y":150}' | jq .

# Verify convergence — all hashes should be equal
curl -s localhost:8080/status | jq .stateHash
curl -s localhost:8081/status | jq .stateHash
curl -s localhost:8082/status | jq .stateHash
```

All three hashes identical.

---

### 2. Edges and dangling edge behaviour

```bash
# Create two more nodes
curl -s -X POST localhost:8080/node \
  -H 'Content-Type: application/json' \
  -d '{"id":"n2","title":"Transform","x":400,"y":200}' | jq .id

curl -s -X POST localhost:8080/node \
  -H 'Content-Type: application/json' \
  -d '{"id":"n3","title":"Output","x":600,"y":200}' | jq .id

# Create edges
curl -s -X POST localhost:8080/edge \
  -H 'Content-Type: application/json' \
  -d '{"id":"e1","source":"n1","target":"n2"}' | jq .

curl -s -X POST localhost:8080/edge \
  -H 'Content-Type: application/json' \
  -d '{"id":"e2","source":"n2","target":"n3"}' | jq .

# Check the full graph on replica B
curl -s localhost:8081/graph | jq '{nodes: (.nodes | length), edges: (.edges | length)}'
```

```json
{ "nodes": 3, "edges": 2 }
```

```bash
# Delete the middle node — edges to/from it become dangling
curl -s -X DELETE localhost:8080/node/n2

# Dangling edges are filtered at materialization — they vanish from /graph
curl -s localhost:8082/graph | jq '{nodes: (.nodes | length), edges: (.edges | length)}'
```

```json
{ "nodes": 2, "edges": 0 }
```

The edges are still in the CRDT state (tombstone-free — they were never deleted), but they are invisible in the materialized view because their endpoint is gone. If n2 is re-created, the edges reappear.

---

### 3. Concurrent conflicting writes — add-wins

This is the core CRDT property. Simultaneously delete a node on replica A and create it on replica B — the create wins.

```bash
# Create a node to contest
curl -s -X POST localhost:8080/node \
  -H 'Content-Type: application/json' \
  -d '{"id":"contest","title":"Contested","x":0,"y":0}'

# In two separate terminals, run these as close to simultaneously as possible:

# Terminal 1 — delete from A
curl -s -X DELETE localhost:8080/node/contest

# Terminal 2 — rename from B (proves B "sees" the node)
curl -s -X PATCH localhost:8081/node/contest/title \
  -H 'Content-Type: application/json' \
  -d '{"title":"Survivor"}'
```

```bash
# Check all three replicas — add-wins: the node survives
curl -s localhost:8080/graph | jq '.nodes[] | select(.id=="contest")'
curl -s localhost:8081/graph | jq '.nodes[] | select(.id=="contest")'
curl -s localhost:8082/graph | jq '.nodes[] | select(.id=="contest")'
```

The node is present on all three replicas. The OR-Set add-wins semantics mean a concurrent create always beats a concurrent delete.

---

### 4. Network partition and recovery

Isolate replica B — both directions blocked, TCP connections kept alive.

```bash
# Partition B
curl -s -X POST localhost:8081/sim/isolate

# Confirm B is isolated
curl -s localhost:8081/status | jq '.chaos'
```

```json
{ "latencyMs": 0, "lossRate": 0, "isolated": true }
```

```bash
# Write to A and C while B is isolated
curl -s -X POST localhost:8080/node \
  -H 'Content-Type: application/json' \
  -d '{"id":"p1","title":"During Partition","x":50,"y":50}'

# B has 0 of the new node — it's partitioned
curl -s localhost:8081/graph | jq '.nodes | length'  # → old count, no p1

# A and C are converged with each other
curl -s localhost:8080/status | jq .stateHash
curl -s localhost:8082/status | jq .stateHash  # same hash

# B diverges
curl -s localhost:8081/status | jq .stateHash   # different hash
```

Recover B:

```bash
curl -s -X POST localhost:8081/sim/recover
```

```bash
# Wait ~3 seconds for the anti-entropy tick, then check
sleep 3
curl -s localhost:8080/status | jq .stateHash
curl -s localhost:8081/status | jq .stateHash
curl -s localhost:8082/status | jq .stateHash
```

All three hashes identical. B replayed every operation it missed — no data loss, no manual sync.

---

### 5. Packet loss

Configure replica A to drop 50% of outgoing messages.

```bash
curl -s -X POST localhost:8080/sim/loss \
  -H 'Content-Type: application/json' \
  -d '{"rate":0.5}'

# Write several nodes from A
for i in $(seq 1 10); do
  curl -s -X POST localhost:8080/node \
    -H 'Content-Type: application/json' \
    -d "{\"id\":\"loss$i\",\"title\":\"Loss $i\",\"x\":$((i*50)),\"y\":0}" > /dev/null
done

# B and C may have fewer nodes than A due to loss
curl -s localhost:8080/status | jq .nodes
curl -s localhost:8081/status | jq .nodes
curl -s localhost:8082/status | jq .nodes
```

Anti-entropy heals the gaps within seconds. Clear the loss and watch convergence complete:

```bash
curl -s -X POST localhost:8080/sim/loss \
  -H 'Content-Type: application/json' \
  -d '{"rate":0}'

sleep 4
curl -s localhost:8080/status | jq .stateHash
curl -s localhost:8081/status | jq .stateHash
curl -s localhost:8082/status | jq .stateHash
```

---

### 6. Latency injection

Add 300 ms of outgoing latency to replica C — all of its broadcasts and anti-entropy messages are delayed.

```bash
curl -s -X POST localhost:8082/sim/latency \
  -H 'Content-Type: application/json' \
  -d '{"ms":300}'

# Writes to C take 300 ms to reach A and B
curl -s -X POST localhost:8082/node \
  -H 'Content-Type: application/json' \
  -d '{"id":"slow","title":"Slow Node","x":0,"y":0}'

# Clear latency
curl -s -X POST localhost:8082/sim/latency \
  -H 'Content-Type: application/json' \
  -d '{"ms":0}'
```

---

### 7. Virtual users — simulated load

Start 10 virtual users on replica A firing 5 ops/sec each (50 ops/sec total):

```bash
curl -s -X POST localhost:8080/sim/users/start \
  -H 'Content-Type: application/json' \
  -d '{"users":10,"opsPerSec":5}'
```

```bash
# Watch ops accumulate
sleep 3
curl -s localhost:8080/sim/users/stats | jq .
```

```json
{ "running": true, "users": 10, "opsPerSec": 5, "totalOps": 152 }
```

```bash
# All three replicas converge under live load
curl -s localhost:8080/status | jq .stateHash
curl -s localhost:8081/status | jq .stateHash
curl -s localhost:8082/status | jq .stateHash

# Stop the simulation
curl -s -X POST localhost:8080/sim/users/stop
```

---

### 8. Time travel — replay the operation log

With some ops in the log (run the sim for a few seconds first):

```bash
# How many ops does replica A have?
curl -s localhost:8080/status | jq .opLogLen
# e.g. → 120
```

```bash
# What did the graph look like after op 10?
curl -s 'localhost:8080/replay?upto=9' | jq '{nodes: (.nodes|length), edges: (.edges|length)}'

# After op 50?
curl -s 'localhost:8080/replay?upto=49' | jq '{nodes: (.nodes|length), edges: (.edges|length)}'

# Current state
curl -s 'localhost:8080/replay?upto=119' | jq '{nodes: (.nodes|length), edges: (.edges|length)}'
```

The response is identical in shape to `GET /graph` — the full materialized snapshot at that point in time. The dashboard scrubber drives this endpoint interactively.

---

### 9. Dashboard walkthrough

Open `http://localhost:3000`.

**What you see:**

| Section | Description |
|---|---|
| Header banner | **✓ Converged** (green) or **⚠ Diverged** (red) — updates every 1.5 s |
| Online count | `3/3 replicas online` |
| Replica panels (×3) | Nodes, edges, ops, hash, vector clock chips, chaos/sim badges, all controls |
| Convergence chart | SVG line chart — per-replica node counts over the last 90 s. Red bands mark divergence windows. |
| Graph (React Flow) | Live graph from the selected replica — nodes + edges |
| Replay bar | Range slider over the full op log. Drag left to travel back in time. Click **● LIVE** to return to live view. |
| Operation timeline | Last 50 ops, newest first. Color-coded: green = create, blue = rename/move, red = delete. |

**Dashboard demo script:**

1. Click **Start Sim** on Replica A — 10 users, 5 ops/s. Graph fills; timeline streams.
2. Click **Isolate** on Replica B — `ISOLATED` badge appears. B's line on the convergence chart flattens; red band begins. Banner flips to **⚠ Diverged**.
3. Click **Recover** on Replica B — within 3 s the red band ends. Banner returns to **✓ Converged**.
4. Click **Stop Sim** on Replica A. Drag the replay scrubber left. The graph shrinks op-by-op. Drag it back right — the graph rebuilds. Click **● LIVE**.

---

## Running Without Docker

```bash
# Install frontend dependencies (once)
make frontend-install

# Terminal 1 — replica A
REPLICA_ID=replica-a REST_ADDR=:8080 \
  PEERS="replica-b=http://localhost:8081,replica-c=http://localhost:8082" \
  ./bin/replica

# Terminal 2 — replica B
REPLICA_ID=replica-b REST_ADDR=:8081 \
  PEERS="replica-a=http://localhost:8080,replica-c=http://localhost:8082" \
  ./bin/replica

# Terminal 3 — replica C
REPLICA_ID=replica-c REST_ADDR=:8082 \
  PEERS="replica-a=http://localhost:8080,replica-b=http://localhost:8081" \
  ./bin/replica

# Terminal 4 — dashboard dev server (hot-reload)
make frontend-dev
```

Build the binary first: `make build`.

> **Use the compiled binary, not `go run`**. `go run` wraps the binary in a subprocess — `kill $PID` hits the wrapper, not the replica. Orphan processes will hold ports open.

---

## Tests

```bash
make test          # go test -race ./...
```

All tests use the race detector. Current coverage:

| Package | Tests | What they cover |
|---|---|---|
| `internal/graph` | 8 | Node/edge CRUD, dangling edge filter, hash determinism |
| `internal/replica` | 6 | Operation generation, Ingest idempotency, gap-aware vector clock, anti-entropy diff |
| `internal/simulation` | 9 | Chaos: default no-drop, isolation, packet loss, delay, snapshot |
| `internal/simulation` | 5 | Simulator: ops accumulate, start/stop idempotent, invalid params, convergence, cumulative stats |
| `internal/transport` | 3 | Live 2-replica convergence, chaos isolation convergence, chaos packet-loss convergence |
| `internal/api` | 4 | Replay: empty log, bad params, out-of-bounds clamp, step-by-step node count |

---

## Makefile

```bash
make build            # compile to bin/replica
make run              # start one replica on :8080 (no peers)
make test             # go test -race ./...
make tidy             # go mod tidy
make docker-up        # build all 4 images and start (replicas + dashboard)
make docker-down      # stop and remove containers
make docker-logs      # tail logs from all services
make clean            # remove bin/, frontend/dist/, frontend/node_modules/
make frontend-install # npm install in ./frontend (run once)
make frontend-dev     # Vite dev server on :3000 with hot-reload
make frontend-build   # tsc && vite build → frontend/dist/
```

---

## REST + WebSocket API

| Method | Path | Body / Query | Purpose |
|---|---|---|---|
| `GET` | `/health` | — | Liveness check |
| `GET` | `/status` | — | Full replica state: counts, vector clock, `stateHash`, chaos, sim |
| `GET` | `/graph` | — | Materialized graph snapshot (dangling edges filtered) |
| `GET` | `/ops` | — | Last 50 operations, newest first |
| `GET` | `/replay` | `?upto=<N>` | Graph snapshot after replaying ops 0..N |
| `POST` | `/node` | `{id, title, x, y}` | Create node |
| `PATCH` | `/node/:id/title` | `{title}` | Rename node (LWW) |
| `PATCH` | `/node/:id/position` | `{x, y}` | Move node (LWW) |
| `DELETE` | `/node/:id` | — | Delete node (edges dangle, filtered at read) |
| `POST` | `/edge` | `{id, source, target}` | Create edge |
| `DELETE` | `/edge/:id` | — | Delete edge |
| `GET` (WS) | `/ws` | — | Replica-to-replica replication WebSocket |
| `POST` | `/sim/latency` | `{"ms": 200}` | Set outgoing message delay |
| `POST` | `/sim/loss` | `{"rate": 0.3}` | Set outgoing packet loss probability (0–1) |
| `POST` | `/sim/isolate` | — | Soft-partition this replica (both directions) |
| `POST` | `/sim/recover` | — | Lift soft-partition |
| `POST` | `/sim/users/start` | `{"users": 10, "opsPerSec": 5.0}` | Start virtual users |
| `POST` | `/sim/users/stop` | — | Stop virtual users |
| `GET` | `/sim/users/stats` | — | Running state, user count, total ops |

---

## Architecture

```
                     ┌─────────────────┐
                     │   Browser       │
                     │  (React + Vite) │
                     │  localhost:3000  │
                     └────────┬────────┘
                              │ REST polls (1.5 s)
           ┌──────────────────┼──────────────────┐
           ▼                  ▼                  ▼
     ┌──────────┐       ┌──────────┐       ┌──────────┐
     │replica-a │       │replica-b │       │replica-c │
     │  :8080   │       │  :8081   │       │  :8082   │
     └────┬─────┘       └────┬─────┘       └────┬─────┘
          │                  │                  │
          └──────────────────┴──────────────────┘
                   WebSocket replication
                  (broadcast + anti-entropy)
```

Each replica is **symmetric** — no leader, no coordinator. Any replica accepts any write. Operations propagate to all peers via two mechanisms:

- **Broadcast** (fast path): every local mutation is sent to all connected peers immediately.
- **Anti-entropy** (reliability backstop): every 3 seconds, replicas exchange vector clocks and replay any operations the peer has not yet seen. This heals anything broadcast missed.

---

## How the Algorithms Work

### OR-Set (Add-Wins Observed-Remove Set)

Each create operation tags the item with a unique `(replicaID, counter)` pair. Deletion names the specific tags it is removing. If a concurrent create generates a *new* tag (not in the delete's tombstone set), the item survives. Result: concurrent create + delete → item present. This is the "add-wins" semantic.

### HLC-Ordered LWW Register

Node title and position use Last-Write-Wins with Hybrid Logical Clocks. An HLC timestamp is `(physicalMs, logicalCounter, replicaID)`. It always advances monotonically even when the system clock goes backward, and it embeds a tiebreaker for simultaneous writes. A rename from replica A at `(1000, 0, a)` beats one from replica B at `(999, 5, b)` because the physical component is higher. The raw wall clock is never used directly for ordering.

### Gap-Aware Vector Clock

Each replica tracks the highest *contiguous* sequence it has seen from every other replica — not the maximum observed sequence. If op #3 from a peer is dropped but #4 arrives, the clock stays at 2. Anti-entropy compares clocks and re-sends #3. A max-merge clock would stay at 4, silently skipping the gap forever.

### Anti-Entropy

Every 3 seconds, each replica calls `MissingFor(peerClock)` — it walks its op log and returns every operation whose origin sequence exceeds what the peer's clock shows it has seen. The peer ingests them. `Ingest` is idempotent by operation ID, so duplicate deliveries are harmless.

### Convergence Hash

`stateHash` is `SHA-256(canonical JSON of materialized state)`. The state is canonicalized by sorting node IDs and edge IDs before hashing — identical CRDT state from different operation orderings produces the same hash. Equal hashes on two replicas is a proof, not just a likely indicator, that they hold identical state.

---

## Project Structure

```
sentinel-sync/
├── cmd/replica/main.go           ← binary entry point
├── internal/
│   ├── crdt/
│   │   ├── operation.go          ← Operation type, OpType constants
│   │   ├── hlc.go                ← Hybrid Logical Clock
│   │   ├── vector_clock.go       ← increment / merge / compare
│   │   ├── orset.go              ← add-wins OR-Set
│   │   └── lww.go                ← HLC-ordered LWW register
│   ├── graph/
│   │   ├── state.go              ← CRDT graph state: Apply, Snapshot, Hash
│   │   ├── node.go / edge.go     ← node and edge types + payloads
│   │   └── graph_test.go
│   ├── replica/
│   │   ├── replica.go            ← Replica: emit, Ingest, MissingFor, Clock
│   │   └── replica_test.go
│   ├── transport/
│   │   ├── transport.go          ← WebSocket manager: broadcast + anti-entropy
│   │   └── transport_test.go
│   ├── simulation/
│   │   ├── chaos.go              ← Chaos: latency, loss rate, isolation
│   │   ├── chaos_test.go
│   │   ├── simulator.go          ← Simulator: virtual user goroutines
│   │   └── simulator_test.go
│   └── api/
│       ├── handlers.go           ← graph CRUD + /status + /ops handlers
│       ├── replay.go             ← GET /replay handler
│       ├── replay_test.go
│       ├── sim.go                ← /sim/latency, /loss, /isolate, /recover
│       ├── sim_users.go          ← /sim/users/start, /stop, /stats
│       └── routes.go             ← route table
├── frontend/
│   ├── src/
│   │   ├── App.tsx               ← layout, history buffer, replay state
│   │   ├── App.css               ← dark theme (CSS variables, no framework)
│   │   ├── types.ts              ← TypeScript mirrors of Go API types
│   │   ├── hooks/useReplica.ts   ← useReplicas, useGraph, useOps, useReplay, apiPost
│   │   └── components/
│   │       ├── ReplicaPanel.tsx  ← per-replica status card + controls
│   │       ├── GraphView.tsx     ← React Flow wrapper
│   │       ├── Timeline.tsx      ← operation timeline
│   │       ├── ConvergenceChart.tsx ← SVG line chart with divergence bands
│   │       └── ReplayBar.tsx     ← scrubber + Go Live button
│   ├── Dockerfile                ← node:20-alpine builder → nginx:1.27-alpine
│   └── nginx.conf                ← SPA fallback on :3000
├── docs/
│   ├── BLUEPRINT.md              ← project vision and goals
│   ├── SYSTEM_DESIGN.md          ← detailed design doc
│   └── IMPLEMENTATION_PLAN.md    ← 8-phase build plan
├── Dockerfile                    ← replica binary image
├── docker-compose.yml            ← 3 replicas + dashboard
├── Makefile
└── DEVLOG.md                     ← build journal: every file and decision explained
```

---

## Design Tradeoffs

### Why OR-Set (add-wins) instead of delete-wins?

In a workflow graph editor, accidental deletions are rare and expensive. Add-wins means a concurrent create always survives a concurrent delete. Delete-wins would silently discard a node that another user just created — confusing and hard to recover from. The tradeoff is that a deliberate delete may "lose" if there's an in-flight create, but that is the less disruptive failure mode.

### Why HLC instead of vector clocks for LWW ordering?

Vector clocks track causality but don't provide a total order — two concurrent writes from different replicas have incomparable vector clocks, so you cannot decide which "wins". HLC provides both: it advances monotonically (causality) and always produces a comparable timestamp (total order for LWW). The `(physicalMs, logical, replicaID)` tiebreaker makes the order deterministic even for operations generated in the same millisecond.

### Why WebSocket for replication instead of gRPC?

WebSocket is persistent (no per-op connection overhead), bidirectional (both broadcast and anti-entropy use the same connection), and requires no code generation or protobuf schema. For a 3-node cluster the performance difference from gRPC is negligible. The meaningful boundary is REST (client-facing, human-readable) vs WebSocket (replica-to-replica, low-overhead persistent).

### Why soft partition instead of killing connections?

Keeping TCP connections alive during `sim/isolate` means recovery is instant: the next anti-entropy tick (≤3 s) sees the recovered state and syncs. Killing connections would add reconnection delay (dial timeout + WebSocket handshake) before anti-entropy could run. For demo purposes, the important moment to show is the hashes converging — not the reconnection plumbing.

### Why polling instead of WebSocket push for the dashboard?

The dashboard is an *observer* of the distributed system, not a participant. REST polling is simpler, survives replica restarts with no reconnection logic, and makes each poll a fresh independent HTTP request — easier to reason about under fault injection. The 1.5 s interval with `AbortSignal.timeout(1000)` means a crashed replica surfaces as "offline" within 2.5 s with no hanging connections.

---

## Status

All 8 phases complete.

| Phase | What it built |
|---|---|
| 1 — Single Replica | In-memory graph engine, REST API |
| 2 — Replica Architecture | 3-replica Docker Compose cluster, divergence demonstrable |
| 3 — CRDT Engine | OR-Set, HLC, vector clock, LWW, convergence hash |
| 4 — Replication | WebSocket broadcast + gap-aware anti-entropy |
| 5 — Network Simulation | Latency, packet loss, soft partition — REST-controlled |
| 6 — Simulated Users | Virtual user goroutines with WaitGroup drain |
| 7 — Dashboard | React 18 + React Flow + dark theme + all sim controls |
| 8 — Replay | Convergence chart, time travel scrubber, GET /replay |

---

## Non-Goals

Intentionally excluded (learning project, not a production system):

- Persistence / WAL (replicas start empty on restart)
- TLS
- Raft / Paxos (no consensus needed — CRDTs are leaderless by design)
- Quorum reads/writes
- Kubernetes

---

See [`DEVLOG.md`](DEVLOG.md) for the complete build journal — every file, design decision, and concept explained in the order it was built.
