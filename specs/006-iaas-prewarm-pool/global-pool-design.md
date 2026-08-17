# Design: Global Pool Mode (Realtime Allocation + Sticky Sub-ENI Cache)

**Status**: Draft (finalized in design discussion, 2026-08-17)

**Parent spec**: `006-iaas-prewarm-pool` — this document extends the prewarm
design with a second IaaS pool mode. It changes no prewarm behavior other
than the metadata schema upgrade (§2).

## 1. Motivation

The node-level prewarm pool binds one pool (and all of its sub-ENIs) to one
node. Customers who insist on the **1 Deployment : 1 IPPool** model with a
modest IP surplus (e.g. 32 Pods / 64 IPs across 10 nodes) cannot split such
a pool into per-node slices: the scheduler is IP-unaware and per-node slices
(~6 IPs each) are too small to survive scheduling skew and rolling-update
surge. Realtime cloud allocation matches the 1:1 model but puts cloud API
latency and throttling on the CNI ADD critical path.

**Global pool mode** keeps the 1 deploy : 1 pool user model and recovers
prewarm-class restart latency by making sub-ENIs *sticky*: they are created
on first use (realtime), kept bound to the node after the Pod is deleted
(cache), and reused with zero cloud calls when a new Pod lands on the same
node. Reclaim is watermark-driven, not time-driven.

| Scenario | Cloud calls | Latency class |
|---|---|---|
| First Pod on an IP | 1 (create+attach) | realtime mode |
| Pod restart on same node (cache hit) | 0 | prewarm mode |
| Pod moved to another node | 2 (detach+attach) | slow path |
| Pod delete | 0 | — |

## 2. Metadata schema v2

`status.ipMetaData.metadata` (still a JSON-encoded string, provider-owned)
changes its decoded shape:

```jsonc
// Node-level pool (prewarm mode; node identity comes from spec.nodeName)
{
  "scope": "node-50",
  "parentNic": "enp11s0f0np0",
  "ips": {
    "192.168.110.10": { "ipv6": "fd00:110::10", "mac": "fa:16:3e:aa:01", "vlan": 100 },
    "192.168.110.11": { "ipv6": "fd00:110::11", "mac": "fa:16:3e:aa:02", "vlan": 100 }
  }
}
```

```jsonc
// Global pool (realtime + sticky cache)
{
  "scope": "",                        // explicit empty string = global
  "parentNic": "enp11s0f0np0",
  "ips": {
    "192.168.130.10": { "ipv6": "fd00:130::10", "mac": "fa:16:3e:bb:01", "vlan": 100, "node": "node-50" },
    "192.168.130.11": { "ipv6": "fd00:130::11", "mac": "fa:16:3e:bb:02", "vlan": 100, "node": "node-60" },
    "192.168.130.12": { "ipv6": "fd00:130::12", "mac": "fa:16:3e:bb:03", "vlan": 100 }
    // entry without "node" = sub-ENI created but currently detached
  }
}
```

Rules:

- `scope` is **mandatory**. `"<nodeName>"` = node-level pool (all IPs bound
  to that node; `ips[].node` MUST NOT appear; the value MUST equal
  `spec.nodeName`). `""` = global pool (per-IP placement comes from
  `ips[ip].node`; a missing `node` means created-but-unbound). An explicit
  empty string doubles as the "provider has initialized this pool" marker —
  a missing `scope` (or missing metadata) means not yet reconciled.
- `parentNic` stays pool-level: one pool maps to one parent NIC name, which
  is identical across nodes.
- **`vlan: -1` is the detaching sentinel.** The cloud API keeps ip/mac
  stable across detach but assigns a new VLAN on every attach, so a cached
  `vlan` is only trustworthy while the sub-ENI stays attached. Before
  detaching, the provider sets the entry's `vlan` to `-1` (the reclaim race
  guard write of §5). Semantics for readers:
  - `detaching(ip) = ips[ip].node present && ips[ip].vlan == -1` — the IP
    MUST NOT be allocated at all (skipped in both the hit predicate and the
    cold-path candidate set) until the provider finishes the transition;
  - an unbound entry (no `node`) may keep `vlan: -1` after the detach
    completes; it remains a normal cold-path candidate because the RPC
    response — not the cached entry — supplies the authoritative new VLAN;
  - a hit additionally requires `vlan != -1`, so a Pod can never be
    configured with a stale VLAN of a detached (or detaching) sub-ENI.
- Effective placement and the cache-hit predicate (uniform across modes):

```
effectiveNode(ip) = scope != "" ? scope : ips[ip].node
detaching(ip)     = ips[ip].node present && ips[ip].vlan == -1
hit(ip)           = effectiveNode(ip) == localNode && ip ∉ status.allocatedIPs
                    && ips[ip].vlan != -1
```

- Writers keep emitting v2 only; readers accept the legacy shape (top-level
  address keys + reserved `parentNic` key) during migration.
