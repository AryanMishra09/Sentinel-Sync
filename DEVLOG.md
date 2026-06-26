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

*Last updated: Phase 2 complete (3 independent replicas via Docker Compose,
peer-aware, divergence demonstrated, race-clean). Next: Phase 3 CRDT engine.*
