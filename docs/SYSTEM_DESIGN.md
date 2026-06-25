# SYSTEM_DESIGN.md

# SentinelSync

> Distributed State Synchronization Engine using Graph CRDTs

---

# 1. System Goal

The purpose of SentinelSync is to guarantee:

```text
Multiple replicas

+
Concurrent operations

+
Network failures

=
Eventually identical state
```

without:

```text
Central coordinator
Leader node
Global lock
Manual conflict resolution
```

Every replica should independently process operations and still converge to the same graph.

---

# 2. High-Level Architecture

System consists of:

```text
                 Web Dashboard
                        |
                        |
                  WebSocket API
                        |
                        |
      ------------------------------------
      |                |                |
      |                |                |
      ▼                ▼                ▼

  Replica A       Replica B       Replica C
```

Each replica contains:

```text
CRDT Engine

Graph State

Vector Clock

Operation Log

Replication Engine
```

Each replica is equal.

No leader exists.

No coordinator exists.

Every replica can:

* receive operations
* generate operations
* replicate operations
* merge operations

---

# 3. Why No Leader?

In SentinelCache:

```text
Leader
```

was necessary because:

```text
Failure detection
Failover
Membership changes
```

needed coordination.

In SentinelSync:

```text
CRDT
```

already guarantees convergence.

Therefore:

```text
Leader unnecessary
```

Removing the leader:

* simplifies architecture
* removes single point of failure
* demonstrates true distributed synchronization

---

# 4. State Model

SentinelSync synchronizes a graph.

Example:

```text
Email
  |
  v
AI Processor
  |
  v
Slack
```

State:

```go
type Graph struct {
    Nodes map[string]*Node
    Edges map[string]*Edge
}
```

---

# 5. Node Model

```go
type Node struct {
    ID string

    Title string

    X float64
    Y float64

    CreatedAt int64
}
```

Node operations:

```text
Create
Delete
Rename
Move
```

---

# 6. Edge Model

```go
type Edge struct {
    ID string

    Source string
    Target string
}
```

Edge operations:

```text
Create
Delete
```

---

# 7. Why Graph CRDT?

Alternative:

```text
Text CRDT
```

used by:

```text
Google Docs
Automerge
YJS
```

Rejected.

Reasons:

* harder frontend
* cursor synchronization
* selection synchronization
* formatting complexity

Graph CRDT provides:

* same consistency concepts
* simpler implementation
* better visualization

---

# 8. Operation-Based CRDT

Important design decision.

Two major CRDT categories exist.

---

## State-Based CRDT

Replica periodically sends:

```text
Entire state
```

Example:

```text
Replica A
→ Graph snapshot

Replica B
→ Graph snapshot
```

Then merge.

---

Advantages:

* simple

Disadvantages:

* large payloads
* less interesting
* weak replication story

---

Rejected.

---

## Operation-Based CRDT

Chosen.

Replicas send:

```text
CreateNode
RenameNode
DeleteNode
MoveNode
```

instead of:

```text
Entire graph
```

Advantages:

* efficient
* closer to real systems
* teaches distributed messaging

---

## Delivery Guarantees (Important)

A pure operation-based CRDT (CmRDT) is only correct if the transport layer
guarantees:

```text
Exactly-once delivery
Causal-order delivery
No permanent message loss
```

SentinelSync deliberately violates all three (latency, packet loss, partition,
crash). Naive "broadcast and apply" would therefore diverge permanently under
loss.

We resolve this honestly: SentinelSync is **not** a pure op-based CRDT. It is a
**hybrid**:

```text
Operation-based CRDT
+
Durable operation log (per replica)
+
Anti-entropy reconciliation (resync)
```

This is exactly how real systems (e.g. Riak) behave. The operation log + resync
turns an unreliable network into the reliable causal delivery the CRDT layer
assumes.

Concretely, the design relies on the following minimum properties, achieved by
the log + resync rather than by the network:

* **At-least-once** delivery via broadcast + retry.
* **Exactly-once application** via operation IDs (dedup at the apply layer).
* **Eventual causal completeness** via anti-entropy: any gap caused by loss or
  partition is closed when replicas exchange vector clocks and replay missing
  operations.

Anti-entropy is therefore a **first-class part of the engine**, not a recovery
afterthought. The packet-loss demo only converges because resync runs
continuously — see §25.

---

# 9. Operation Structure

Every change becomes an operation.

```go
type Operation struct {
    ID string

    ReplicaID string

    Type string

    Payload json.RawMessage

    // Used for anti-entropy / causality detection (see §11–12), NOT for merge.
    VectorClock map[string]int64

    // Hybrid Logical Clock timestamp. Drives LWW register conflict
    // resolution (see §17). NOT a raw wall clock — see §11.
    HLC HLCTimestamp
}

type HLCTimestamp struct {
    Physical int64  // wall-clock millis, monotonically capped
    Logical  int64  // tiebreaker counter for same-millisecond events
    ReplicaID string
}
```