- For paired pools, metadata continues to exist only on the v4 primary pool.

## 3. Ownership contract (unchanged, now shared by both modes)

| Artifact | Sole writer | Readers |
|---|---|---|
| `status.allocatedIPs` | Spiderpool (agent IPAM) | provider (read-only, to derive idleness) |
| `status.ipMetaData` | provider | Spiderpool (read-only, informer cache) |
| Cloud API | provider | — |

The provider's **in-memory state is authoritative** for cloud facts; the CR
is a persisted view (asynchronously flushed) plus a bootstrap accelerator.
All provider decisions (allocate RPC, reclaim, GC) MUST read memory, never
the CR. Recovery rebuilds memory by listing cloud sub-ENIs via `{pool, ip}`
tags. This requires a single active provider instance (leader election).

Derived state (never stored):

```
idle(ip)  = bound(ip) && ip ∉ allocatedIPs     // reusable cache
bound     = |ips entries with node|
created   = |ips entries|
unbound   = created - bound
free      = |spec.ips| - created
```

## 4. Allocation flow (global pool)

### 4.1 Hot path — cache hit (zero cloud calls, zero RPC)

CNI ADD → agent evaluates `hit(ip)` over the informer metadata snapshot →
commits `allocatedIPs` (existing optimistic-lock path) → configures the Pod
interface from cached `{ipv6, mac, vlan}`. Equivalent to prewarm-mode speed.

### 4.2 Cold path — cache miss (synchronous provider RPC)

1. IPAM selects a candidate inside the ordinary availability set
   (`spec.ips` ∖ `excludeIPs` ∖ reserved ∖ `allocatedIPs`), ordered by:
   1. **unbound** IPs (entry without `node`, or no entry): 1 cloud call;
   2. **idle on another node** (LRU coldest): 2 cloud calls (steal).
2. Agent commits the `allocatedIPs` claim first (mutual exclusion), then
   synchronously calls the provider allocation RPC
   `Allocate{pool, ipv4, ipv6, targetNode, podRef}`.
3. Provider executes according to its in-memory state — not exists →
   create+attach; unbound → attach; bound elsewhere → detach+attach; bound
   on `targetNode` → idempotent no-op — and returns `{ipv6, mac, vlan}`.
4. Agent configures the Pod interface from the RPC response immediately; it
   does **not** wait for metadata persistence.
5. Provider updates memory synchronously and flushes `ips[ip]` to the CR
   asynchronously (§6). On RPC failure the agent rolls back its
   `allocatedIPs` claim and fails the ADD (kubelet backoff retries).

The RPC MUST be idempotent per `{pool, ipv4, targetNode}` and return typed
errors (`CapacityExceeded`, `CloudThrottled`, ...).

### 4.3 Delete path

CNI DEL removes the `allocatedIPs` entry only. **No provider call, no cloud
operation.** The IP silently becomes derived-idle cache on its node; the
provider observes the pool update and records `idleSince` in memory.

### 4.4 Async-window effects (accepted)

Because metadata flush lags reality, two benign races exist; both resolve
correctly because the provider decides from memory and the RPC is
idempotent:

- an IP freshly attached then quickly freed may look unbound to another
  node → provider does detach+attach instead of create (1 extra call);
- a freshly idle IP may be invisible to a local Pod → wasted RPC that the
  provider answers from memory with zero cloud calls.

The `allocatedIPs` claim (synchronous, spiderpool-owned) is what prevents
double allocation; it has no async window.

## 5. Watermark reclaim (provider-internal)

