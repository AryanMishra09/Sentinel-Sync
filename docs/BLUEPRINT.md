# PROJECT_BLUEPRINT.md

# SentinelSync

> A distributed state synchronization engine built with CRDTs and eventual consistency.

---

# 1. Vision

Most developers use collaborative software every day:

* Google Docs
* Notion
* Figma
* Miro
* Excalidraw
* Linear

Multiple users edit the same data simultaneously and somehow every user eventually sees the same result.

At first glance this appears simple.

In reality it is one of the hardest problems in distributed systems.

SentinelSync exists to understand and implement the core mechanisms behind these systems from first principles.

Rather than building another text editor or SaaS clone, SentinelSync focuses on the distributed systems problem itself:

> How do multiple replicas independently process concurrent updates and still converge to the same state without losing data?

The goal is not to compete with Google Docs, Figma, Yjs, or Automerge.

The goal is to build a distributed synchronization engine that demonstrates:

* Conflict Resolution
* Eventual Consistency
* Replica Convergence
* Network Partition Recovery
* State Synchronization
* Distributed Replication

through a highly visual simulation platform.

---

# 2. Problem Statement

Imagine three users editing the same system simultaneously.

User A:

```text
Rename Node:
AI Processor
→ GPT Processor
```

User B:

```text
Move Node:
(100,100)
→ (400,300)
```

User C:

```text
Delete Node
```

All three operations happen at approximately the same time.

Questions immediately appear:

* Which operation wins?
* Does deletion override rename?
* What if the delete arrives later?
* What if one replica is offline?
* What if messages arrive out of order?
* What if the network partitions?

Traditional CRUD applications do not handle these scenarios well.

Most systems simply use:

```text
Last Write Wins
```

which frequently causes data loss.

Distributed collaborative systems require a fundamentally different approach.

---

# 3. Why This Problem Matters

Modern software increasingly relies on collaborative and distributed workflows.

Examples:

### Google Docs

Multiple users edit text simultaneously.

### Figma

Multiple users modify the same design.

### Notion

Multiple users update the same page.

### Miro

Multiple users manipulate the same board.

### Git

Multiple developers modify the same codebase.

### Distributed Databases

Multiple replicas modify shared state.

The same underlying problem exists everywhere:

> Concurrent modifications must eventually converge.

SentinelSync explores this problem in isolation.

---

# 4. Existing Approaches

Several approaches have historically been used.

---

## Approach 1 — Last Write Wins (LWW)

The simplest approach.

Example:

User A:

```text
Title = GPT Processor
```

User B:

```text
Title = AI Engine
```

Both updates arrive.

The newest timestamp wins.

Final state:

```text
Title = AI Engine
```

---

### Advantages

* Simple
* Easy implementation
* Common in CRUD systems

---

### Disadvantages

* Data loss
* Concurrent edits discarded
* Poor collaborative experience

---

### Decision

Rejected **as a global strategy**.

Document-level LWW (newest write replaces the entire state) is what causes data
loss, and the entire purpose of SentinelSync is to avoid that.

Note the nuance: we still use a **scoped LWW register** later (HLC-ordered) for
genuinely single-valued fields — a node's title and position — where two
concurrent edits *cannot* both be kept and one must deterministically win. That
is narrow, intentional, and not the same as throwing away the whole document.
Structure (which nodes/edges exist) is handled by CRDT sets that lose nothing.

---

## Approach 2 — Locking

Example:

```text
User A acquires lock
```

Other users:

```text
Read Only
```

until lock released.

---

### Advantages

* No conflicts
* Easy reasoning

---

### Disadvantages

* Terrible user experience
* Not realtime
* Poor scalability

---

### Decision

Rejected.

Modern collaborative systems avoid locking wherever possible.

---

## Approach 3 — Operational Transformation (OT)

Used historically by:

* Google Docs

Instead of sending entire state:

```text
Document
```

the system sends:

```text
Operation
```

Example:

```text
Insert "Hello"
Position 10
```

Operations are transformed relative to one another before application.

---

### Advantages

* Battle tested
* Efficient
* Proven at scale

---

### Disadvantages

* Complex transformation logic
* Difficult reasoning
* Requires coordination

---

### Decision

Rejected.

While OT is highly important historically, it hides many distributed systems concepts behind transformation logic.

Our goal is learning consistency, not reproducing Google Docs.

---

## Approach 4 — CRDT

CRDT:

```text
Conflict-Free Replicated Data Type
```

Core idea:

Design data structures that naturally converge.

Replicas can:

* diverge
* reconnect
* synchronize

without coordination.

