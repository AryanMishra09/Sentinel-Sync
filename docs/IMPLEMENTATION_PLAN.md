# IMPLEMENTATION_PLAN.md

# SentinelSync

> Distributed State Synchronization Engine using CRDTs

---

# 1. Project Philosophy

This project is NOT:

* another workflow SaaS
* another Notion clone
* another Miro clone
* another React editor

This project IS:

* a distributed systems project
* a consistency project
* a CRDT project
* a synchronization engine

Frontend exists only to visualize behavior.

Backend is the star.

---

# 2. Final Tech Stack

## Backend

```text
Go 1.24+
```

Reason:

* consistency with SentinelCache
* concurrency primitives
* strong typing
* distributed systems ecosystem

---

## API Layer

```text
Gin
```

Reason:

* familiar
* simple
* excellent ecosystem

---

## Realtime Communication

```text
WebSockets
```

Library:

```text
gorilla/websocket
```

Used for:

* replica communication
* dashboard updates
* operation streaming

---

## Frontend

```text
React
TypeScript
Vite
```

---

## Graph Rendering

```text
React Flow
```

Reason:

* workflow visualization
* drag/drop nodes
* edge rendering
* battle tested

---

## State Management

```text
Zustand
```

Simple.

No Redux.

---

## Deployment

```text
Docker Compose
```

3 replicas.

Exactly like SentinelCache.

---

# 3. Final Architecture

```text
                    Dashboard

                         |
                         |
                    WebSocket
                         |
     ----------------------------------------
     |                 |                   |
     |                 |                   |
     ▼                 ▼                   ▼

  Replica A        Replica B          Replica C

      ↕               ↕                  ↕

      CRDT Replication Layer
```

---

# 4. Repository Structure

```text
sentinel-sync/

├── cmd/
│   └── replica/
│       └── main.go

├── internal/

│   ├── crdt/
│   │   ├── orset.go
│   │   ├── lww.go
│   │   ├── hlc.go            # Hybrid Logical Clock (LWW ordering)
│   │   ├── vector_clock.go   # used for sync + concurrency detection only
│   │   └── operation.go

│   ├── graph/
│   │   ├── node.go
│   │   ├── edge.go
│   │   ├── graph.go
│   │   ├── materialize.go    # resolve OR-Sets/LWW, filter dangling edges
│   │   └── convergence.go    # canonical state hash (test oracle + metric)

│   ├── replica/
│   │   ├── replica.go
│   │   ├── manager.go
│   │   ├── sync.go           # anti-entropy reconciliation (first-class)
│   │   └── antientropy.go    # periodic vector-clock diff + op replay

│   ├── transport/
│   │   ├── websocket.go
│   │   └── broadcaster.go

│   ├── simulation/
│   │   ├── users.go
│   │   ├── latency.go
│   │   ├── partition.go
│   │   └── packet_loss.go

│   ├── api/
│   │   ├── routes.go
│   │   └── handlers.go

│   └── metrics/
│       └── metrics.go

├── frontend/

├── docker-compose.yml

├── Makefile

└── README.md
```

---

# 5. Phase Breakdown

The project will be built in:

```text
Phase 1
Single Replica

Phase 2
Multi Replica

Phase 3
CRDT Convergence

Phase 4
Network Failures

Phase 5
Simulation Dashboard
```

---

# PHASE 1

# Single Replica

Goal:

```text
Build local graph engine
```

No networking.

No CRDT.

No replicas.

---

## Features

### Create Node

```json
{
  "id":"1",
  "title":"Email"
}
```

---

### Rename Node

```json
{
  "id":"1",
  "title":"AI Processor"
}
```

---

### Move Node

```json
{
  "id":"1",
  "x":400,
  "y":200
}
```

---

### Delete Node

```json
{
  "id":"1"
}
```

---

### Create Edge

```json
{
  "source":"1",
  "target":"2"
}
```

---

### Delete Edge

```json
{
  "edge":"e1"
}
```

---

## Deliverable

Single graph editor.

No synchronization.

No distributed system yet.

---

# PHASE 2

# Replica Architecture

Goal:

```text
3 independent replicas
```

Each replica:

```text
Own graph
Own state
Own operation log
```

---

Docker Compose:

```text
replica-a : 8080

replica-b : 8081

replica-c : 8082
```

---

Replica structure:

```go
type Replica struct {
    ID string

    Graph *Graph

    VectorClock VectorClock

    OperationLog []Operation
}
```

---

## Deliverable

3 replicas.

Still no synchronization.

---

# PHASE 3