Reclaim lives in the provider, symmetric to prewarm ("prewarm = create
ahead of demand, reclaim = destroy behind demand"), reusing its informer,
workqueue, cloud client, and rate limiter.

User-facing configuration: **two global thresholds only** (provider flags,
applied to every global pool):

| Flag | Default | Semantics |
|---|---|---|
| `detachThreshold` | 60% | while `bound/total ≥ 60%`: detach idle entries (LRU coldest) — sub-ENI/port kept, entry loses `node` |
| `deleteThreshold` | 90% | while `created/total ≥ 90%`: delete the longest-unbound sub-ENIs — entry removed |

Two-stage waterfall, each threshold owning one transition:
`bound →(60%, mobility)→ unbound →(90%, port budget)→ deleted`. Reclaim
only ever touches idle/unbound IPs; in-use IPs are untouchable, so a
triggered threshold with nothing idle is a safe no-op.

Trigger machinery (event-driven, non-blocking):

- pool informer Update → predicate: provider annotation present, and the
  `allocatedIPs` count changed (**self-caused metadata-only updates are
  filtered out to prevent self-triggering**) → pool key enqueued into a
  deduplicating, debounced (seconds) workqueue → one reclaim goroutine
  evaluates the snapshot; informer resync is the periodic fallback.
- per round: bounded batch (e.g. 10) on a **separate low-QPS cloud budget**;
  allocation RPCs never queue behind reclaim.
- race guard: before detaching, the provider marks the entry as detaching
  by setting its `vlan` to `-1` in metadata (one CR write; same-object
  resourceVersion serializes it against agent `allocatedIPs` claims),
  re-reads `allocatedIPs`, and aborts (restoring the real `vlan`) if the
  IP was claimed. Agents treat `node`-present + `vlan == -1` entries as
  detaching and skip them in hit/candidate sets. The sentinel is not an
  extra field: detach genuinely invalidates the cached VLAN (the cloud
  reassigns it on the next attach), so `-1` doubles as "VLAN unknown".
- **hard, non-configurable guard**: when a node's total sub-ENI count
  (across pools) approaches the parent-NIC limit (256), that node's idle
  cache is reclaimed unconditionally and first — correctness, not policy.
- reclaim state transitions are flushed to the CR **in real time** (they
  are low-frequency; prompt visibility of `unbound` restores the 1-call
  fast path for other nodes).

## 6. Metadata flush discipline

- Allocation-path updates: in-memory dirty-set + per-pool serial flush
  queue; debounce 500ms–1s or size-triggered. Each flush writes the current
  per-IP **snapshot** (last-wins), never an event log, so conflict retries
  are idempotent. Snapshot versioning: entries dirtied during an in-flight
  flush stay dirty for the next round.
- Reclaim/GC transitions: real-time single writes (§5).
- The two status writers (`allocatedIPs` vs `ipMetaData`) use SSA with
  distinct field managers (or field-scoped patches), so they do not
  conflict on fields; object-level optimistic concurrency is retained
  deliberately as the serialization point for the reclaim race guard.

## 7. Paired (dual-stack) pools in global mode

Pairing is **dynamic at creation, sticky afterwards** — there is no index
rule. This matches the existing prewarm implementation, where the pair is
whatever the metadata entry records.

- Reused as-is: `ipam.spidernet.io/pair-pool` validation
  (`ippool_validate.go`), sibling-v6 shielding in `selectByPod`
  (`pkg/ipam/allocate.go`), and the pair-or-nothing commit machinery
  `AllocateIPPair` (`pkg/ippoolmanager/ippool_manager.go`) including its
  conflict-retry + Pod-UID convergence fast path.
- Hit path: select an entry with `hit(v4)` (global pools add the
  `effectiveNode == localNode` condition to `FindReadyIPPairMetadata`);
  both families come from the same entry.
- Cold path: IPAM picks a free v4 (candidate ordering of §4.2) and **any
  free v6**, passes both to the RPC; the provider creates one dual-stack
  sub-ENI; the entry's `ipv6` value becomes the sticky pair for the
  sub-ENI's lifetime. Commit reuses the existing two-pool bookkeeping.
- v6 availability = existing filter chain (`spec.ips`, `excludeIPs`,
  SpiderReservedIP, `allocatedIPs`) **plus one new exclusion: any address
  referenced by an existing metadata `entry.ipv6`** (a cached sub-ENI locks
  its v6 even while no Pod uses it). Deleting the sub-ENI frees both sides.
- Non-provider (static) pools gain nothing new: dynamic pairing degenerates
  into the existing independent dual-stack allocation, so the pair-pool
  annotation stays a provider-mode concept.

## 8. Component responsibilities

**Spiderpool** (allocation bookkeeping and placement policy):

- agent IPAM: hit predicate with node filtering; three-tier cold-path
  candidate ordering; v6 metadata-reference exclusion; synchronous
  allocation RPC client with claim rollback on failure.
- No new controller machinery: Pod DEL already updates the pool CR, which
  *is* the reclaim trigger signal — no per-agent queues.

**Provider** (cloud resource lifecycle: prewarm, allocate-on-demand,
reclaim, reconcile):

- schema v2 writer (+ legacy reader);
- global-pool recognition (annotation present, no `spec.nodeName`) and
  empty-metadata initialization, no prewarming;
- idempotent synchronous `Allocate` RPC (memory-authoritative);
- async snapshot flusher; real-time reclaim writes;
- watermark reclaim goroutine with the two thresholds, event predicates,
  node-capacity hard guard;
- `{pool, ip}` tagging, startup cloud list to rebuild memory, periodic
  drift reconciliation and orphan cleanup (bindings whose podRef vanished
  before the RPC response was consumed).

## 9. Explicit non-goals

- No scheduler awareness (extended resources / scheduler plugin): the
  sticky cache self-adapts to the actual scheduling distribution, which is
  the point of this mode. Scheduler-visible capacity remains a possible
  follow-up for the node-level prewarm mode.
- No per-node quota planning or rebalancing of cached sub-ENIs; watermarks
  plus the LRU steal path replace them.
- No TTL-based reclaim by default (a TTL can be added later as an optional
  cost-control knob for clouds that bill per port).
