# Contract: SpiderIPPool IaaS-Pool Extension

This feature has no HTTP/gRPC/CLI-facing API surface of its own; its
"contract" is the Kubernetes CRD field/annotation shape shared between
Spiderpool (writer of `status.allocatedIPs`, reader of `status.ipMetaData`)
and the external, private IaaS provider controller (writer of
`status.ipMetaData`). This document is the authoritative shape both sides
must honor. It is the CRD-field equivalent of an API contract for this
project type (Kubernetes controller + IPAM library), per plan.md step
"Define interface contracts... for the project type".

> **Revision note (v5 alignment)**: the former `status.iaasReadyIPs`/
> `status.iaasFailedIPs` list ledgers and `status.conditions` are replaced by
> a single cloud-neutral `status.ipMetaData` structure, and the
> `ipam.spidernet.io/iaas-pool: "true"` marker is replaced by
> `ipam.spidernet.io/iaas-provider: "<vendor>"`.
>
> **Revision note (v6, 2026-08-12)**: `ipMetaData.metadata` is a JSON string
> encoding the logical per-IP map, and `observedGeneration` identifies the
> pool spec generation for which the provider completed reconciliation.
> Spiderpool consumes an immutable parsed cache snapshot and fails closed
> while generation and observed generation differ. No `phase` is added.

## Annotations (input contract — operator/provider writes, Spiderpool reads & validates)

```yaml
metadata:
  annotations:
    ipam.spidernet.io/iaas-provider: "huaweicloud"      # required to opt a pool into all behavior in this feature
    ipam.spidernet.io/pair-pool: "<sibling-pool-name>"  # optional; required only for dual-stack paired pools
```

- `iaas-provider` MUST be a supported vendor name to opt a pool into this
  feature's readiness-intersection behavior. The supported vendor list is
  currently exactly `huaweicloud`; the validating webhook rejects any other
  non-empty value. Absence of the annotation means "not an IaaS pool" — full
  backward compatibility, FR-006.
  An IaaS pool's `spec.ips` MUST still be populated normally (the full
  range the provider is expected to prewarm from) — it is NOT left empty and
  is NOT a separate address space; the annotation only means "addresses here
  require provider prewarm confirmation before allocation," not "addresses
  come from somewhere other than `spec.ips`".
- `pair-pool` MUST name another `SpiderIPPool` object (may not yet exist at
  admission time). Validation rules are enforced only once both pools exist
  (see below).

## Label (output contract — Spiderpool writes, provider/external watchers read)

```yaml
metadata:
  labels:
    ipam.spidernet.io/iaas-provider: "huaweicloud"   # kept in sync with the annotation of the same key by the mutating webhook
```

