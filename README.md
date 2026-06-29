# SentinelSync

> A distributed state synchronization engine built with CRDTs and eventual consistency.

SentinelSync explores the consistency side of distributed systems — how multiple
replicas independently process concurrent updates and still converge to the same
state, with no central coordinator, under latency, packet loss, and partitions.
It is the consistency-focused companion to [SentinelCache](../sentinel-cache)
(availability, failure detection, leader election).

The state being synchronized is a **workflow graph** (nodes + edges), chosen over
text so the distributed-systems problem stays in focus instead of editor
plumbing. See [`docs/`](docs) for the blueprint, system design, and phased plan.

## Status

- **Phase 1 — Single Replica (done).** Standalone in-memory graph engine behind a
  REST API. No networking, no CRDT.
- **Phase 2 — Replica Architecture (done).** Three independent replicas
  (`replica-a/b/c`) via Docker Compose, each peer-aware but with **no sync** —
  divergence is demonstrable. Scaffolds the `crdt` types and the `Replica` struct.
- **Phase 3 — CRDT Engine (done).** State is now convergent: node/edge presence is
  an **OR-Set** (add-wins), title/position are **HLC-ordered LWW registers**, every
  mutation emits an **operation** that advances a **vector clock**, and a
  **convergence hash** (`stateHash` in `/status`) is the test oracle. Dangling
  edges are filtered at materialization. Convergence is proven by tests; ops are
  hand-fed (no transport yet).
- **Phase 4 — Replication Layer (done).** Each replica now has a WebSocket
  transport (`internal/transport`) that **broadcasts** every locally-generated
  operation to all peers (fast path) and runs an **anti-entropy** loop — periodic
  vector-clock diffs that replay any ops a peer missed (reliability backstop). A
  **gap-aware vector clock** (`recordSeq`) tracks only the contiguous prefix of
  each origin's sequence, so a dropped op triggers a re-request rather than being
  silently skipped. The 3-replica Docker cluster converges to an identical
  `stateHash` within milliseconds of a write. All tests pass with `-race`.

- **Phase 5 — Network Simulation (done).** Runtime fault injection via REST:
  `POST /sim/latency` (delay), `POST /sim/loss` (packet loss rate), `POST /sim/isolate`
  (soft partition — both directions blocked), `POST /sim/recover` (lift partition).
  Isolation keeps TCP connections alive so recovery is instantaneous on the next
  anti-entropy tick (≤3 s). `/status` exposes current chaos settings.
  Partition+recovery demonstrated on live 3-replica Docker cluster.

- **Phase 6 — Simulated Users (done).** Virtual users inside the backend fire random
  graph mutations (create/rename/move/delete) at a configurable rate via
  `POST /sim/users/start {"users":10,"opsPerSec":5.0}`. Each user goroutine tracks
  its own node namespace to avoid ID conflicts; deletes and concurrent mutations are
  handled gracefully. `Stop()` uses a `sync.WaitGroup` to drain goroutines before
  returning — ensuring no ops leak after the call. Live demo: 10 users, 5 ops/sec,
  150 ops in 3 s, all three replicas converged to the same `stateHash`.

- **Phase 7 — Dashboard (done).** React 18 + Vite 5 + React Flow 12 frontend
  served by nginx in Docker (`localhost:3000`). Live convergence banner (✓/⚠),
  per-replica status panels with inline chaos/sim controls, React Flow graph
  visualization, and an operation timeline. Polls `/status`, `/graph`, and `/ops`
  every 1.5 s with `AbortSignal.timeout(1000)` so a crashed replica never stalls
  the UI. Full partition+recovery demo visible from a single browser tab.

- **Phase 8 — Replay and Time Travel (done).** `GET /replay?upto=<index>` replays
  the op log through a throwaway replica and returns the graph at that moment.
  Frontend adds a convergence chart (SVG line chart of per-replica node counts,
  red bands for divergence windows) and a scrubber (range slider, debounced
  150 ms, "Go Live" button). History accumulates in App.tsx — up to 60 samples,
  enough to show a full partition+recovery cycle at 1.5 s polling. 4 new replay
  tests, all race-clean.

Project is **feature-complete** for the 8-phase plan.

## Quick start

```bash
make run         # start one replica on :8080
make test        # run tests with the race detector
make build       # compile to bin/replica

make docker-up   # start replicas (a:8080, b:8081, c:8082) + dashboard (:3000)
make docker-down # stop all containers

make frontend-install  # npm install (run once after clone)
make frontend-dev      # Vite dev server on :3000 (hot-reload)
make frontend-build    # production build → frontend/dist/
```

### REST + WebSocket API

| Method | Path | Purpose |
|---|---|---|
| `GET` | `/health` | Liveness |
| `GET` | `/status` | Replica id, peers, counts, vector clock, `stateHash`, tombstones |
| `GET` | `/graph` | Materialized graph snapshot (dangling edges filtered) |
| `POST` | `/node` | Create node `{id,title,x,y}` |
| `PATCH` | `/node/:id/title` | Rename `{title}` (LWW) |
| `PATCH` | `/node/:id/position` | Move `{x,y}` (LWW) |
| `DELETE` | `/node/:id` | Delete node (no cascade; edges dangle and are filtered) |
| `POST` | `/edge` | Create edge `{id,source,target}` (dangling allowed) |
| `DELETE` | `/edge/:id` | Delete edge |
| `GET` (WS) | `/ws` | Peer-to-peer replication endpoint (Phase 4) |
| `POST` | `/sim/latency` | Set outgoing message delay `{"ms":200}` (Phase 5) |
| `POST` | `/sim/loss` | Set packet loss probability `{"rate":0.3}` (Phase 5) |
| `POST` | `/sim/isolate` | Soft-partition this replica (Phase 5) |
| `POST` | `/sim/recover` | Lift soft-partition (Phase 5) |
| `POST` | `/sim/users/start` | Start virtual users `{"users":10,"opsPerSec":5.0}` (Phase 6) |
| `POST` | `/sim/users/stop` | Stop virtual users (Phase 6) |
| `GET` | `/sim/users/stats` | Sim stats: running, totalOps (Phase 6) |
| `GET` | `/ops` | Last 50 operations, newest first (Phase 7) |
| `GET` | `/replay` | Graph snapshot at op index `?upto=<N>` (Phase 8) |

Build narrative and per-file rationale live in [`DEVLOG.md`](DEVLOG.md).