# CRDT Engine

This is the core.

---

## Step 1

Operation Model

Every mutation becomes:

```go
type Operation struct {
    ID string

    ReplicaID string

    Type string

    Payload []byte

    // Sync + concurrency detection only — NOT used by merge logic.
    VectorClock map[string]int

    // Hybrid Logical Clock — drives LWW conflict resolution (Step 4).
    HLC HLCTimestamp
}
```

---

## Step 2

Vector Clocks

Implement:

```go
Increment()

Merge()

Compare()
```

> Scope note: vector clocks are NOT used to merge state. OR-Set merges via tags
> (Step 3); LWW merges via HLC (Step 4). Vector clocks exist for two things only:
> (1) the anti-entropy diff in Phase 4 ("which ops is this peer missing?"), and
> (2) labeling concurrent vs causal ops in the timeline UI. Implement them for
> those, not for the merge functions.

---

## Step 3

OR-Set

Used for:

```text
Nodes
Edges
```

Supports:

```text
Add

Remove

Merge
```

---

## Step 4

LWW Register (HLC-ordered)

Used for:

```text
Title

Position
```

Supports:

```text
Concurrent rename

Concurrent move
```

Ordering is by **Hybrid Logical Clock**, never a raw wall clock:

```go
type HLCTimestamp struct {
    Physical  int64  // monotonic, max(local, remote)+ on receive
    Logical   int64  // same-millisecond tiebreak
    ReplicaID string // final deterministic tiebreak
}
```

Compare order: Physical → Logical → ReplicaID. This avoids the clock-skew
contradiction (a raw `time.Now()` would let the fastest clock win every rename).
Implement `hlc.Now()`, `hlc.Update(remote)`, and `hlc.Compare(a, b)`.

---

## Step 5

Edge Referential Integrity

Two independent OR-Sets (nodes, edges) converge individually but can leave a
**dangling edge** (edge survives, its endpoint node was concurrently deleted).

Policy: do NOT mutate the edge OR-Set. Enforce the invariant at materialization
— `graph/materialize.go` filters out any edge whose source/target is not a live
node. If the node is re-added later, the edge reappears automatically.

---

## Step 6

Convergence Checker

Build `graph/convergence.go`:

```text
canonicalize(materialized graph) -> sorted, resolved form
hash(canonical) -> stateHash
```

