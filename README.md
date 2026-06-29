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

Roadmap: **Phase 6** simulated users → 7 dashboard → 8 replay / time travel.

## Quick start

```bash
make run         # start one replica on :8080
make test        # run tests with the race detector
make build       # compile to bin/replica

make docker-up   # start the 3-replica cluster (a:8080, b:8081, c:8082)
make docker-down # stop it
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

Build narrative and per-file rationale live in [`DEVLOG.md`](DEVLOG.md).