Consumers (e.g., the external provider's `informer`) MUST use this label
(not the annotation) for server-side watch filtering, since Kubernetes watch
label selectors are server-side but annotation selectors are not. A provider
MAY additionally filter by its own vendor value
(`ipam.spidernet.io/iaas-provider=huaweicloud`).

## Validating Webhook Contract (SpiderIPPool create/update)

| Condition | Result |
|---|---|
| `iaas-provider` set to an unsupported vendor value | Rejected — supported list is currently `huaweicloud` |
| `pair-pool` == own pool name | `403 Forbidden` / `field.Invalid` — self-reference not allowed |
| `pair-pool` refers to an existing pool with the same `spec.ipVersion` | Rejected — same-version pairing not allowed |
| `pair-pool` refers to a pool that does not exist | Allowed (no rejection) |
| Both pools exist, v4 pool's (`spec.ips` - `spec.excludeIPs`) count > v6 pool's | Rejected |
| Both pools exist, v4 count <= v6 count | Allowed |
| Both pools exist, `spec.nodeName` or `spec.podAffinity` differ | Rejected |
| Both pools exist, `spec.nodeName`/`spec.podAffinity` identical | Allowed |
| No `pair-pool` annotation at all | No pairing validation applied (existing behavior) |

## Status Field Contract (`status.ipMetaData`)

**Writer**: external IaaS provider controller, via Server-Side Apply with a
dedicated field manager (per proposal §3.3 single-writer principle).
**Reader**: Spiderpool IPAM (`AllocateIP`), read-only.

```yaml
status:
  allocatedIPs: '{...}'          # existing, Spiderpool-owned, unchanged shape
  totalIPCount: 64               # existing, unchanged
  allocatedIPCount: 12           # existing, unchanged
  ipMetaData:                    # NEW, provider-owned (primary pool only)
    parentNic: eth0              # pool-level parent NIC on the bound node
    metadata: '{"192.168.1.10":{"ipv6":"fd00::10","mac":"fa:16:3e:aa:bb:cc","vlan":2014},"192.168.1.12":{"ipv6":"fd00::12","mac":"fa:16:3e:dd:ee:ff","vlan":2015}}'
    observedGeneration: 7        # generation fully reconciled by provider
    readyIPCount: 2              # number of IPs WITH a metadata entry (= prewarmed)
    unreadyIPCount: 4            # number of spec.ips IPs WITHOUT a metadata entry (= unready/failed)
```

The decoded `metadata` payload type is
`map[string]IPMetadataEntry`; only its Kubernetes storage representation is a
string. Consumers MUST validate decoding before use.

There is deliberately NO `status.conditions` and NO per-IP failure list:
prewarm failure is expressed purely as absence from `metadata` (counted in
`unreadyIPCount`); per-IP failure detail lives in provider logs only.

**Consumption rules Spiderpool MUST follow** (this is the binding part of the
contract for this feature's implementation):

1. Never write to `status.ipMetaData` — the field is provider-owned.
   Spiderpool's own SSA/update calls MUST NOT include this field (avoid
   field-manager conflicts).
2. Whether metadata-gating applies to a pool is decided **solely** by the
   `iaas-provider` label — NOT by whether `ipMetaData.metadata` happens to
   be empty. Before consuming metadata, Spiderpool MUST require
   `ipMetaData.observedGeneration == metadata.generation`; mismatch means the
   provider has not published a result for the current spec and allocation
   MUST fail closed with a retryable error.
3. For a generation-matched IaaS pool, Spiderpool reads an immutable,
   successfully decoded agent-cache snapshot for the same pool UID and
   observed generation, computes the normal
   `spec.ips`-derived available-candidate set (excluding `excludeIPs`,
   reserved IPs, and already-`allocatedIPs`) exactly as it does for any other
   pool, then **intersects** that candidate set with the keys of
   the decoded metadata. Only addresses in the intersection are
   allocatable; the first such address (ascending order) is selected and its
   entry's `mac`/`vlan` are copied onto the resulting Pod interface config.
4. If a pool does not carry the `iaas-provider` label, `ipMetaData` MUST NOT
   be consulted at all, even if populated — behavior is byte-for-byte
   unchanged from before this feature (FR-011).
5. If the intersection computed above is empty (including the case of a
   freshly-created pool with no `metadata` entries yet), Spiderpool returns
   the same `ErrIPUsedOut`-class error used for ordinary pool exhaustion —
   this is not a distinct/blocking error path.
6. A malformed JSON string or missing/current-generation cache snapshot MUST
   fail closed; Spiderpool MUST NOT use a prior snapshot or fall back to
   ordinary un-prewarmed allocation.
7. `readyIPCount`/`unreadyIPCount` are observational only — they MUST NOT
   gate or block allocation decisions (spec §5.3 / FR requirement: gate
   per-IP, not per-pool).

## Generation Publication and Cache Contract

The provider MUST treat the following as one atomic publication:

```text
metadata + readyIPCount + unreadyIPCount + observedGeneration
```

- It advances `observedGeneration` to the pool's current generation after a
  complete, trustworthy evaluation. Partial per-IP failures are a valid
  completed result: successful entries are included, failed entries are
  absent, and the counters describe both groups.
- If reconciliation aborts, cannot produce a trustworthy full snapshot, or
  cannot persist the atomic status publication, it leaves
  `observedGeneration` unchanged.
- A spec edit naturally creates `generation > observedGeneration`; no webhook
  status mutation and no separate `phase=Updating` transition is needed.

Spiderpool's pool informer receives both spec and pure status Update events.
On each relevant event the agent:

1. disables allocation immediately when generation and observed generation
   differ;
2. when they match, decodes the metadata string once and atomically installs
   an immutable snapshot keyed by pool UID and observed generation;
3. reuses the parsed snapshot across Pod allocations;
4. does not reparse when only `resourceVersion`/`allocatedIPs` changed and the
   observed generation and metadata content are unchanged;
5. evicts snapshots when the pool is deleted or its UID changes.

The cache is never authoritative. Agent restart/informer replay reconstructs
it from status. A cache miss is a closed gate, not permission to use stale
metadata.

## Single-Metadata-On-Primary-Pool Model (authoritative)

There is exactly ONE `ipMetaData` per paired pool-set, and it lives entirely
on the **primary pool**, which by convention is always the **v4** pool of a
pair. The v6 (sibling/non-primary) pool NEVER carries its own
`status.ipMetaData` — the field, even if present in its CRD schema (it is a
generic, additive field on every `SpiderIPPool`), MUST NOT be populated by
the provider for a non-primary pool. Consequences:

- **No single-stack consumption of a paired pool.** A Pod MAY NOT allocate
  only the v4 or only the v6 side of a pool that carries a `pair-pool`
  annotation — allocation from a paired IaaS pool is always all-or-nothing
  for the pair (see "Allocation Flow" below). This eliminates any scenario
  where one side of a ready pair is in use while the other is not, which in
  turn means the two pools' own independent `status.allocatedIPs` occupancy
  is always symmetric for a given metadata entry: if the v4 address is
  allocated, the v6 address is *always* also allocated (to the same Pod), and
  vice versa.
- Because occupancy is always symmetric, each pool's own existing
  `spec.ips`-shrink-while-in-use webhook validation (`validateIPPoolIPInUse`)
  is, by itself, a complete and sufficient safety net for the "address is
  currently used by a Pod" case on **either** side — no cross-pool occupancy
  check, and no "downgrade to single-stack entry" fallback, is needed at
  allocation time. (Degradation of a metadata entry remains a provider-side
  reconcile outcome, never a Spiderpool allocation-time behavior.)

## Allocation Flow (primary-pool-driven, sibling pool passive)

For a Pod resolved to allocate from a paired IaaS pool (v4 as the primary,
requested directly or via pool-name resolution), Spiderpool's `AllocateIP`
MUST follow this flow:

1. Compute the normal `spec.ips`-derived candidate set for the **v4 (primary)
   pool only** (existing logic, unchanged), intersect it with the keys of
   the decoded `v4pool.status.ipMetaData.metadata`, and pick the first candidate per
   existing ordering rules. Do **not** separately compute or consult a
   candidate set for the v6 sibling pool — the v6 pool's own
   `spec.ips`/`excludeIPs` are consulted only by the provider, during
   prewarm (see below), not by Spiderpool at allocation time.
2. The selected `IPMetadataEntry` already carries the paired `ipv6` address
   (and `mac`/`vlan`) — read it directly from the same entry; there is no
   separate v6-side selection step.
3. **Lightweight sanity check** before finalizing: confirm the `ipv6` address
   is still present in the sibling pool's current `spec.ips` and does not
   already appear in the sibling pool's `status.allocatedIPs`. This is not a
   new full candidate-set computation — it is the same two basic checks any
   allocation already performs, just applied to the sibling pool's own
   fields, and guards against the (rare, provider-reconcile-lag) case where
   the v6 address was just removed from `spec.ips` but the metadata entry
   hasn't been cleaned up yet. If the check fails, treat it the same as an
   ordinary candidate miss (retry selection against the remaining
   intersection, or return `ErrIPUsedOut`-class error if exhausted).
4. On success, write `status.allocatedIPs` on **both** the v4 pool (ipv4) and
   the v6 pool (ipv6) atomically, as today.

This means the provider's prewarm step, not Spiderpool's allocation step, is
responsible for guaranteeing that any address recorded in
`v4pool.status.ipMetaData.metadata` is simultaneously valid/available in both
pools' `spec.ips` (excluding `excludeIPs`, reserved ranges, etc. on both
sides) at the time it is prewarmed — see the corresponding provider-side
proposal document for that responsibility.