This is the **test oracle** (assert all replicas' hashes equal after quiescence)
and the source of the divergence/convergence metrics. Build it now, before any
chaos — it is how you know the CRDT is correct.

---

## Deliverable

Replica convergence, **proven by equal state hashes** across replicas.

No networking failures yet.

---

# PHASE 4

# Replication Layer

Goal:

```text
Replica communication
```

---

Each replica opens:

```text
WebSocket Server
```

and connects to peers.

---

Example:

```text
A ↔ B

B ↔ C

A ↔ C
```

---

Operation Flow:

```text
User

↓

Replica A

↓

Create Operation

↓

Apply Local

↓

Broadcast

↓

Replica B

↓

Apply

↓

Replica C

↓

Apply
```

---

## Exactly-once application

Broadcast over a full mesh means an op can arrive twice (multiple paths, retries).
Dedup by operation ID at the apply layer so replay/duplicates are idempotent.

---

## Anti-entropy (build it here, not in Phase 5)

Broadcast alone diverges under loss — this is the hybrid model from SYSTEM_DESIGN
§8. Implement `replica/antientropy.go` now:

```text
periodically (and on reconnect):
1. exchange vector clocks with a peer
2. compute the diff (which ops the peer lacks)
3. send the missing ops from the operation log
4. peer replays them (idempotent via op ID)
```

Phase 5's partition/crash recovery is then just this same loop running after the
network heals — no separate recovery code path.

---

## Deliverable

Live synchronization **that self-heals** — operations lost in transit are
recovered by the next anti-entropy round.

---

# PHASE 5

# Network Simulation

Most important demo phase.

---

## Latency Injection

Example:

```text
200ms
```

All outgoing messages delayed.

---

Implementation:

```go
time.Sleep(latency)
```

before sending.

---

## Packet Loss

Example:

```text
20%
```

Implementation:

```go
rand.Float64()
```

drop message.

---

## Failure Models (two distinct buttons)

Keep these separate — they demonstrate different things.

### Soft isolation (a partition, not a crash)

```text
Button: "Isolate Replica B"
Impl:   stop sending/receiving; process + in-memory state retained
Recovery: cheap delta sync (small vector-clock diff)
```

This is honestly a partition; do not label it "crash".

### Hard crash (state lost)

```text
Button / CLI: docker compose stop replica-b
Impl:   process dies; in-memory op log + graph lost (V1)
Recovery: full catch-up — B restarts with a zero clock, so anti-entropy
          streams the ENTIRE history from peers and B replays it
```

Caveat: if all replicas hard-crash together, in-memory state is gone everywhere
(persistent log is V2). Peer recovery needs at least one survivor.

---

## Network Partition

Example:

```text
A ↔ C

B isolated
```

Operations continue.

---

## Rejoin

Replica B reconnects.

Requests missing operations.

Replays operation log.

Converges.

---

## Deliverable

Distributed systems demo.

---

# PHASE 6

# Simulated Users

This is where project becomes unique.

---

Instead of:

```text
Open 3 browser tabs
```

we create:

```text
Virtual users
```

inside backend.

---

Example:

```text
100 users

Generate:

Create Node

Move Node

Rename Node

Delete Node
```

randomly.

---

Simulator:

```go
type SimulatedUser struct {
    ID string

    ActionsPerSecond int
}
```

---

Goal:

```text
Stress test convergence
```

---

# PHASE 7

# Dashboard

The dashboard exists to explain:

```text
Why state converged
```

not merely display state.

---

Layout

```text
--------------------------------------------------

Replica Controls

--------------------------------------------------

Graph

--------------------------------------------------

Replica States

--------------------------------------------------

Operation Timeline

--------------------------------------------------

Metrics

--------------------------------------------------
```

---

# Replica Controls

Buttons:

```text
Isolate Replica   (soft partition — state retained)

Hard Crash        (process down — state lost, full catch-up on restart)

Recover Replica   (triggers anti-entropy resync)

Partition Network

Add Latency

Add Packet Loss
```

---

# Graph View

Visual workflow graph.

Shows:

```text
Nodes

Edges
```

live.

---

# Replica State View

Display:

```text
Replica A

Nodes: 45

Operations: 700

Clock:
A=10
B=8
C=12
```

---

# Timeline

Every operation.

Example:

```text
10:01:00

CreateNode

Replica A
```

---

# Metrics

Display:

```text
Ops/sec

Replication Lag

Divergence Count      (replicas off the majority state hash — Step 6)

Convergence Time      (last op → all state hashes equal)

Messages/sec

OR-Set size / replica (tombstone growth — shows why GC is needed)

Operation-log size    (unbounded-growth visualization)
```

---

# Phase 8

# Resume Enhancement Features

These are optional but huge for interviews.

---

## Replay System

Replay:

```text
Entire history
```

from operation log.

---

## Time Travel

Move timeline:

```text
T1

T2

T3
```

and visualize state.

---

## Convergence Visualizer

Show:

```text
Replica A = 50 nodes

Replica B = 47 nodes

Replica C = 52 nodes
```

during partition.

Then:

```text
50

50

50
```

after recovery.

Backed by the convergence checker (Phase 3, Step 6): the visualizer plots each
replica's node count *and* its canonical state hash. Divergence = differing
hashes during the partition; convergence = all hashes collapse to one value after
recovery. The hash is what makes "they're actually identical" provable, not just
visually similar.

Very impressive demo.

---

# Final Demo Scenario

Equivalent to:

```bash
docker compose stop node-a
```

in SentinelCache.

---

Demo:

```text
100 virtual users

3 replicas

20% packet loss

Replica B partitioned
```

System state:

```text
A = 50 nodes

B = 30 nodes

C = 48 nodes
```

---

Reconnect:

```text
Replica B
```

---

Observe:

```text
Vector clock exchange

Missing operation sync

CRDT merge

Convergence
```

---

Final state:

```text
A = 52 nodes

B = 52 nodes

C = 52 nodes
```

Graph identical everywhere.

---

# Completion Criteria

Project is complete when:

✓ 3 replicas running

✓ Operation-based CRDT implemented

✓ OR-Set implemented

✓ LWW registers implemented (HLC-ordered, no raw wall clock)

✓ Vector clocks implemented (used for sync + concurrency detection)

✓ Edge referential integrity (no dangling edges materialized)

✓ Convergence checker (canonical state hash) proves replicas equal

✓ Anti-entropy reconciliation self-heals dropped operations

✓ Replication working

✓ Packet loss simulation

✓ Network partition simulation

✓ Replica recovery

✓ 100 virtual users

✓ Real-time dashboard

✓ Docker Compose deployment

✓ Demonstrable eventual consistency

✓ Resume-worthy distributed systems project