Eventually:

```text
All replicas converge
```

to the same state.

---

### Advantages

* Distributed by design
* Offline friendly
* Partition tolerant
* Modern architecture

---

### Disadvantages

* More complex data structures
* Memory overhead — CRDTs retain metadata (tags, tombstones) and by default
  *never forget*; bounding this needs causal-stability garbage collection (a V2
  concern, made visible as a metric in V1)
* Requires careful design
* Op-based CRDTs assume reliable causal delivery — which an unreliable network
  does not provide, so we pair them with an operation log + anti-entropy resync
  (see §8a)

---

### Decision

Chosen.

CRDTs expose the distributed systems concepts we want to learn:

* Convergence
* Replication
* Consistency
* Conflict Resolution
* Synchronization

---

# 5. Why Not Build A Text Editor?

Initially the obvious idea appears to be:

```text
Google Docs Clone
```

However this introduces significant frontend complexity:

* Cursor synchronization
* Text selections
* Rich text rendering
* Formatting
* Copy/paste behavior

These concerns distract from the distributed systems problem.

The project risks becoming:

```text
Frontend Project
```

instead of:

```text
Distributed Systems Project
```

---

### Decision

Rejected.

---

# 6. Why Build A Workflow Graph?

Instead of text, SentinelSync synchronizes:

```text
Graph State
```

Example:

Email Trigger
|
v
AI Processor
|
v
Slack Notification

Operations become:

* Create Node
* Delete Node
* Rename Node
* Move Node
* Connect Nodes
* Disconnect Nodes

These map naturally to CRDT operations while remaining highly visual.

---

### Advantages

* Easy to visualize
* Easier than text CRDTs
* Rich enough for concurrency problems
* Demonstrates distributed state synchronization clearly

---

### Decision

Chosen.

---

# 7. Why A Simulation Dashboard Instead Of A Product UI?

The goal is understanding the system.

Not building another workflow SaaS.

Most collaborative projects fail because they focus on:

* UX
* Styling
* Features

instead of:

* Replication
* Convergence
* Consistency

SentinelSync therefore behaves more like:

* Redis Insight
* Kafka UI
* Grafana

than:

* Figma
* Notion

The dashboard exists solely to observe distributed behavior.

---

# 8. High-Level Architecture

The system consists of:

```text
Replica A
Replica B
Replica C
```

Each replica stores:

```text
Graph State
```

locally.

Every replica can:

* Receive operations
* Apply operations
* Replicate operations
* Merge operations

independently.

No central coordinator exists.

---

# 8a. Delivery Model (Hybrid)

A subtle but important honesty: an operation-based CRDT is only correct if
operations are delivered exactly once, in causal order, with no permanent loss.
SentinelSync deliberately breaks all of that (latency, packet loss, partitions,
crashes).

So SentinelSync is not a *pure* op-based CRDT. It is a hybrid:

```text
Op-based CRDT
+
Per-replica operation log
+
Anti-entropy reconciliation (replicas exchange vector clocks and replay
the operations a peer is missing)
```

The log + anti-entropy turn an unreliable network into the reliable causal
delivery the CRDT layer assumes — which is exactly how real distributed systems
behave. This makes resync a core feature, not a recovery afterthought.

---

# 9. Consistency Model

SentinelSync intentionally adopts:

```text
Eventual Consistency
```

not:

```text
Strong Consistency
```

Why?

Because eventual consistency is the natural environment for CRDTs.

The goal is:

```text
Replicas may temporarily disagree
```

but eventually:

```text
Replicas converge
```

to identical state.

This is the exact behavior used in many distributed systems.

---

# 10. Core Learning Objectives

By building SentinelSync we aim to gain hands-on experience with:

* CRDT Design
* Eventual Consistency
* Replica Synchronization
* Distributed Replication
* Conflict Resolution
* Vector Clocks (for sync + concurrency detection)
* Hybrid Logical Clocks (for LWW ordering without clock-skew bugs)
* Anti-Entropy Reconciliation
* Convergence Verification (canonical state hashing)
* Network Partitions
* State Reconciliation
* Distributed Simulations
* Real-Time Systems

---

# 11. Relationship To SentinelCache

SentinelCache explored:

* Failure Detection
* Leader Election
* Replication
* Availability
* Distributed Infrastructure

SentinelSync explores:

* Consistency
* Convergence
* Conflict Resolution
* Synchronization
* Distributed State

Together they cover two major pillars of distributed systems:

```text
SentinelCache
Availability

SentinelSync
Consistency
```

This combination provides significantly broader distributed systems understanding than either project alone.