## Paired-Address Reconcile Contract (provider-owned, event-driven only)

When an address that is part of an `ipMetaData.metadata` entry is removed
from its own pool's `spec.ips` — regardless of whether the edit was made on
the v4 (primary) pool or the v6 (sibling) pool — the provider MUST reconcile
the metadata. This is the ONLY entity responsible for this reconciliation,
and it is entirely out of Spiderpool's scope; Spiderpool performs no
cross-pool writes of its own.

- **Level-triggered, not edge-triggered**: on every reconcile invocation
  (triggered by a `spec.ips` change event on either the v4 or the v6 pool),
  the provider MUST recompute, fresh, which `metadata` entries on the
  primary (v4) pool have an address (key and/or `ipv6`) no longer present
  in the corresponding pool's current `spec.ips` — it MUST NOT rely on a
  remembered "what changed" diff. This makes the algorithm identical
  regardless of which side (v4 or v6) was edited, and idempotent/restart-safe
  (no persisted "in-progress" markers needed).
- **Prefer re-pairing over deleting.** For an entry where exactly one side's
  address has become invalid (no longer in that pool's `spec.ips`) while the
  other side's address remains valid:
  1. The provider SHOULD first look for a replacement address on the invalid
     side: an address present in that pool's current `spec.ips`, not already
     used by another metadata entry, and not currently allocated. If found,
     the provider SHOULD call the cloud API to re-attach the existing sub-ENI
     (or equivalent physical resource) with the new address, then update the
     entry accordingly — `mac`/`vlan` are unchanged; note that a v4-side
     replacement changes the map key itself (delete old key, insert new key
     with the same entry value).
  2. Only if no replacement candidate exists on the invalid side does the
     provider fall back to a degraded outcome: a lost v6 side clears the
     entry's `ipv6` field; a lost v4 side re-keys the entry by its `ipv6`
     address (single-family record) — in both cases without calling any
     cloud release API (the still-valid side's resource is untouched and
     still backed by the same sub-ENI).
  3. An entry is only fully deleted from `metadata` **and** released via the
     cloud API (`ReleaseIP`, which tears down the whole sub-ENI) when
     **both** sides' addresses are absent from their respective pools'
     `spec.ips` — i.e. the pair is genuinely orphaned on both ends, not just
     one.
  After any of these outcomes the provider MUST recompute and atomically
  rewrite metadata, `readyIPCount`/`unreadyIPCount`, and
  `observedGeneration` for the reconciled generation in the same SSA apply.
  Because of the no-single-stack-consumption rule above, an entry undergoing
  this reconciliation is guaranteed to not be currently in use by a Pod (a
  used entry's addresses cannot be removed from `spec.ips` in the first
  place, on either side, thanks to each pool's own
  `validateIPPoolIPInUse` webhook) — so no occupancy re-check at
  cloud-API-call time is required beyond what the webhook already guarantees
  at the `spec.ips` write itself.
- **Trigger model — event-driven only, no periodic scan.** The provider MUST
  implement this purely as a reaction to watch/informer events on
  IaaS-labeled pools whose `spec.ips` changed. The provider MUST NOT run a
  periodic/scheduled reconciliation loop that scans pools for orphaned or
  out-of-sync metadata entries — this is an explicit scope exclusion for
  this iteration, deferred rather than permanently excluded (see also the
  no-periodic-retry rule below). If any periodic behavior is implemented, it
  MUST be limited to read-only observability of currently-unready addresses
  (`spec.ips` minus `metadata` keys, IPv4-focused), and MUST NOT perform any
  write/reconcile/retry action.
- **Self-terminating, bounded hops.** Because the metadata lives only on the
  primary (v4) pool, a `spec.ips` edit on the v6 (sibling) pool triggers a
  reconcile that reads/writes only the v4 pool's `status.ipMetaData` (it
  does not write anything to the v6 pool's own status, since the v6 pool has
  none). A subsequent `spec.ips` edit on the v4 pool (e.g. as a side effect of
  a re-pairing attempt failing and falling back to degrade) triggers a
  reconcile that reads/writes the same metadata again; the "recompute fresh
  every time" rule (above) means a second pass over already-consistent
  metadata is a no-op. This bounds convergence to at most two reconcile
  passes and requires no separate check-before-write guard beyond normal
  idempotency.
- **Prewarm-attempt retries are unaffected by the no-periodic-scan rule**:
  the exclusion above only rules out periodic/scheduled re-scanning of
  already-recorded outcomes (both failed-prewarm absence and orphaned-pair
  detection). It does NOT prohibit the provider from retrying a bounded
  number of times (e.g. up to 3 attempts) synchronously within a single
  prewarm/re-pairing attempt before giving up — a failed address simply
  stays absent from `metadata` and is counted in `unreadyIPCount`.

## Backward Compatibility Statement

The annotations and `ipMetaData` status block remain optional, so non-IaaS
pools and existing manifests are unaffected. The v6 draft changes
`ipMetaData.metadata` from a structural map to a string; because this feature
is still on its feature branch, existing draft status data MUST be cleared or
migrated before applying the regenerated schema. `observedGeneration` is
additive. CRD schema changes MUST be generated from Go source using
`make manifests generate-k8s-api`, never edited manually.
