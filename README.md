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

**Phase 1 — Single Replica (done).** A standalone in-memory graph engine behind a
REST API. No networking, no CRDT yet. This is the local state model every later
phase makes convergent.

Roadmap: Phase 2 multi-replica → 3 CRDT engine (OR-Set, HLC-LWW, vector clocks,
convergence checker) → 4 replication + anti-entropy → 5 network simulation →
6 simulated users → 7 dashboard → 8 replay / time travel.

## Quick start

```bash
make run        # start one replica on :8080
make test       # run tests with the race detector
make build      # compile to bin/replica
```

### REST API (Phase 1)

| Method | Path | Purpose |
|---|---|---|
| `GET` | `/health` | Liveness |
| `GET` | `/status` | Replica ID + node/edge counts |
| `GET` | `/graph` | Full graph snapshot |
| `POST` | `/node` | Create node `{id,title,x,y}` |
| `PATCH` | `/node/:id/title` | Rename `{title}` |
| `PATCH` | `/node/:id/position` | Move `{x,y}` |
| `DELETE` | `/node/:id` | Delete node (cascades its edges) |
| `POST` | `/edge` | Create edge `{id,source,target}` |
| `DELETE` | `/edge/:id` | Delete edge |

Build narrative and per-file rationale live in [`DEVLOG.md`](DEVLOG.md).