> Note: `HLC` replaces the naive `Timestamp int64` used in early drafts. A raw
> wall clock would contradict §11 (clocks are unreliable). See §17.

---

# 10. Why Operation IDs?

Suppose:

```text
Network retries
```

occur.

Replica may receive:

```text
Same operation twice
```

Without IDs:

```text
Duplicate application
```

possible.

Operation IDs provide:

```text
Exactly-once semantics
```

at application layer.

---

# 11. Vector Clocks

One of the most important concepts.

---

Problem:

How do we know whether:

```text
Operation A

happened before

Operation B
```

?

Wall clock timestamps are unreliable.

Machines may have:

```text
Clock skew
```

---

Solution:

Vector Clocks.

---

Replica A:

```text
A=5
B=2
C=1
```

Replica B:

```text
A=4
B=8
C=1
```

These clocks allow us to determine:

```text
causal relationship
```

between events.

---

## What Vector Clocks Are (and Are Not) Used For

This is a common point of confusion, so state it plainly.

Vector clocks are **not** used to merge state. The CRDT merge logic needs none
of them:

* OR-Set converges via unique add/remove **tags** (§13).
* LWW register converges via **HLC timestamp + ReplicaID** tiebreak (§17).

Vector clocks earn their place in two other roles:

1. **Anti-entropy / sync (§25):** comparing two replicas' vector clocks tells us
   exactly which operations a peer is missing after loss or partition.
2. **Concurrency detection (UI):** distinguishing "A happened-before B" from
   "A concurrent with B" so the dashboard timeline can label concurrent edits.

If you find yourself reaching for the vector clock inside a merge function, stop
— that decision belongs to the OR-Set tag or the HLC.

---

# 12. Why Vector Clocks?

Without vector clocks:

```text
Concurrent operations
```

become difficult to detect.

We need to distinguish:

```text
Operation happened before
```

from

```text
Operation happened concurrently
```

CRDT conflict resolution depends on this distinction.

---

# 13. OR-Set

Used for:

```text
Nodes
Edges
```

---

Problem:

Suppose:

```text
Add Node

Delete Node

Add Node again
```

Simple sets break.

---

Solution:

Observed Remove Set.

Each addition receives:

```text
unique tag
```

Example:

```text
Node A

tag1
tag2
tag3
```

Delete only removes observed tags.

This prevents:

```text
Zombie resurrection bugs
```

common in distributed systems.

---

## OR-Set Memory Growth (Known Limitation)

An OR-Set never truly forgets. Every add stores a tag; every delete stores a
remove-tag (a tombstone). Under a stress run of 100 users × 1000 operations,
these sets grow **unbounded** — the OR-Set only grows, it never shrinks.

Production CRDTs prune tombstones once an operation is *causally stable* (every
replica has seen it, so no future delete can reference an older tag). SentinelSync
does **not** implement causal-stability GC in V1.

Decision: accept unbounded growth in V1, and **turn it into a teaching artifact**.
The dashboard exposes a "set size vs operations applied" metric (§30) so the
growth is visible — making concrete *why* real systems need tombstone GC.
Causal-stability pruning is listed as a V2 enhancement (§32).

---

## 13a. Edge Referential Integrity

Nodes and edges are two independent OR-Sets. Each converges correctly on its
own — but convergence of the two sets independently does **not** preserve the
cross-object invariant "an edge references live nodes".

The classic concurrency case:

```text
Replica A:  addEdge(X → Y)
Replica B:  deleteNode(Y)      (concurrent)
```

After merge, both operations survive: the edge OR-Set contains `X → Y`, but the
node OR-Set no longer contains `Y`. The result is a **dangling edge** pointing at
a node that does not exist. This is the signature bug of graph CRDTs.

Decision: **edges are derived against live nodes.** An edge is considered present
only if both endpoints are present in the materialized node set. Specifically:

1. The edge OR-Set still stores `X → Y` (we do not mutate it — that would not
   converge, and `Y` could legitimately be re-added later).
2. When **materializing** the graph (for rendering, hashing, or metrics), we
   filter out any edge whose source or target is not a live node.
3. If `Y` is later re-added (new tag), the edge `X → Y` becomes visible again
   automatically — no special resurrection logic needed.

This keeps both OR-Sets pure (add/remove only) while enforcing the invariant at
the read/materialization boundary. The convergence checker (§29a) hashes the
*materialized* graph, so dangling edges never affect convergence comparisons.

---

# 14. Node Creation

Operation:

```json
{
  "type":"create_node",
  "nodeId":"123"
}
```

Process:

1. Add node tag to OR-Set
2. Broadcast operation
3. Merge on peers

Result:

