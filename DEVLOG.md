# SentinelSync — Build Log & Concept Journal

Every file, every decision, every concept — in the order we built them.
Append to this file as the project grows.

This is the consistency-focused sibling of SentinelCache. Where SentinelCache
asked *"how does a cluster stay available when nodes die?"*, SentinelSync asks
*"how do replicas that edited independently end up identical?"*

---

## Table of Contents

1. [Project Overview](#1-project-overview)
2. [Design Corrections Before Code](#2-design-corrections-before-code)
3. [go.mod — The Module File](#3-gomod--the-module-file)
4. [Directory Structure](#4-directory-structure)
5. [internal/graph/node.go — Node Model](#5-internalgraphnodego--node-model)
6. [internal/graph/edge.go — Edge Model](#6-internalgraphedgego--edge-model)
7. [internal/graph/graph.go — The Graph Engine](#7-internalgraphgraphgo--the-graph-engine)
8. [internal/api/handlers.go — REST Handlers (Gin)](#8-internalapihandlersgo--rest-handlers-gin)
9. [internal/api/routes.go — Route Table](#9-internalapiroutesgo--route-table)
10. [cmd/replica/main.go — Entry Point](#10-cmdreplicamaingo--entry-point)
11. [Makefile — Developer Workflow](#11-makefile--developer-workflow)
12. [internal/graph/graph_test.go — Tests](#12-internalgraphgraph_testgo--tests)
13. [Phase 1 — Demo Output](#13-phase-1--demo-output)
14. [Known Limitations & What Phase 2 Adds](#14-known-limitations--what-phase-2-adds)

### Phase 2 — Replica Architecture

15. [internal/crdt — Type Scaffolding](#15-internalcrdt--type-scaffolding)
16. [internal/replica/replica.go — The Replica](#16-internalreplicareplicago--the-replica)
17. [Wiring: main.go, PEERS, and richer /status](#17-wiring-maingo-peers-and-richer-status)
18. [Dockerfile & docker-compose.yml — The 3-Replica Cluster](#18-dockerfile--docker-composeyml--the-3-replica-cluster)
19. [Phase 2 — Demo Output (Divergence Without Sync)](#19-phase-2--demo-output-divergence-without-sync)
20. [Phase 2 — Known Limitations & What Phase 3 Adds](#20-phase-2--known-limitations--what-phase-3-adds)

### Phase 3 — CRDT Engine (the core)

21. [The Shift: State Mutates Only By Operations](#21-the-shift-state-mutates-only-by-operations)
22. [crdt/hlc.go — Hybrid Logical Clock](#22-crdthlcgo--hybrid-logical-clock)
23. [crdt/vector_clock.go — Increment / Merge / Compare](#23-crdtvector_clockgo--increment--merge--compare)
24. [crdt/orset.go — Add-Wins Observed-Remove Set](#24-crdtorsetgo--add-wins-observed-remove-set)
25. [crdt/lww.go — HLC-Ordered LWW Register](#25-crdtlwwgo--hlc-ordered-lww-register)
26. [graph: State, Apply, Materialize, Convergence Hash](#26-graph-state-apply-materialize-convergence-hash)
27. [replica: Operation Generation & Ingest](#27-replica-operation-generation--ingest)
28. [Convergence Tests — The Proof](#28-convergence-tests--the-proof)
29. [Phase 3 — Demo Output (Dangling Edge Round-Trip)](#29-phase-3--demo-output-dangling-edge-round-trip)
30. [Phase 3 — Known Limitations & What Phase 4 Adds](#30-phase-3--known-limitations--what-phase-4-adds)

### Phase 4 — Replication Layer

31. [The Reliability Problem With Op-Based CRDTs](#31-the-reliability-problem-with-op-based-crdts)
32. [internal/transport/transport.go — The WebSocket Manager](#32-internaltransporttransportgo--the-websocket-manager)
33. [internal/replica/replica.go — Gap-Aware Vector Clock](#33-internalreplicareplicago--gap-aware-vector-clock)
34. [internal/transport/transport_test.go — Live Convergence Tests](#34-internaltransporttransport_testgo--live-convergence-tests)
35. [internal/replica/replica_test.go — Anti-Entropy Gap-Aware Test](#35-internalreplicareplica_testgo--anti-entropy-gap-aware-test)
36. [Wiring: cmd/replica/main.go + go.mod](#36-wiring-cmdreplicamaingo--gomod)
37. [Phase 4 — Demo Output (Live 3-Replica Convergence)](#37-phase-4--demo-output-live-3-replica-convergence)
38. [Phase 4 — Known Limitations & What Phase 5 Adds](#38-phase-4--known-limitations--what-phase-5-adds)

### Phase 5 — Network Simulation

39. [The Problem With a Perfect Network](#39-the-problem-with-a-perfect-network)
40. [internal/simulation/chaos.go — The Fault Injector](#40-internalsimulationchaosgo--the-fault-injector)
41. [Transport Integration — Checking Chaos on Every Send/Receive](#41-transport-integration--checking-chaos-on-every-sendreceive)
42. [internal/api/sim.go — /sim REST Endpoints](#42-internalapisimgo--sim-rest-endpoints)
43. [Chaos Tests](#43-chaos-tests)
44. [Phase 5 — Demo Output (Partition + Recovery)](#44-phase-5--demo-output-partition--recovery)
45. [Phase 5 — Known Limitations & What Phase 6 Adds](#45-phase-5--known-limitations--what-phase-6-adds)

### Phase 6 — Simulated Users

46. [Why Virtual Users](#46-why-virtual-users)
47. [internal/simulation/simulator.go — The Virtual User Engine](#47-internalsimulationsimulatorgo--the-virtual-user-engine)
48. [API and Wiring](#48-api-and-wiring)
49. [Why Stop Waits (The WaitGroup Bug)](#49-why-stop-waits-the-waitgroup-bug)
50. [Phase 6 Tests](#50-phase-6-tests)
51. [Phase 6 — Demo Output (100 Users Under Load)](#51-phase-6--demo-output-100-users-under-load)
52. [Phase 6 — Known Limitations & What Phase 7 Adds](#52-phase-6--known-limitations--what-phase-7-adds)

---

## 1. Project Overview

**What we are building:**
SentinelSync — a distributed state synchronization engine in Go. Multiple
replicas edit a shared **workflow graph** (nodes + edges) concurrently and
converge to identical state with **no leader and no central coordinator**, even
under latency, packet loss, and network partitions.

**The problem it solves:**
Concurrent edits to shared state are everywhere — Google Docs, Figma, Notion,
distributed databases. The naive answer (Last Write Wins on the whole document)
loses data. SentinelSync uses CRDTs (Conflict-free Replicated Data Types) so
replicas can diverge, reconnect, and merge *without* losing user changes.

**Why a graph and not a text editor:**
A text CRDT drags in cursors, selections, and rich-text rendering — a frontend
project wearing a distributed-systems costume. A node/edge graph exposes the same
convergence problems (concurrent add/remove/rename/move) while staying visual and
simple. The frontend exists only to *observe* distributed behavior, like Grafana
or Kafka UI — not to be a product.

**Why this is a good resume project:**
It demonstrates CRDTs, vector clocks, hybrid logical clocks, eventual
consistency, anti-entropy reconciliation, and partition recovery — the
consistency half of distributed systems. The demo (partition a replica, keep
editing, reconnect, watch every replica's state hash collapse to one value) is
the interview story. Together with SentinelCache it covers both pillars:
**availability** and **consistency**.

**Communication model (key architectural decision):**

| Layer | Protocol | Why |
|---|---|---|
| Client / dashboard → Replica | REST (Gin) | Human-readable, curl-friendly |
| Replica → Replica (Phase 4) | WebSocket | Persistent, streams operations between peers |

Same split philosophy as SentinelCache: a simple external API, a purpose-built
internal transport. In Phase 1 only the REST layer exists.

---

## 2. Design Corrections Before Code

Before writing a line of Go we hardened the design docs (`docs/`). Seven issues
were fixed; four of them directly shape how the code in later phases is written,
so they are recorded here:

| Issue | Resolution | Why it matters to the code |
|---|---|---|
| LWW used a raw wall-clock timestamp | Switch to a **Hybrid Logical Clock (HLC)** | A raw `time.Now()` lets the fastest-clock replica win every rename. `CreatedAt` in `node.go` is therefore metadata only — never a tiebreaker. |
| "Op-based CRDT" + packet loss is a contradiction | Documented the real model: **op-based CRDT + operation log + anti-entropy** | Phase 4 builds anti-entropy as a first-class loop, not a recovery afterthought. |
| Dangling edges under concurrency | Edges are filtered against live nodes **at materialization**, OR-Sets stay pure | Phase 1's eager "both endpoints must exist" check is explicitly a single-replica shortcut that gets *replaced*, not extended. |
| What vector clocks are for | Sync + concurrency detection, **not** merge | Stops us from reaching for the vector clock inside merge functions in Phase 3. |

The other three (convergence checker as a first-class test oracle, soft-crash vs
hard-crash distinction, OR-Set tombstone growth as a visualized limitation) shape
Phases 3–5. Full detail in `docs/SYSTEM_DESIGN.md` and `docs/IMPLEMENTATION_PLAN.md`.

**Lesson:** correcting the design on paper is far cheaper than discovering a
clock-skew bug after three replicas are wired together.

---

## 3. `go.mod` — The Module File

**File:** `go.mod`

```
module github.com/aryan-mishra/sentinel-sync

go 1.25.0

require github.com/gin-gonic/gin v1.12.0
```

**What it does:**
- Declares the module path — the prefix for every internal import
  (`github.com/aryan-mishra/sentinel-sync/internal/graph`).
- Pins the Go version and the one direct dependency we need in Phase 1: Gin.

**Why only Gin so far:**
Phase 1 is a local engine + REST API. No gRPC, no WebSocket, no protobuf yet —
those arrive when replicas start talking to each other (Phase 4). We add
dependencies exactly when a phase needs them, keeping the early build lean.

---

## 4. Directory Structure

```
sentinel-sync/
├── cmd/
│   └── replica/
│       └── main.go          ← binary entry point (one replica)
├── internal/
│   ├── graph/
│   │   ├── node.go          ← Node model
│   │   ├── edge.go          ← Edge model
│   │   ├── graph.go         ← in-memory graph engine (the Phase 1 core)
│   │   └── graph_test.go
│   └── api/
│       ├── handlers.go      ← Gin REST handlers
│       └── routes.go        ← route table
├── docs/
│   ├── BLUEPRINT.md
│   ├── SYSTEM_DESIGN.md
│   └── IMPLEMENTATION_PLAN.md
├── Makefile
├── go.mod
├── go.sum
├── README.md
└── DEVLOG.md                ← this file
```

**`internal/` convention:**
The Go compiler forbids anything outside this module from importing packages
under `internal/`. It is a compiler rule, not just a naming habit — nobody can
accidentally depend on SentinelSync's internals as a library.

**`cmd/replica` (not `cmd/node`):**
SentinelCache called its binary a *node*; SentinelSync calls it a *replica*. The
vocabulary is intentional — in a CRDT system every process is an equal replica of
the same logical state, with no leader. Naming the binary `replica` keeps that
"no special node" idea front and center.

**Why `graph/` is a package of its own:**
The graph engine is pure domain logic — it knows nothing about HTTP, JSON, or
networking. Keeping it isolated means Phase 3 can wrap each field in a CRDT type
without touching the API layer, and the engine stays unit-testable with no server
running.

---

## 5. `internal/graph/node.go` — Node Model

**File:** `internal/graph/node.go`

A node is one vertex in the workflow graph:

```go
type Node struct {
    ID        string
    Title     string
    X, Y      float64
    CreatedAt int64   // wall-clock millis — METADATA ONLY
}
```

**The field layout anticipates the CRDT split.**
In Phase 3 the fields stop being plain values:
- *Presence* (does this node exist?) → an **OR-Set** membership with unique tags.
- `Title` → an **HLC-ordered LWW register**.
- `X, Y` → a single **LWW register** (so two concurrent drags resolve to one
  position, deterministically).

Laying the struct out this way now means the Phase 3 refactor is mechanical.

**Why `CreatedAt` must never resolve conflicts.**
This is the single most important comment in the file. `CreatedAt` is a wall
clock. SYSTEM_DESIGN §11 rejects wall clocks for ordering because of clock skew —
that was the whole reason for HLCs. If we ever broke a rename tie with
`CreatedAt`, we would silently reintroduce the exact bug we designed around. It
exists for display ("created 3 minutes ago") and nothing else.

---

## 6. `internal/graph/edge.go` — Edge Model

**File:** `internal/graph/edge.go`

```go
type Edge struct {
    ID     string
    Source string
    Target string
}
```

An edge is a directed reference between two node IDs. Simple struct — the
interesting part is the **invariant**, and how that invariant changes across
phases.

**Phase 1 (single replica): eager enforcement.**
- An edge can only be created if both endpoints currently exist.
- Deleting a node *cascades* to every edge touching it.

This keeps the single-replica graph always-consistent: there is never a dangling
edge because there is no concurrency to create one.

**Phase 3 (CRDTs): the invariant moves to read time.**
Once nodes and edges are independent OR-Sets, a replica can `addEdge(X→Y)` while
another concurrently `deleteNode(Y)`. Both operations are valid and both survive
the merge — leaving an edge pointing at a node that no longer exists. This is the
signature graph-CRDT bug (SYSTEM_DESIGN §13a). The fix is *not* to cascade-delete
(that wouldn't converge, and `Y` might be re-added). Instead:
- The edge OR-Set stays pure (add/remove only).
- When the graph is **materialized** (for rendering, hashing, metrics), edges
  whose endpoints aren't live are filtered out.
- If `Y` is re-added later, the edge reappears automatically.

So Phase 1 deliberately builds the *happy path* that Phase 3 will make converge.
The cascade is a shortcut with a known expiry date, documented in the code.

---

## 7. `internal/graph/graph.go` — The Graph Engine

**File:** `internal/graph/graph.go`

This is the Phase 1 core: an in-memory graph mutated under a lock.

### Data structure

```go
type Graph struct {
    mu    sync.RWMutex
    nodes map[string]*Node
    edges map[string]*Edge
    now   func() time.Time   // injected clock, for tests
}
```

Two maps keyed by ID give O(1) lookup, insert, and delete for both nodes and
edges — all we need at this stage.

### Why `sync.RWMutex` here (the opposite choice from SentinelCache's cache)

SentinelCache's LRU used a plain `sync.Mutex` because its `Get` was secretly a
write (it moved the accessed item to the front of the list). SentinelSync's read
path is the opposite: `Snapshot` genuinely only reads. Several goroutines will
call it concurrently (the REST handler, and later the dashboard stream and the
convergence checker), and they should not block each other. So `RWMutex` —
many readers *or* one writer — is the correct fit. Same reasoning, opposite
conclusion, because the read path's true nature differs.

### Operations

| Method | Behavior | Becomes (Phase 3) |
|---|---|---|
| `CreateNode` | insert; error if ID exists | OR-Set add (unique tag) |
| `RenameNode` | set title; error if missing | LWW-register write (HLC) |
| `MoveNode` | set X,Y; error if missing | LWW-register write (HLC) |
| `DeleteNode` | delete + **cascade edges** | OR-Set remove |
| `CreateEdge` | insert; both endpoints must exist | OR-Set add |
| `DeleteEdge` | delete; error if missing | OR-Set remove |

### Returning clones, not live pointers

Every method that hands a node/edge back to the caller returns a **copy**:

```go
func (n *Node) clone() *Node { c := *n; return &c }
```

If we returned the live `*Node`, the API layer (or any caller) could mutate
engine state without holding the lock — a data race waiting to happen. Returning
copies makes the lock the *only* path to mutation. `Snapshot` does the same for
the whole graph. This is the Phase 1 stand-in for the "materialize" step that
Phase 3 formalizes.

### Sentinel errors

The engine returns typed errors (`ErrNodeNotFound`, `ErrNodeExists`,
`ErrEndpointMissing`, …) rather than HTTP codes. The engine knows nothing about
HTTP; the API layer translates these to status codes with `errors.Is`. This keeps
the domain layer transport-agnostic — it could later be driven by a WebSocket or
a test harness instead of Gin with zero changes.

### The injected clock

`now func() time.Time` lets tests supply a deterministic time and lets Phase 3
swap in an HLC without rewriting call sites. A small seam now, a big convenience
later.

---

## 8. `internal/api/handlers.go` — REST Handlers (Gin)

**File:** `internal/api/handlers.go`

**Why Gin (same reasons as SentinelCache):**
- `c.ShouldBindJSON(&req)` — body parsing + validation (`binding:"required"`) in
  one line.
- `c.JSON(...)` — clean responses, no boilerplate.
- `c.Param("id")` — readable path params.

**Validation at the boundary.**
Request structs use `binding:"required"` so a missing `id` or `title` is rejected
with `400` before the engine is ever touched. The engine assumes well-formed
input; the handler is the gatekeeper.

**Error mapping — one place, `errors.Is`:**

```go
func (h *Handler) writeErr(c *gin.Context, err error) {
    switch {
    case errors.Is(err, graph.ErrNodeNotFound), errors.Is(err, graph.ErrEdgeNotFound):
        c.JSON(404, ...)
    case errors.Is(err, graph.ErrNodeExists), errors.Is(err, graph.ErrEdgeExists):
        c.JSON(409, ...)   // Conflict
    case errors.Is(err, graph.ErrEndpointMissing):
        c.JSON(422, ...)   // Unprocessable Entity
    default:
        c.JSON(500, ...)
    }
}
```

Mapping in a single helper means every handler reports errors consistently, and
adding an error type later touches exactly one function. `409 Conflict` for
"already exists" and `422` for "edge endpoint missing" are deliberate, accurate
HTTP semantics — not everything is a `400`.

**The handler holds the replica ID** so `/status` can report which replica
answered. In Phase 2+ this becomes essential: every response will say which
replica served it, which is how the dashboard shows divergence.

---

## 9. `internal/api/routes.go` — Route Table

**File:** `internal/api/routes.go`

A resource-oriented layout:

| Method | Path | Purpose |
|---|---|---|
| `GET` | `/health` | Liveness probe |
| `GET` | `/status` | Replica ID + node/edge counts |
| `GET` | `/graph` | Full graph snapshot |
| `POST` | `/node` | Create node |
| `PATCH` | `/node/:id/title` | Rename |
| `PATCH` | `/node/:id/position` | Move |
| `DELETE` | `/node/:id` | Delete node (cascades edges) |
| `POST` | `/edge` | Create edge |
| `DELETE` | `/edge/:id` | Delete edge |

**Why `PATCH` for rename/move, not `POST` or `PUT`:**
Rename and move are *partial* updates to an existing node — exactly what `PATCH`
means. Splitting them into `/node/:id/title` and `/node/:id/position` (rather than
one "update node" endpoint) maps each route 1:1 to a future CRDT operation:
`RenameNode` and `MoveNode` will become distinct operations on the wire in Phase
3, each carrying its own HLC. The API shape already mirrors the operation model.

**Routing logic is intentionally absent.**
Every later phase adds more routers under the same `RegisterRoutes` seam
(`/sim`, `/network`, replication endpoints). For now each replica is a standalone
island serving its own graph.

---

## 10. `cmd/replica/main.go` — Entry Point

**File:** `cmd/replica/main.go`

Its only job: read config, wire components, start the server.

```go
g := graph.New()
h := api.NewHandler(replicaID, g)
r := gin.Default()
h.RegisterRoutes(r)
r.Run(restAddr)
```

**Why environment variables for config:**
Docker Compose (Phase 2) will start `replica-a`, `replica-b`, `replica-c` from a
single image, differing only by env vars. No flags, no config files, no
rebuilds — exactly the pattern SentinelCache used.

| Variable | Default | Purpose |
|---|---|---|
| `REPLICA_ID` | `replica-a` | Identity of this replica |
| `REST_ADDR` | `:8080` | Client/dashboard REST address |

Phase 2 adds `PEERS` / `SEED` style variables once replicas need to find each
other.

---

## 11. `Makefile` — Developer Workflow

**File:** `Makefile`

| Target | What it does |
|---|---|
| `make build` | Compile to `bin/replica` |
| `make run` | Run one replica locally |
| `make test` | `go test -race ./...` |
| `make tidy` | `go mod tidy` |
| `make clean` | Remove `bin/` |

**Why `-race` is the default test target.**
The graph engine's entire correctness story in Phase 1 is "the lock protects the
maps." The Go race detector is the tool that *proves* it — a concurrent test
(below) run under `-race` will flag any unsynchronized access. Making it the
default means we never accidentally test without it.

---

## 12. `internal/graph/graph_test.go` — Tests

**File:** `internal/graph/graph_test.go`

| Test | What it verifies |
|---|---|
| `TestCreateAndSnapshot` | Create works; duplicate ID → `ErrNodeExists`; snapshot reflects state |
| `TestRenameAndMove` | Rename/move mutate the node; missing ID → `ErrNodeNotFound` |
| `TestEdgeRequiresEndpoints` | Edge to a missing node → `ErrEndpointMissing`; succeeds once both exist |
| `TestDeleteNodeCascadesEdges` | Deleting a node removes its edges (Phase 1 eager invariant) |
| `TestConcurrentWrites` | 50 goroutines hammering the engine — must be clean under `-race` |

`TestConcurrentWrites` is the important one: it is the executable proof that the
`RWMutex` actually serializes writers. It would catch a forgotten `Lock()` the
moment it's introduced.

---

## 13. Phase 1 — Demo Output

Single replica on `:8099`, driven with curl:

```
== health ==
{"ok":true}

== create nodes ==
{"id":"1","title":"Email","x":0,"y":0,"createdAt":...}
{"id":"2","title":"AI Processor","x":100,"y":100,"createdAt":...}

== edge ==
{"id":"e1","source":"1","target":"2"}

== rename ==
{"id":"2","title":"GPT Processor",...}

== move ==
{"id":"1","title":"Email","x":400,"y":300,...}

== bad edge (missing endpoint) ==
{"error":"edge endpoint node does not exist"}     ← 422, invariant enforced

== graph ==
{"nodes":[{"id":"1",...},{"id":"2",...}],"edges":[{"id":"e1",...}]}

== delete node 1 (cascades e1) ==
{"ok":true}

== status ==
{"edges":0,"nodes":1,"replicaId":"replica-a"}     ← edge cascaded away with node 1
```

A complete local graph editor: create / rename / move / delete nodes, create /
delete edges, endpoint validation, and cascade delete — all behind a REST API,
all race-clean.

---

## 14. Known Limitations & What Phase 2 Adds

These are limitations *by design* for Phase 1 — each is removed by a specific
later phase.

| Limitation | Impact | Removed in |
|---|---|---|
| One replica only | No distribution, no sync | Phase 2 (3 replicas) |
| State is in-memory | Restart loses everything | persistent log is a V2 item |
| Eager edge invariant (cascade + endpoint check) | Won't converge under concurrency | Phase 3 (materialization filter) |
| Plain fields, no CRDT | Concurrent edits would clobber | Phase 3 (OR-Set + HLC-LWW) |
| No causal metadata | Can't tell concurrent from causal | Phase 3 (vector clocks) |

**Next: Phase 2 — Replica Architecture.** Stand up three independent replicas
(`replica-a/b/c`) via Docker Compose, each with its own graph, operation log, and
vector clock — still *without* synchronization. That sets the stage for Phase 3,
where the CRDT engine makes them converge.

---

# Phase 2 — Replica Architecture

**Goal:** stand up three independent replicas (`replica-a/b/c`) from one image
via Docker Compose. Each has its own graph, its own (empty) operation log, and
its own (zeroed) vector clock. **There is still no synchronization** — the point
of this phase is to build the cluster topology and *demonstrate divergence*, so
Phase 3's CRDT engine has something concrete to make converge.

---

## 15. `internal/crdt` — Type Scaffolding

**Files:** `internal/crdt/vector_clock.go`, `internal/crdt/operation.go`

Phase 2 introduces the `crdt` package, but **only the type definitions** — no
merge behavior. This is a deliberate discipline: define the shapes the `Replica`
struct and `/status` need now, defer all the logic to Phase 3 where it belongs.

**`VectorClock` (`map[string]uint64`):**
Maps each replica ID to how many operations that replica has originated, as
observed here. It exists in Phase 2 purely so a fresh replica can report a
complete, comparable clock (`a=0,b=0,c=0`). The behavior that matters —
`Increment`, `Merge`, `Compare` — is Phase 3. The package comment restates the
rule we corrected in the docs: **vector clocks are not used to merge state**
(OR-Set tags and the HLC do that); they exist for anti-entropy diffs and
concurrency detection.

**`Operation` + `HLCTimestamp`:**
`Operation` is the unit that *will* be logged and broadcast: an ID (for
exactly-once application), origin replica, type, payload, plus the two pieces of
causal metadata — a `VectorClock` and an `HLCTimestamp`. The op types
(`create_node`, `rename_node`, …) map 1:1 onto the Phase 1 graph methods and the
REST routes, so the wire model and the API already line up. `HLCTimestamp`
captures the corrected design (physical, logical, replicaID) — definition now,
`Now/Update/Compare` in Phase 3.

**Why define these so early?**
The `Replica` struct embeds a clock and an op log. If those were untyped
placeholders, the Phase 3 refactor would churn the struct and everything reading
it. Pinning the types now makes Phase 3 additive, not destructive.

---

## 16. `internal/replica/replica.go` — The Replica

**File:** `internal/replica/replica.go`

A `Replica` is the per-process unit of the cluster:

```go
type Replica struct {
    ID    string
    Peers []Peer        // {id, address} of the other replicas

    Graph *graph.Graph  // the Phase 1 engine, unchanged

    mu    sync.RWMutex
    clock crdt.VectorClock   // zeroed in Phase 2
    oplog []crdt.Operation   // empty in Phase 2
}
```

**"Replica", not "node" — and every replica is equal.**
The naming is the lesson: in a CRDT system there is no leader and no coordinator,
so there is no special node. `New(id, peers)` seeds the vector clock with *itself
plus every peer* at zero, so the clock is complete and comparable from the first
millisecond.

**The graph is untouched.**
Phase 1's engine drops in as-is. The `Replica` *wraps* it rather than modifying
it — separation of concerns paying off already. Phase 3 will intercept the graph
mutations to emit operations, but the engine's core stays the same.

**Its own `sync.RWMutex`.**
The clock and op log get their own lock, independent of the graph's lock. They
protect different state mutated on different paths (graph = REST writes; clock/log
= operation generation in Phase 3). Two locks, two concerns — no false contention
between reading the graph and reading the clock.

**`Clock()` and `OpLogLen()`** return snapshots/counts for `/status`. In Phase 2
they always report zeros — which is exactly the evidence that no operations are
flowing.

---

## 17. Wiring: `main.go`, `PEERS`, and richer `/status`

**Files:** `cmd/replica/main.go`, `internal/api/handlers.go`

**`PEERS` env var — peer discovery without a registry.**
Each replica learns its peers from a single env string:

```
PEERS="replica-b=http://replica-b:8080,replica-c=http://replica-c:8080"
```

`parsePeers` splits it into `{id, address}` pairs, skipping malformed entries. An
empty `PEERS` means single-replica mode — Phase 1 behavior still works unchanged.
This static config is enough for a fixed 3-replica cluster; dynamic membership
isn't needed for the project's goals.

**`NewHandler` now takes a `*replica.Replica`.**
The handler reaches the graph via `r.Graph`, and `/status` now reports the full
replica view:

```json
{
  "replicaId": "replica-a",
  "peers": [{"id":"replica-b","address":"..."}, ...],
  "nodes": 2, "edges": 0,
  "vectorClock": {"replica-a":0,"replica-b":0,"replica-c":0},
  "opLogLen": 0
}
```

This endpoint is the Phase 2 instrument: three replicas, three different
`nodes`/`edges` counts, identical all-zero clocks — divergence made visible.
No import cycle: `api → replica → {graph, crdt}`, and `replica` never imports
`api`.

---

## 18. `Dockerfile` & `docker-compose.yml` — The 3-Replica Cluster

**Files:** `Dockerfile`, `docker-compose.yml`

**Multi-stage Dockerfile (same pattern as SentinelCache):**
Stage 1 (the full `golang` image) compiles; stage 2 (bare `alpine`) carries only
the binary. `COPY go.mod go.sum` before `COPY .` so the dependency layer is cached
and only re-downloads when deps change. Final image: a few MB, no toolchain.

**Compose — one image, three identities:**

| Container | Host port | In-network |
|---|---|---|
| replica-a | 8080 | `:8080` |
| replica-b | 8081 | `:8080` |
| replica-c | 8082 | `:8080` |

Every container listens on `:8080` inside Docker's network; the host port mapping
just lets us `curl` each one. Each gets a different `REPLICA_ID` and a `PEERS`
string naming the *other two* by their Docker hostnames.

**`PEERS` recorded, not dialed.**
This is the key Phase 2 caveat: the peer addresses are stored and shown in
`/status`, but **no replica contacts another**. The addresses sit ready for
Phase 4, when replicas open WebSocket connections to exactly these hosts.

Makefile gains `docker-up` / `docker-down` / `docker-logs`.

---

## 19. Phase 2 — Demo Output (Divergence Without Sync)

`make docker-up`, then drive the three replicas independently:

```
== all three status (empty, independent) ==
:8080 → nodes:0 edges:0 clock:{a:0,b:0,c:0} peers:[b,c]
:8081 → nodes:0 edges:0 clock:{a:0,b:0,c:0} peers:[a,c]
:8082 → nodes:0 edges:0 clock:{a:0,b:0,c:0} peers:[a,b]

== write 2 nodes to replica-a, 1 node to replica-b, nothing to replica-c ==
POST :8080/node {id:1,Email}   POST :8080/node {id:2,AI}
POST :8081/node {id:9,OnlyOnB}

== resulting state — they DIVERGE, because nothing syncs ==
replica-a → nodes:2
replica-b → nodes:1
replica-c → nodes:0

replica-a /graph → [Email, AI]
replica-c /graph → []          ← never heard about any write
```

This is the whole Phase 2 story in one screen: **three replicas, three different
states, and they stay that way.** The vector clocks all read zero because no
*operations* are generated yet (Phase 3) — `opLogLen` is `0` everywhere. The
divergence here is precisely what the CRDT engine + replication will erase in
Phases 3–4, and the convergence checker will *prove* erased.

---

## 20. Phase 2 — Known Limitations & What Phase 3 Adds

| Limitation (by design) | Removed in |
|---|---|
| Replicas never communicate (`PEERS` recorded, not dialed) | Phase 4 (WebSocket replication) |
| Vector clock always zero, op log always empty | Phase 3 (mutations emit operations) |
| Writes to different replicas diverge permanently | Phase 3 + 4 (CRDT merge + sync) |
| Plain graph fields clobber on concurrent edits | Phase 3 (OR-Set + HLC-LWW) |
| State in-memory; container restart loses it | persistent log is a V2 item |

**Next: Phase 3 — the CRDT Engine.** This is the core. Wrap node/edge presence in
an **OR-Set**, wrap title/position in an **HLC-ordered LWW register**, make every
graph mutation emit an `Operation` that advances the **vector clock** and appends
to the log, and build the **convergence checker** (canonical state hash) as the
test oracle — before any networking. By the end of Phase 3 two replicas fed the
same operations in any order must produce identical state hashes.

---

# Phase 3 — CRDT Engine (the core)

**Goal:** make state *convergent*. Wrap node/edge presence in OR-Sets, title and
position in HLC-ordered LWW registers, turn every mutation into an `Operation`
that advances the vector clock + HLC and appends to the log, and build the
**convergence checker** (a content hash) as the test oracle — all before any
networking. The bar: two replicas fed the same operations *in any order* produce
identical state hashes. Met.

---

## 21. The Shift: State Mutates Only By Operations

This is the conceptual heart of the whole project, so it gets its own section.

Through Phase 2, the graph was mutated *directly* — `CreateNode` reached in and
wrote a map. In Phase 3 that becomes illegal. **The only way state changes is
`State.Apply(operation)`.** A REST call no longer mutates the graph; it asks the
replica to *generate an operation*, which is then applied (locally now, and
broadcast to peers in Phase 4).

Why this matters: if the only mutation path is "apply an operation", and
operations are designed so that applying them in any order yields the same result
(OR-Set unions + max-HLC LWW), then a replica that has seen the same set of
operations as another — regardless of order or duplication — is in the same
state. That property *is* eventual consistency. Everything else in this phase
exists to make that one sentence true.

The data flow:

```
REST call ─▶ Replica.CreateNode() ─▶ build Operation (tag, HLC, vector clock)
                                       │
                                       ├─▶ append to operation log
                                       └─▶ State.Apply(op) ─▶ mutate CRDTs
Peer op  ─▶ Replica.Ingest(op) ────────┘   (dedup, merge clock, advance HLC)
```

Local generation and remote ingest converge on the same `State.Apply`. Phase 4
just adds the transport that carries an op from one replica's log to another's
`Ingest`.

---

## 22. `crdt/hlc.go` — Hybrid Logical Clock

**File:** `internal/crdt/hlc.go`

The single most important correction from the design review, now in code. LWW
registers are ordered by an **HLC**, never a raw wall clock.

**The problem it solves:** if rename conflicts were resolved by `time.Now()`, the
replica with the fastest physical clock would win *every* concurrent rename —
non-deterministic, skew-dependent, exactly what §11 of the design rejected.

**The algorithm:**
- `Now()` (local event): `physical = max(physical, wall())`; if the wall clock
  didn't advance, bump `logical` so timestamps stay strictly increasing.
- `Update(remote)` (on receive): advance to dominate both local and remote —
  `physical = max(local, remote, wall())`, with the logical counter reconciled so
  a cause never out-ranks its effect.

**`HLCTimestamp.After`** defines the total order `(physical, logical, replicaID)`.
The `replicaID` final tiebreak is what makes the winner *deterministic* — every
replica independently computes the same one.

**Injected clock:** `NewHLC(id, now)` takes a `func() int64` for wall millis;
tests pass a counter so HLC winners are predictable. The real binary passes
`nil` → `time.Now().UnixMilli()`.

---

## 23. `crdt/vector_clock.go` — Increment / Merge / Compare

**File:** `internal/crdt/vector_clock.go`

The clock type existed since Phase 2; Phase 3 adds the behavior:
- `Increment(id)` — bump own component, once per locally generated op.
- `Merge(other)` — element-wise max; fold in everything a peer has seen.
- `Compare(other)` → `Before | After | Equal | Concurrent`.

**Used for sync and concurrency detection — not merge.** Worth repeating because
it's a classic trap: the merge logic (OR-Set tags, HLC) never consults the vector
clock. The clock earns its keep in Phase 4's anti-entropy ("which ops is this peer
missing?") and in labeling concurrent vs causal edits in the timeline. `Compare`
is built now and tested; it gets *used* in Phase 4.

---

## 24. `crdt/orset.go` — Add-Wins Observed-Remove Set

**File:** `internal/crdt/orset.go`

Node and edge *presence* is an OR-Set. The mechanics:
- Every add records a unique **tag** `(replicaID, counter)`.
- A remove records the tags it **observed** at that moment as tombstones.
- An element is present iff it has an add tag that is not tombstoned.

**Add-wins, concretely:** if replica A deletes node X (observing tag `a-1`) while
replica B concurrently adds X (tag `b-1`), the delete only tombstones `a-1`.
After merge, `b-1` is still live → X survives. This is what prevents the
"zombie resurrection" bug naive sets suffer, and it's the behavior
`TestORSetAddWins` and `TestConcurrentCreateDeleteAddWins` pin down.

**Why the remove carries observed tags** (rather than just the element id): that
is precisely the information that lets a concurrent, *unobserved* add escape the
delete. A remove that blindly erased the element would lose the concurrent add.

**`Merge` is a set union** of adds and of removes — commutative, associative,
idempotent. That is the formal reason merge order never matters.

**Honest limitation in the comments:** tombstones grow forever (no
causal-stability GC in V1). Surfaced as the `tombstones` field in `/status` and
`TombstoneCount()`.

---

## 25. `crdt/lww.go` — HLC-Ordered LWW Register

**File:** `internal/crdt/lww.go`

A generic `LWWRegister[T]` holding a value and its HLC timestamp. `Set(v, ts)`
applies the write only if `ts.After(current)`. Used for `Title` (string) and
`Position` (an `{X,Y}` struct).

Because `Set` keeps the max-HLC write, applying the same writes in any order
lands on the same value — the register-level convergence that the whole graph's
convergence is built from. `TestRenameLWWHigherHLCWins` checks both apply orders
reach the same winner.

This is the **scoped** LWW the blueprint endorses (one field that genuinely can't
keep two values), not the document-level LWW that loses data and was rejected.

---

## 26. graph: State, Apply, Materialize, Convergence Hash

**Files:** `internal/graph/graph.go`, `materialize.go`, `convergence.go`
(and `node.go` / `edge.go` for the materialized types)

The plain Phase 1 engine is replaced by a CRDT `State`:

```go
type State struct {
    nodes, edges *crdt.ORSet                          // presence
    titles    map[string]*crdt.LWWRegister[string]    // per-node title
    positions map[string]*crdt.LWWRegister[Position]  // per-node position
    endpoints map[string]endpoint                     // edge -> src/target
    createdAt map[string]int64                        // display metadata
}
```

**`Apply(op)` is the whole state machine** — a switch over op type that updates
the right CRDT. It is deterministic and order-independent by construction. The
operation payloads (`CreateNodePayload`, …) live in the `graph` package because
`Apply` interprets them; `crdt.Operation` only carries the raw bytes, so `crdt`
never imports `graph` (no cycle).

**`materialize.go` — the read boundary.** `Snapshot()` resolves the CRDTs into
plain `Node`/`Edge` lists, sorted by ID, and is where **edge referential
integrity** is enforced: an edge is emitted only if both endpoints are present
nodes. The edge OR-Set is never mutated for this — a node deleted out from under
an edge simply makes the edge vanish from the view, and *reappear* if the node
returns. This is the §13a policy, and the live demo (§29) shows the round-trip.

**`convergence.go` — the oracle.** `Hash()` is a SHA-256 over the canonical
snapshot, with length-prefixed fields so `"ab"+"c"` can't collide with `"a"+"bc"`.
`CreatedAt` is deliberately excluded — it's display metadata, not logical state.
This hash is the test oracle *and* the `stateHash` in `/status` (so an external
script can compare replicas), and it will drive the divergence metric in Phase 5.

**Local-convenience errors:** `CreateNode` on an existing id still returns a
`409` for nice API behavior, but these checks are *local only* — `Apply`/`Ingest`
never reject a peer's operation. A replica must accept anything a peer sends.

---

## 27. replica: Operation Generation & Ingest

**File:** `internal/replica/replica.go`

The `Replica` becomes the operation engine.

**`emit(type, build)`** is the local-generation core: lock, `clock.Increment`,
`hlc.Now()`, mint the unique tag `(id, counter)`, marshal the payload, assemble
the `Operation` (carrying a vector-clock snapshot + HLC), record it as applied,
append to the log, and `state.Apply` it. Every public mutation (`CreateNode`,
`RenameNode`, …) is a thin wrapper that supplies its payload.

**Delete carries observed tags:** `DeleteNode` reads `state.ObservedNodeTags(id)`
and ships them in the op, so the add-wins semantics survive replication.

**`Ingest(op)`** is the remote path: dedup by op ID (idempotent —
`TestIngestIdempotent`), `clock.Merge`, `hlc.Update`, append, `state.Apply`. This
is the exact entry point Phase 4's transport will call.

**Two locks, still:** the graph `State` has its own lock; the replica's lock
guards the clock/log/applied set. Generation serializes through the replica lock
so the log and clock stay consistent.

**No cascade delete.** Deleting a node no longer touches edges (Phase 1 did). The
dangling edge is handled at materialization — the correct CRDT behavior, and the
visible difference from Phase 1.

---

## 28. Convergence Tests — The Proof

**Files:** `internal/graph/graph_test.go`, `internal/replica/replica_test.go`

| Test | What it proves |
|---|---|
| `TestConvergenceOrderIndependent` | Same ops, forward vs reverse order → identical hash |
| `TestRenameLWWHigherHLCWins` | LWW winner is the same regardless of apply order |
| `TestORSetAddWins` | Concurrent add survives a delete that didn't observe it |
| `TestDanglingEdgeFilteredAndRestored` | Edge hidden when endpoint deleted, restored when re-added |
| `TestTwoReplicasConverge` | Two replicas, independent edits, exchange logs → equal hash |
| `TestConcurrentRenameConverges` | Concurrent renames of one node converge to one winner |
| `TestConcurrentCreateDeleteAddWins` | Add-wins holds end-to-end across replicas |
| `TestIngestIdempotent` | Duplicate delivery changes nothing |

`syncAll` in the replica test cross-feeds every replica's op log into the others
— a lossless stand-in for Phase 4's network. The whole suite runs under `-race`.

---

## 29. Phase 3 — Demo Output (Dangling Edge Round-Trip)

Single replica, compiled binary (use the binary, not `go run` — a killed
`go run` orphans its child, the lesson SentinelCache's DEVLOG already recorded;
we hit it again here):

```
== create 2 nodes + edge ==
graph: nodes[1:Email, 2:AI]  edges[e1: 1->2]

== dangling edge to nonexistent node 99 (ALLOWED now, was 422 in Phase 1) ==
POST /edge {e2: 1->99} → echoed {id:e2,...}
visible edges: [e1]                ← e2 created in the OR-Set but filtered

== delete node 2 → e1 dangles & disappears (NO cascade) ==
nodes: [1]  edges: []              ← e1 hidden, but still in the edge OR-Set

== re-add node 2 → e1 REAPPEARS ==
nodes: [1, 2]  edges: [e1]         ← edge OR-Set untouched, so it comes back

== status ==
{ nodes:2, edges:1, opLogLen:6, tombstones:1,
  vectorClock:{replica-a:6},
  stateHash:"ba68493d…" }
```

The reappearing edge is the whole §13a story in one curl session — and it is the
behavior Phase 1's cascade delete made *impossible*. `stateHash` is the
convergence oracle, now observable over HTTP.

---

## 30. Phase 3 — Known Limitations & What Phase 4 Adds

| Limitation (by design) | Removed in |
|---|---|
| Replicas converge only when ops are hand-fed (tests/`syncAll`) | Phase 4 (real transport) |
| `Compare` / vector-clock diff built but unused | Phase 4 (anti-entropy) |
| No retransmission, no loss handling | Phase 4 (op log + anti-entropy) |
| Tombstones & op log grow unbounded | V2 (causal-stability GC) |
| State in-memory | V2 (persistent log) |

**Next: Phase 4 — Replication Layer.** Give each replica a WebSocket transport
that broadcasts every generated operation to peers (who apply it via `Ingest`),
plus the anti-entropy loop that uses the vector-clock diff to recover operations
lost to latency or partition. After Phase 4 the convergence proven here happens
live, over the network — and `syncAll` gets replaced by real wires.

*Last updated: Phase 3 complete.*

---

# Phase 4 — Replication Layer

**Goal:** make convergence happen *live*, over real WebSocket connections between
Docker containers. Phase 3 proved that the same set of operations produces the
same state hash regardless of order. Phase 4 delivers those operations over the
wire — with a broadcast fast path and an anti-entropy backstop that recovers
anything the fast path missed.

At the end of this phase the three-replica Docker cluster is running. Writing to
any one replica causes all three to converge to an identical `stateHash` within
milliseconds. The `syncAll` test helper that hand-fed operations in Phase 3 tests
is now real wires.

---

## 31. The Reliability Problem With Op-Based CRDTs

State-based CRDTs (like a G-Counter or OR-Set) are trivially replicated: just
send the full state, merge. The tradeoff is that every sync payload is the full
state — fine for small registers, brutal for a growing graph.

Op-based CRDTs are better on bandwidth: instead of shipping the full OR-Set, you
ship a three-byte `{op: "add", elem: "n1", tag: {a, 1}}`. But they require every
operation to be delivered to every replica *exactly once and in causal order*.
Miss one operation and the state silently diverges.

That guarantee is easy on a single machine (`syncAll` in tests) and impossible to
promise on a real network. So Phase 4 uses a **hybrid**:

| Layer | What it does | Delivery promise |
|---|---|---|
| Broadcast | Push each new op to all peers immediately | Best-effort, once |
| Anti-entropy | Periodic clock diff → replay what peer missed | Eventually all ops |
| Ingest dedup | Operation IDs make retransmits harmless | Exactly-once apply |

Broadcast gets convergence in milliseconds when nothing is dropped. Anti-entropy
guarantees convergence eventually, no matter what. Dedup makes the two paths
composable — anti-entropy can re-send what broadcast already delivered and no
damage is done.

---

## 32. `internal/transport/transport.go` — The WebSocket Manager

**File:** `internal/transport/transport.go`

The entire peer-to-peer layer lives here. One `Manager` per replica process.

**Wire format:**

```go
type message struct {
    Type  msgType          // "op" | "sync_request" | "sync_response"
    Op    *crdt.Operation  // set for msgOp
    Clock crdt.VectorClock // set for msgSyncReq
    Ops   []crdt.Operation // set for msgSyncResp
}
```

Three message types cover everything:

| Message | Sender | Payload | Receiver action |
|---|---|---|---|
| `op` | Any replica | One operation | `Ingest` it |
| `sync_request` | Any replica | My current vector clock | Compute `MissingFor(clock)`, reply with `sync_response` |
| `sync_response` | Responding peer | Slice of missing ops | `Ingest` each |

**Topology — full mesh:**

Each replica dials every peer (`dialLoop`). Inbound connections land at `HandleWS`. Operations are never relayed — a replica applies what it receives but does not re-broadcast; dedup and anti-entropy close any gap a direct link missed.

**`peerConn` write mutex:**

`gorilla/websocket` forbids concurrent writes on one connection. Broadcast,
anti-entropy, and sync replies can all fire on the same connection simultaneously,
so every connection is wrapped in a `peerConn{mu sync.Mutex, ws}` and all writes
go through `pc.write(v)` which holds the mutex.

**Reconnect loop:**

`dialLoop` reconnects every 2 s after a lost connection. On each fresh connection
it immediately sends a `sync_request` with the current clock — so a replica that
missed messages while disconnected catches up before the next anti-entropy tick
even fires.

**Anti-entropy loop:**

A 3-second ticker sends a `sync_request` (current clock) to every connected peer.
Each peer runs `MissingFor(clock)` and replies with any operations the sender
lacks. This is the backstop that prevents permanent divergence from dropped
packets or partitions.

**Broadcast outside the lock:**

```go
// emit() in replica.go:
bc := r.broadcast   // captured under r.mu
r.mu.Unlock()
if bc != nil {
    bc(op)          // called WITHOUT r.mu held
}
```

A slow peer write (blocked network, full buffer) must never stall a local
mutation. If broadcast held the replica lock, any client request that generated
an operation would block until the transport finished writing to every peer —
turning a network problem into an API availability problem. Capturing `bc` then
releasing the lock before calling it keeps the two concerns independent.

---

## 33. `internal/replica/replica.go` — Gap-Aware Vector Clock

**File:** `internal/replica/replica.go` (additions in Phase 4)

The replica's vector clock tracks the **highest *contiguous* sequence received
from each origin**, not the maximum. This is the `recordSeq` function:

```go
func (r *Replica) recordSeq(origin string, seq uint64) {
    r.seen[origin][seq] = true
    for r.seen[origin][r.clock[origin]+1] {
        r.clock[origin]++
    }
}
```

**Why contiguous and not max?**

Suppose peer A has emitted ops `a-1`, `a-2`, `a-3`. You receive `a-1` and `a-3`
but `a-2` is dropped:

- **Max-merge:** `clock["a"] = 3`. When A asks *"what do you need?"*, you reply
  `{a: 3}`. A computes `MissingFor({a:3})` and returns nothing — it thinks you
  have everything up to 3. `a-2` is lost forever.
- **Contiguous-advance:** `clock["a"] = 1` (the `seen` set has 3 but the
  contiguous run only reaches 1). You reply `{a: 1}`. A returns ops for sequences
  `> 1` — including `a-2`. Convergence recovered.

The `seen map[string]map[uint64]bool` records every sequence received; the loop
in `recordSeq` advances the clock only as far as the contiguous prefix allows.

**`MissingFor`:**

```go
for _, op := range r.oplog {
    if op.VectorClock[op.ReplicaID] > peerClock[op.ReplicaID] {
        out = append(out, op)
    }
}
```

Each operation carries the clock value it was generated at (`op.VectorClock[origin]`). If that value is above what the peer has contiguously seen, the peer needs it.

---

## 34. `internal/transport/transport_test.go` — Live Convergence Tests

**File:** `internal/transport/transport_test.go`

Two integration tests exercise the full transport stack with real gorilla/websocket
connections — not mocks, not channels, real TCP.

**`TestLiveBroadcastConverges`**

Creates two replicas each behind an `httptest.Server`. Connects them. Waits for
the mesh to come up (polls until both sides have an outbound connection). Then
writes 2 nodes + 1 edge on A and asserts that `a.Hash() == b.Hash()` within 2 s.
This validates the fast path: broadcast delivers fresh ops immediately.

**`TestAntiEntropyCatchUp`**

Creates state on A *before* B exists anywhere near it (so broadcast can't deliver
anything). Then connects A and B. Asserts convergence within 5 s.
This validates the backstop: only the anti-entropy sync_request/sync_response
exchange can recover the pre-connection state. If the 3 s ticker fires once and
the response is applied, B catches up.

**Test infrastructure:**

```go
type node struct {
    rep    *replica.Replica
    mgr    *Manager
    server *httptest.Server
    cancel context.CancelFunc
}
```

`startNode` builds a replica + manager + httptest server in one call. `connect`
sets the peer list and starts the transport goroutines. `eventually` polls a
condition with 20 ms sleep intervals up to a deadline — the convergence predicate
is just `a.Hash() == b.Hash()`.

**Race detector:** both tests pass under `-race`. The mutex discipline in
`peerConn`, `Manager.outbound`, and `Replica` are all exercised by real concurrent
goroutines here — the race detector gives confidence the locking is correct.

---

## 35. `internal/replica/replica_test.go` — Anti-Entropy Gap-Aware Test

**File:** `internal/replica/replica_test.go` (Phase 4 addition)

**`TestAntiEntropyGapAware`** is a pure unit test (no networking) that verifies
the contiguous clock behavior in isolation:

1. A emits ops `a-1`, `a-2`, `a-3`.
2. B receives `a-1` and `a-3` but NOT `a-2`.
3. Assert `b.Clock()["a"] == 1` — the gap is not skipped.
4. Call `a.MissingFor(b.Clock())` — assert `a-2` is in the result.
5. Apply the missing ops to B. Assert `a.Hash() == b.Hash()` and
   `b.Clock()["a"] == 3`.

This test exists separately from the transport tests because it isolates the
clock algorithm from networking. If `recordSeq` ever regresses to a max-merge,
this test catches it without needing WebSocket connections.

---

## 36. Wiring: `cmd/replica/main.go` + `go.mod`

**Phase 4 changes to existing files.**

**`cmd/replica/main.go`:**

```go
mgr := transport.NewManager(r)
go mgr.Start(context.Background())
// ...
engine.GET("/ws", gin.WrapF(mgr.HandleWS))
```

Three lines. `NewManager` registers itself as the replica's broadcaster
(`r.SetBroadcast(m.Broadcast)`), so from this point forward every locally
generated operation is automatically pushed to peers. `Start` dials all peers
and runs the anti-entropy loop in the background. `/ws` is where inbound peer
connections land — Gin's `WrapF` bridges the standard `http.HandlerFunc` to
Gin's handler.

**`go.mod`:**

`github.com/gorilla/websocket v1.5.3` added via `go mod tidy` after
`transport.go` was written. The module was already de facto selected by
indirect dep — `go mod tidy` just made it explicit.

---

## 37. Phase 4 — Demo Output (Live 3-Replica Convergence)

Three Docker containers, full-mesh WebSocket topology.

```
docker compose up --build -d
```

All three come up, each dials both peers, anti-entropy tickers start.

**Initial state — all identical (empty graph):**

```json
replica-a: { nodes:0, edges:0, stateHash:"e3b0c44…", vectorClock:{a:0,b:0,c:0} }
replica-b: { nodes:0, edges:0, stateHash:"e3b0c44…", vectorClock:{a:0,b:0,c:0} }
replica-c: { nodes:0, edges:0, stateHash:"e3b0c44…", vectorClock:{a:0,b:0,c:0} }
```

`e3b0c44…` is SHA-256 of the empty string — the canonical empty-graph hash.

**Concurrent writes — A gets 2 nodes + 1 edge, C gets 1 node:**

```
POST localhost:8080/node  {"id":"n1","title":"Email",...}
POST localhost:8080/node  {"id":"n2","title":"AI",...}
POST localhost:8080/edge  {"id":"e1","source":"n1","target":"n2"}
POST localhost:8082/node  {"id":"n3","title":"Slack",...}
```

**Status 2 s later — all three replicas converged:**

```json
replica-a: { nodes:3, edges:1, opLogLen:4, stateHash:"f2fdb135…",
             vectorClock:{a:3, b:0, c:1} }
replica-b: { nodes:3, edges:1, opLogLen:4, stateHash:"f2fdb135…",
             vectorClock:{a:3, b:0, c:1} }
replica-c: { nodes:3, edges:1, opLogLen:4, stateHash:"f2fdb135…",
             vectorClock:{a:3, b:0, c:1} }
```

All three `stateHash` values are identical — `f2fdb135…` — despite the writes
landing on different replicas with no coordination. The vector clock shows
`a:3, c:1`: A generated 3 ops, C generated 1, B generated nothing but holds all
4 (opLogLen:4). This is the Phase 3 convergence proof running live over real TCP.

---

## 38. Phase 4 — Known Limitations & What Phase 5 Adds

| Limitation (by design) | Removed in |
|---|---|
| Network is perfect (Docker overlay, no loss) | Phase 5 (network simulation — drop / delay / partition) |
| Anti-entropy recovery only tested at init | Phase 5 (mid-run partition + reconnect scenario) |
| No back-pressure on broadcast (slow peer blocks its own goroutine) | V2 |
| Tombstones and op log grow unbounded | V2 (causal-stability GC) |
| State in-memory; container restart loses history | V2 (persistent op log) |

**Next: Phase 5 — Network Simulation.** Add a chaos layer that can drop, delay,
reorder, or partition individual replica links at runtime via the REST API. With
that in place we can demonstrate the full resilience story: partition two replicas,
keep writing on both sides, restore the link, watch the anti-entropy backstop
collapse the divergent state hashes back to one value. That is the interview demo.

*Last updated: Phase 4 complete (WebSocket replication — broadcast fast path,
anti-entropy backstop, gap-aware vector clock; all tests race-clean; live
3-replica Docker convergence verified).*

---

# Phase 5 — Network Simulation

**Goal:** make the distributed systems demo *real*. Phase 4 proved convergence on
a perfect Docker overlay network. Phase 5 adds runtime controls to break that
network — inject latency, drop packets, or hard-partition a replica — and then
watch anti-entropy automatically heal the divergence when conditions improve.

After this phase you can demonstrate from a terminal (or later, the dashboard):
isolate replica-b, keep writing on a and c, then recover b and watch all three
`stateHash` values collapse to one value within seconds. That is the interview story.

---

## 39. The Problem With a Perfect Network

Phase 4's live demo was compelling, but there was a catch: the network was
Docker's internal overlay — essentially perfect. No drops, no reorder, no
partition. The anti-entropy backstop was exercised only in the "pre-connect state"
test; the broadcast fast path never failed.

A distributed systems project that only demonstrates convergence under ideal
conditions isn't demonstrating much. The hard part is: *what happens when nodes
diverge, and how do they reunite?*

Two failure modes are worth implementing separately because they demonstrate
different recovery paths:

| Failure | State retained? | Recovery |
|---|---|---|
| **Soft partition** (`POST /sim/isolate`) | Yes — process lives, in-memory op log intact | Cheap delta: B reconnects with a full clock, A sends only the diff |
| **Hard crash** (`docker compose stop replica-b`) | No — process dies, all in-memory state lost | Full catch-up: B restarts with zero clock, anti-entropy streams entire history |

The chaos injector here handles soft partition. Hard crash is covered by the
existing anti-entropy loop — a restarted replica's zero clock causes `MissingFor`
to return every op ever generated, and replay converges it.

---

## 40. `internal/simulation/chaos.go` — The Fault Injector

**File:** `internal/simulation/chaos.go`

Three independent knobs, all guarded by a single `sync.RWMutex`:

| Knob | Direction | Effect |
|---|---|---|
| `latency time.Duration` | Outgoing only | `time.Sleep(latency)` before each write |
| `lossRate float64` | Outgoing only | Drop with probability `p` (uses `math/rand/v2`) |
| `isolated bool` | **Both** | Block all outgoing sends; discard all incoming messages |

The split between "outgoing only" and "both directions" is deliberate:

- **Packet loss** and **latency** model the link from *this replica to its peers*
  — the replica can still receive what peers send to it. This is realistic: links
  are often asymmetrically degraded.
- **Isolation** models a full network partition — nothing gets in or out. This
  is the important demo scenario: a replica is cut off, both sides diverge, then
  the cut heals and anti-entropy re-unites them.

**Key methods:**

```go
ShouldDrop() bool      // isolated || rand < lossRate — gate outgoing writes
ApplyDelay()           // time.Sleep(latency) — called before each write
IsIsolated() bool      // gate incoming message processing in readLoop
Snapshot() Snapshot    // latencyMs, lossRate, isolated — for /status
```

All methods are safe for concurrent use. The transport's broadcast, anti-entropy
ticker, and dial goroutines all run in separate goroutines and can call these
simultaneously.

---

## 41. Transport Integration — Checking Chaos on Every Send/Receive

**File:** `internal/transport/transport.go`

`Manager` gains a `chaos *simulation.Chaos` field (passed in from `main.go`).
Checks are minimal and non-invasive — the transport's structure is unchanged.

**Outgoing checks (broadcast, anti-entropy, initial sync_request on connect):**

```go
// In Broadcast():
for _, pc := range m.conns() {
    if m.chaos.ShouldDrop() { continue }
    m.chaos.ApplyDelay()
    pc.write(message{Type: msgOp, Op: &op})
}

// In antiEntropyLoop():
if m.chaos.IsIsolated() { continue }  // skip entire tick
for _, pc := range m.conns() {
    if m.chaos.ShouldDrop() { continue }
    m.chaos.ApplyDelay()
    pc.write(message{Type: msgSyncReq, Clock: clock})
}
```

**Incoming check (readLoop):**

```go
if m.chaos.IsIsolated() {
    continue  // discard the message — both directions blocked
}
// switch msg.Type { ... }
```

The read loop keeps the TCP connection alive even while isolated. This means
lifting isolation is instant — the goroutine is already waiting on the socket,
so the next message (from the peer's anti-entropy tick or our own) is processed
immediately. No reconnect penalty, no 2-second wait.

**Why not close connections during isolation?**

Closing connections would mean the `dialLoop` tries to reconnect every 2 s. When
isolation is lifted we'd have to wait for a successful dial before recovery could
start. Keeping connections open but discarding messages gives instantaneous
recovery — isolation ends, the next anti-entropy tick fires (at most 3 s), and
we're back.

---

## 42. `internal/api/sim.go` — `/sim` REST Endpoints

**File:** `internal/api/sim.go`

Four endpoints, each returning the updated `Snapshot` so the caller can confirm
what changed:

| Endpoint | Payload | Action |
|---|---|---|
| `POST /sim/latency` | `{"ms": 200}` | Set outgoing message delay |
| `POST /sim/loss` | `{"rate": 0.3}` | Set drop probability `[0.0–1.0]` |
| `POST /sim/isolate` | (none) | Soft-partition this replica |
| `POST /sim/recover` | (none) | Lift soft-partition |

All four are wired in `routes.go` and share the `Handler.chaos` field added to
the existing handler struct. `/status` also now includes `"chaos": {...}` so you
can see each replica's current fault configuration without making a separate call.

**Validation:**
- `ms < 0` → 400 (negative latency is nonsensical)
- `rate` outside `[0.0, 1.0]` → 400

---

## 43. Chaos Tests

Two test files cover Phase 5.

**`internal/simulation/chaos_test.go`** — unit tests for the Chaos struct:

| Test | What it proves |
|---|---|
| `TestChaosDefaultNoDrop` | 1000 trials, zero loss, no drops |
| `TestChaosIsolationAlwaysDrops` | isolated → every `ShouldDrop` = true |
| `TestChaosIsolationLifted` | lifted isolation + zero loss → no drops |
| `TestChaosFullLossDropsAll` | 100% loss → every `ShouldDrop` = true |
| `TestChaosZeroLossDropsNone` | 0% loss → no drops (1000 trials) |
| `TestChaosPartialLossApproximate` | 50% loss → 45–55% drop rate (10k trials) |
| `TestChaosApplyDelayZero` | zero latency → returns in < 5 ms |
| `TestChaosApplyDelayNonZero` | 20 ms latency → delays ≥ 15 ms |
| `TestChaosSnapshot` | all fields round-trip through `Snapshot()` |

**`internal/transport/transport_test.go`** — two new integration tests:

**`TestChaosIsolationConverges`**: soft-isolates B while A writes two nodes.
Asserts B has zero nodes after 100 ms (broadcast was discarded). Lifts isolation.
Asserts convergence within 5 s (anti-entropy delivers the delta). This is the
exact partition-recover scenario from the design docs, running on real WebSocket
connections.

**`TestChaosPacketLossConverges`**: sets 100% outgoing loss on A (so A's broadcast
and anti-entropy sync_requests are all dropped). A writes two nodes. B stays at
zero. Loss cleared. Within 6 s — one anti-entropy tick from A plus the round-trip
— B catches up. Proves that loss is the send-side fault path and recovery relies
on A's periodic sync_request, not B's.

All tests pass under `-race` with real goroutines and gorilla/websocket connections.

---

## 44. Phase 5 — Demo Output (Partition + Recovery)

Three Docker containers, same setup as Phase 4.

**Isolate replica-b:**

```
POST localhost:8081/sim/isolate
→ {"latencyMs":0,"lossRate":0,"isolated":true}
```

**Write on A while B is partitioned:**

```
POST localhost:8080/node  {"id":"n1","title":"Email",...}
POST localhost:8080/node  {"id":"n2","title":"AI",...}
```

**B during partition — diverged:**

```
localhost:8081/status → nodes:0, stateHash:"e3b0c44…" (empty)
localhost:8080/status → nodes:2, stateHash:"c34060f2…"
```

**Recover B:**

```
POST localhost:8081/sim/recover
→ {"latencyMs":0,"lossRate":0,"isolated":false}
```

**All replicas 4 s later — converged:**

```
:8080  nodes=2  hash=c34060f2de17…
:8081  nodes=2  hash=c34060f2de17…
:8082  nodes=2  hash=c34060f2de17…
```

The `stateHash` values are identical. B rejoined, anti-entropy delivered the
2-op delta, and the CRDT merged it. No coordination, no leader, no manual
reconciliation — just the anti-entropy loop doing its job.

---

## 45. Phase 5 — Known Limitations & What Phase 6 Adds

| Limitation (by design) | Removed in |
|---|---|
| Network faults are self-inflicted (API call required; nothing is automatic) | Phase 6 (simulated users + random faults) |
| Only one replica can be faulted at a time from the CLI | Phase 7 dashboard (controls for all replicas in one UI) |
| Latency only applies to outgoing messages, not to incoming | V2 |
| Hard crash recovery is manual (`docker compose stop/start`) | Phase 7 dashboard (Hard Crash button) |
| Tombstones and op log grow unbounded | V2 (causal-stability GC) |

**Next: Phase 6 — Simulated Users.** Add a `SimulatedUser` engine that drives
random create/rename/move/delete operations against a chosen replica at a
configurable rate. With 100 virtual users spread across 3 replicas and packet
loss active, the system runs a continuous convergence stress test — and the
convergence checker (`stateHash`) is the oracle that confirms every replica
ends up identical after quiescence.

*Last updated: Phase 5 complete (network simulation — latency, packet loss, soft
isolation, REST controls; partition+recovery demo verified on live 3-replica
Docker cluster; 13 new tests, all race-clean).*

---

# Phase 6 — Simulated Users

**Goal:** eliminate the manual step. Instead of curl commands driving test writes,
virtual users inside the backend fire random graph mutations autonomously, at
configurable rate, against any replica. The convergence checker (`stateHash`)
remains the oracle — it either stays identical across all three replicas (the
system is converging) or diverges (something is broken). This phase makes the
final demo fully automated: one REST call starts 100 users, and the whole
cluster visibly converges, diverges under partition, and reconverges after recovery.

---

## 46. Why Virtual Users

The alternative — opening 100 browser tabs — demonstrates the UI, not the
distributed system. Virtual users demonstrate that the *engine* handles
concurrent mutations correctly under load, independent of any frontend.

Each virtual user is a goroutine that:
1. Creates nodes until it hits a per-user cap (20)
2. Then issues a random mix of rename, move, and delete against its own nodes
3. Handles operation errors gracefully (a concurrent delete from another user
   makes `RenameNode` return `ErrNodeNotFound` — swallowed, not a panic)

The per-user node namespace (IDs prefixed `u<i>-n<counter>`) means creates
never conflict. Renames and moves hit the same CRDT registers from multiple users
simultaneously, exercising the HLC-ordered LWW resolution path under real load.

---

## 47. `internal/simulation/simulator.go` — The Virtual User Engine

**File:** `internal/simulation/simulator.go`

```go
type Simulator struct {
    replica   *replica.Replica
    mu        sync.Mutex
    cancel    context.CancelFunc
    wg        sync.WaitGroup  // tracks live goroutines
    users     int
    opsPerSec float64
    running   bool
    totalOps  atomic.Int64
}
```

**`Start(users int, opsPerSec float64)`:**
Calls `Stop()` first (waiting for any previous run to drain), then launches
`users` goroutines each ticking at `1/opsPerSec` per second via `time.NewTicker`.

**`Stop()`:**
Cancels the context and then calls `s.wg.Wait()`. **The Wait is critical.** Without
it, goroutines that are mid-`doOp` (already past the `select`, inside a
`CreateNode` call) will still append to the oplog after `Stop()` returns. A caller
that immediately reads `OpLog()` after `Stop()` would get an incomplete snapshot,
and convergence with another replica would fail. The WaitGroup drains every
goroutine before returning.

**`doOp` operation mix:**

```
len(nodes) == 0 OR (len < cap AND rand < 75%): CreateNode
Otherwise (rand.IntN(3)):
  0 → RenameNode
  1 → MoveNode
  2 → DeleteNode
```

Each create uses `<userID>-n<counter>` as the ID — globally unique across users
and restarts. Each user tracks its own `nodes []string`; deletes remove from the
slice. Errors from all operations are swallowed — concurrent deletes from other
users make nodes disappear from the user's list, which is fine.

**`Stats() SimStats`** returns `{running, users, opsPerSec, totalOps}`. `totalOps`
is an `atomic.Int64` — incremented by every goroutine without holding any mutex.

---

## 48. API and Wiring

**`internal/api/sim_users.go`** — three handlers:

| Endpoint | Method | Payload | Action |
|---|---|---|---|
| `/sim/users/start` | `POST` | `{"users":10,"opsPerSec":5.0}` | Start virtual users on this replica |
| `/sim/users/stop` | `POST` | (none) | Stop all virtual users |
| `/sim/users/stats` | `GET` | — | Return current SimStats |

Validation: `users` and `opsPerSec` must both be `> 0`; the Simulator's `Start`
is a no-op if either is ≤ 0 (already guarded in the engine).

**`/status` now includes `"sim"` field** alongside `"chaos"`, so a single call
shows nodes, hashes, chaos settings, and sim activity at once.

**`cmd/replica/main.go`** adds two lines:
```go
sim := simulation.NewSimulator(r)
h := api.NewHandler(r, chaos, sim)
```

The simulator is bound to the *local* replica — it writes to whichever replica
this process is. In a 3-replica cluster, you can independently start different
user counts on each replica via their respective ports.

---

## 49. Why Stop Waits (The WaitGroup Bug)

The first version of `Stop()` just called `s.cancel()` and returned:

```go
func (s *Simulator) Stop() {
    s.mu.Lock()
    s.cancel()
    s.running = false
    s.mu.Unlock()
    // NO wait — goroutines still running!
}
```

`TestSimulatorConvergence` immediately called `syncReplicas(a, b)` after
`sim.Stop()`. The snapshot captured `a.OpLog()` while some user goroutines were
still inside `doOp` (they had been dispatched by `ticker.C` just before the
context was cancelled). Those late ops were added to A's log *after* the snapshot.
B never received them. Hash diverged.

Fix: `s.wg.Wait()` after cancelling. Every goroutine calls `defer s.wg.Done()`
so `Wait()` returns only when all goroutines have cleanly exited. Now any read
of `OpLog()` after `Stop()` is guaranteed to see all ops.

This is a real-world bug class in concurrent Go: cancelling a context doesn't
mean goroutines have stopped. Always wait for the WaitGroup when correctness
depends on no more writes happening.

---

## 50. Phase 6 Tests

**`internal/simulation/simulator_test.go`** — five tests:

| Test | What it proves |
|---|---|
| `TestSimulatorOpsAccumulate` | 3 users @ 20 ops/sec → ≥5 ops in 300ms |
| `TestSimulatorStartStopIdempotent` | double-stop is a no-op; restart works |
| `TestSimulatorInvalidParams` | `users≤0` or `opsPerSec≤0` → no-op, no panic |
| `TestSimulatorConvergence` | simulator-generated ops converge when synced to a clean replica |
| `TestSimulatorStatsCumulative` | `totalOps` grows across restarts (not reset) |

`TestSimulatorConvergence` is the Phase 6 version of the Phase 3 `TestTwoReplicasConverge`
test: it proves the invariant holds under high-rate, randomized, concurrent loads
rather than hand-crafted sequences. 5 users at 30 ops/sec, 300ms = ~45 ops; all
of them converge when synced.

`syncReplicas` in `simulator_test.go` reimplements `syncAll` using only public
API (`OpLog()`, `Ingest()`) to avoid the import cycle (`replica` imports nothing;
`simulation` imports `replica`; `replica_test` is internal and can't import `simulation`).

---

## 51. Phase 6 — Demo Output (100 Users Under Load)

```
POST localhost:8080/sim/users/start  {"users":10,"opsPerSec":5.0}
→ {"running":true,"users":10,"opsPerSec":5,"totalOps":0}
```

After 3 s:

```
A: nodes=109  ops=150  sim={running:true,users:10,opsPerSec:5,totalOps:150}  hash=a793c2774f62
B: nodes=109  ops=150  hash=a793c2774f62
C: nodes=109  ops=150  hash=a793c2774f62
```

10 users, 5 ops/sec each = 50 ops/sec. 150 ops in 3 s. All three replicas
converged to the same `stateHash` under continuous load, with no coordination.

```
POST localhost:8080/sim/users/stop
→ {"running":false,"users":10,"opsPerSec":5,"totalOps":150}
```

**Combined scenario (the full demo):**

```bash
# Start load on A
POST :8080/sim/users/start  {"users":10,"opsPerSec":5.0}

# Partition B mid-run
POST :8081/sim/isolate

# Keep watching — A and C converge, B diverges
# ...

# Recover B
POST :8081/sim/recover

# B catches up via anti-entropy within 3 s
```

This is the interview demo: real load, real divergence, real recovery, provable
convergence via identical state hashes.

---

## 52. Phase 6 — Known Limitations & What Phase 7 Adds

| Limitation (by design) | Removed in |
|---|---|
| Each user only mutates its own nodes (no cross-user contention on renames) | Could be extended in V2 |
| Simulator only targets the local replica (no round-robin across replicas) | Phase 7 dashboard controls |
| `totalOps` not reset on restart (by design — cumulative) | Add a reset endpoint if needed |
| No edges in simulation (nodes only) | Easy to add; edges make demo noisier |

**Next: Phase 7 — Dashboard.** A React + React Flow frontend that visualizes
the live graph, replica states (nodes/ops/hash/clock), operation timeline, and
simulation controls — all the `/sim` and `/status` endpoints wired into one UI.
After Phase 7, the entire demo runs from a browser.

*Last updated: Phase 6 complete (simulated users — virtual user engine with
WaitGroup drain, REST start/stop/stats, 5 new tests race-clean, live Docker demo
150 ops/3s all converging).*