```text
Node exists everywhere
```

---

# 15. Node Deletion

Operation:

```json
{
  "type":"delete_node",
  "nodeId":"123"
}
```

Process:

1. Mark OR-Set remove tag
2. Broadcast
3. Merge

Result:

```text
Node disappears everywhere
```

Eventually.

---

# 16. Node Rename

Problem:

Two users rename simultaneously.

Replica A:

```text
GPT Processor
```

Replica B:

```text
LLM Processor
```

Which wins?

---

Solution:

LWW Register.

---

# 17. LWW Register

Last Write Wins Register.

Stores:

```go
type LWW struct {
    Value string

    // Hybrid Logical Clock, NOT a raw wall clock. See below.
    HLC HLCTimestamp
}
```

---

## Why HLC and not a wall clock

This is the most important correctness fix in the design.

§11 argued that wall-clock timestamps are unreliable because of clock skew —
that was the entire justification for vector clocks. It would be a direct
contradiction to then resolve rename/move conflicts with a raw `time.Now()`:
under skew, the replica with the fastest clock would silently win every rename,
non-deterministically.

So the LWW register is ordered by a **Hybrid Logical Clock (HLC)**:

```text
HLC = (physical, logical, replicaID)
```

* `physical` tracks wall time but is **monotonic** and bounded — on receiving a
  remote op, a replica advances its HLC to `max(local, remote) (+1 logical)`,
  so it never moves backward and never trails a peer it has heard from.
* `logical` breaks ties within the same millisecond.
* `replicaID` is the final deterministic tiebreaker.

HLC gives us human-meaningful timestamps (close to wall time, good for the UI)
while remaining causally consistent (a cause never out-ranks its effect on the
same replica's view).

---

Conflict:

```text
Rename A

Rename B
```

Resolution (compared in order):

```text
1. Higher HLC.physical wins
2. else higher HLC.logical wins
3. else higher ReplicaID (lexical) wins
```

Deterministic. Independent of physical clock skew between machines.

---

# 18. Node Position

Position uses same strategy.

```go
X
Y
HLC   // same Hybrid Logical Clock ordering as §17
```

Move conflicts resolved via:

```text
LWW Register (HLC-ordered)
```

Note this is lossy by design: two concurrent drags converge to one position and
the other is discarded. That is the accepted, Figma-like tradeoff for position —
we do not merge coordinates.

---

# 19. Replication Model

Each replica maintains:

```text
WebSocket connections
```

to peers.

Operations propagate:

```text
CreateNode
```

↓

```text
Broadcast
```

↓

```text
All replicas
```

↓

```text
Apply
```

↓

```text
Converge
```

---

# 20. Network Simulation

Core feature.

Dashboard can inject:

```text
Latency

Packet Loss

Partition

Crash
```

---

# 21. Latency Injection

Example:

```text
200ms
```

Every outgoing message delayed.

Allows observation of:

```text
Temporary divergence
```

---

# 22. Packet Loss

Example:

```text
20%
```

Randomly drop messages.

Observe:

```text
Missing operations
```

until synchronization occurs.

---

# 23. Network Partition

Example:

```text
Replica B isolated
```

System becomes:

```text
A ↔ C

B isolated
```

Users continue editing.

When B rejoins:

```text
Automatic convergence
```

occurs.

---

# 24. Replica Crash

There are **two distinct failure models** and they teach different things. Do
not conflate them — keep them as separate dashboard controls.

---

## (a) Soft crash — really a partition

Dashboard button: "Isolate Replica B".

Implementation: stop sending/receiving, but the process keeps running and keeps
its in-memory state (graph + operation log).

```text
Effect: B stops exchanging operations
State: fully retained in memory
```

On recovery, B still holds everything it had; only the *gap* accumulated during
isolation needs to be reconciled. This is the **partition** scenario (§23) and
recovery is cheap. (We label this honestly as a partition, not a crash.)

---

## (b) Hard crash — process and state lost

```bash
docker compose stop replica-b
```

The container dies. Because the operation log is **in memory only** in V1 (§26),
B loses its entire log and graph.

```text
Effect: process gone
State: lost entirely
```

On restart:

```bash
docker compose start replica-b
```

B comes up **empty** and must perform a **full catch-up** from its peers — not a
delta. Its vector clock is zero, so anti-entropy (§25) streams the entire
operation history from A and C and B replays it from scratch.

> Caveat: if **all** replicas hard-crash at once, the in-memory log is gone
> everywhere and state is unrecoverable in V1. A persistent log (§32, V2) removes
> this limitation. Peer-based recovery only works while at least one replica
> survives.

---

# 25. Synchronization Strategy (Anti-Entropy)

This is the engine's reliability layer (see §8 — Delivery Guarantees). It runs
both periodically (background anti-entropy) and on reconnect.

When two replicas reconcile:

1. Exchange vector clocks
2. Determine missing operations (the diff between the two clocks)
3. Request missing operations
4. Replay operation log (operations are idempotent via op ID, so replay is safe)
5. Reach convergence

The same protocol covers **both** recovery cases from §24:

* **Partition rejoin / soft crash:** the clock diff is small → a delta sync.
* **Hard crash restart:** the recovering replica's clock is zero → the diff is
  the *entire* history → a full catch-up.

Because the protocol is driven purely by the vector-clock diff, both cases use
one code path; "full catch-up" is just "delta sync where the delta is
everything".

---

# 26. Why Operation Log?

Without operation log:

```text
Lost operations
```

cannot be recovered.

Each replica stores:

```go
[]Operation
```

in memory (V1). Acts like:

```text
mini event store
```

Two consequences follow from "in memory", both addressed elsewhere:

* It is volatile — a hard crash loses it, forcing full catch-up from peers (§24).
  A persistent log is the first V2 item (§32).
* It only grows, like the OR-Set tombstones (§13). Operation-log size is another
  visualized metric (§30), and bounded retention via causal stability is V2.

---

# 27. Dashboard Design

Purpose:

Observe.

Not edit documents.

---

Layout:

```text
+-----------------------------------+

Simulation Controls

Replica Controls

Operation Controls

+-----------------------------------+

Graph Visualization

+-----------------------------------+

Replica Status

+-----------------------------------+

Operation Timeline

+-----------------------------------+
```

---

# 28. Simulation Engine

Instead of opening:

```text
50 browsers
```

system creates:

```text
virtual users
```

inside backend.

Example:

```text
100 users

1000 operations
```

generated automatically.

This allows stress testing.

---

# 29. Why Simulated Users?

Much more valuable than:

```text
Open 3 browser tabs
```

because we can test:

```text
50 users
100 users
500 users
```

easily.

---

# 29a. Convergence Verification (Core)

You cannot *prove* eventual consistency by eyeballing three graphs. The system
needs a programmatic oracle, and it is a first-class feature — built in Phase 3,
before any chaos is introduced.

Mechanism:

```text
1. Canonicalize each replica's MATERIALIZED graph
   - live nodes only (OR-Set resolved)
   - live edges only (dangling edges filtered, §13a)
   - LWW fields resolved (title, position)
   - nodes/edges sorted by ID (stable order)

2. Hash the canonical form -> stateHash

3. After quiescence, assert:
   stateHash(A) == stateHash(B) == stateHash(C)
```

This hash is reused two ways:

* As the **test oracle**: convergence tests assert all hashes equal once the
  operation queues drain.
* As the **divergence metric** (§30): during a partition the hashes differ;
  "divergence count" = number of replicas not matching the majority hash, and
  "convergence time" = time from last operation until all hashes equal.

The same canonicalization feeds the convergence visualizer (Phase 8) — the
"50 / 47 / 52 → 50 / 50 / 50" demo is literally these hashes (and node counts)
converging.

---

# 30. Metrics

Track:

```text
Operations/sec

Convergence Time        (from §29a hash equality)

Replica Divergence      (count of replicas off the majority hash, §29a)

Replication Lag

Messages/sec

Merge Time

OR-Set size / replica   (tombstone growth, §13)

Operation-log size      (unbounded-growth visualization, §26)
```

These become interview discussion points. The last two exist specifically to
make the V1 "never forgets" limitation visible and concrete.

---

# 31. Demonstration Scenario

Equivalent to:

```bash
docker compose stop node-a
```

in SentinelCache.

Demo:

```text
100 users

20% packet loss

Replica B partitioned
```

Continue editing.

Reconnect B.

Observe:

```text
Replica B catches up

All replicas converge

Graph identical everywhere
```

This proves:

```text
Eventual Consistency
```

visually.

---

# 32. Future Enhancements

V2:

* Persistent event log (removes the all-replicas-crash data loss, §24)
* Causal-stability GC — prune OR-Set tombstones and the operation log once an
  operation is observed by every replica (bounds the growth from §13 and §26)
* Snapshotting
* Prometheus metrics
* Grafana dashboards

V3:

* Text CRDT
* Rich text editing
* Offline browser clients
* Multi-region replication

---

# 33. Success Criteria

Project considered complete when:

✓ 3 replicas running

✓ Workflow graph synchronized

✓ Concurrent operations supported

✓ Vector clocks implemented

✓ OR-Set implemented

✓ LWW registers implemented (HLC-ordered)

✓ Convergence checker (canonical state hash) verifies all replicas equal

✓ Edge referential integrity (no dangling edges in materialized graph)

✓ Network partition simulation works

✓ Replica recovery works

✓ Dashboard visualizes convergence

✓ 100 concurrent simulated users supported

✓ Docker Compose demo reproducible

