# Contract: SpiderIPPool IaaS-Pool Extension

This feature has no HTTP/gRPC/CLI-facing API surface of its own; its
"contract" is the Kubernetes CRD field/annotation shape shared between
Spiderpool (writer of `status.allocatedIPs`, reader of
`status.iaasReadyIPs`/`status.iaasFailedIPs`) and the external, private IaaS
provider controller (writer of `status.iaasReadyIPs`/`status.iaasFailedIPs`/
`status.conditions`). This document is the authoritative shape both sides must
honor. It is the CRD-field equivalent of an API contract for this project type
(Kubernetes controller + IPAM library), per plan.md step "Define interface
contracts... for the project type".

## Annotations (input contract — operator/provider writes, Spiderpool reads & validates)

```yaml
metadata:
  annotations:
    ipam.spidernet.io/iaas-pool: "true"            # required to opt a pool into all behavior in this feature
    ipam.spidernet.io/pair-pool: "<sibling-pool-name>"  # optional; required only for dual-stack paired pools
```

- `iaas-pool` MUST be exactly the string `"true"` to opt a pool into this
  feature's readiness-intersection behavior (any other value, including
  absence, means "not an IaaS pool" — full backward compatibility, FR-006).
  An `iaas-pool` pool's `spec.ips` MUST still be populated normally (the full
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
    ipam.spidernet.io/iaas-pool: "true"   # kept in sync with the annotation of the same key by the mutating webhook
```

Consumers (e.g., the external provider's `informer`) MUST use this label
(not the annotation) for server-side watch filtering, since Kubernetes watch
label selectors are server-side but annotation selectors are not.

## Validating Webhook Contract (SpiderIPPool create/update)

| Condition | Result |
|---|---|
| `pair-pool` == own pool name | `403 Forbidden` / `field.Invalid` — self-reference not allowed |
| `pair-pool` refers to an existing pool with the same `spec.ipVersion` | Rejected — same-version pairing not allowed |
| `pair-pool` refers to a pool that does not exist | Allowed (no rejection) |
| Both pools exist, v4 pool's (`spec.ips` - `spec.excludeIPs`) count > v6 pool's | Rejected |
| Both pools exist, v4 count <= v6 count | Allowed |
| Both pools exist, `spec.nodeName` or `spec.podAffinity` differ | Rejected |
| Both pools exist, `spec.nodeName`/`spec.podAffinity` identical | Allowed |
| No `pair-pool` annotation at all | No pairing validation applied (existing behavior) |

## Status Field Contract (`status.iaasReadyIPs`, `status.iaasFailedIPs`, `status.conditions`)

**Writer**: external IaaS provider controller, via Server-Side Apply with a
dedicated field manager (per proposal §3.3 single-writer principle).
**Reader**: Spiderpool IPAM (`AllocateIP`), read-only.

```yaml
status:
  allocatedIPs: '{...}'          # existing, Spiderpool-owned, unchanged shape
  totalIPCount: 64               # existing, unchanged
  allocatedIPCount: 12           # existing, unchanged
  iaasReadyIPs:                  # NEW, provider-owned — successfully prewarmed
    - ipv4: 192.168.1.10
      ipv6: fd00::10             # only present for paired pools; absent for single-stack pools
      mac: "fa:16:3e:xx:xx:xx"
      vlanID: 2014
  iaasFailedIPs:                 # NEW, provider-owned — attempted, not ready
    - ipv4: 192.168.1.11
      ipv6: fd00::11             # address-identity only, no mac/vlanID/error detail
  conditions:                    # NEW, provider-owned
    - type: IaasReady
      status: "True"              # "True" only when iaasFailedIPs is empty
      reason: AllReady             # or e.g. PartialPrewarmFailed
      message: "64/64 ready"
```

**Consumption rules Spiderpool MUST follow** (this is the binding part of the
contract for this feature's implementation):

1. Never write to `status.iaasReadyIPs`, `status.iaasFailedIPs`, or
   `status.conditions` — these fields are provider-owned. Spiderpool's own
   SSA/update calls MUST NOT include these fields (avoid field-manager
   conflicts).
2. Whether ledger-gating applies to a pool is decided **solely** by the
   `iaas-pool` label — NOT by whether `iaasReadyIPs`/`iaasFailedIPs` happen to
   be empty. For an `iaas-pool`-labeled pool, Spiderpool computes the normal
   `spec.ips`-derived available-candidate set (excluding `excludeIPs`,
   reserved IPs, and already-`allocatedIPs`) exactly as it does for any other
   pool, then **intersects** that candidate set with the addresses present in
   `status.iaasReadyIPs`. Only addresses in the intersection are allocatable;
   the first such address (ascending order) is selected and its
   `mac`/`vlanID` are copied onto the resulting Pod interface config.
3. If a pool does not carry the `iaas-pool` label, `iaasReadyIPs`/
   `iaasFailedIPs` MUST NOT be consulted at all, even if populated — behavior
   is byte-for-byte unchanged from before this feature (FR-011).
4. If the intersection computed in rule 2 is empty (including the case of a
   freshly-created pool with no `iaasReadyIPs` entries yet), Spiderpool
   returns the same `ErrIPUsedOut`-class error used for ordinary pool
   exhaustion — this is not a distinct/blocking error path.
5. `status.conditions[].type == "IaasReady"` is observational only — it MUST
   NOT gate or block allocation decisions (spec §5.3 / FR requirement: gate
   per-IP, not per-pool).

## Single-Ledger-On-Primary-Pool Model (authoritative)

There is exactly ONE ledger per paired pool-set, and it lives entirely on the
**primary pool**, which by convention is always the **v4** pool of a pair.
The v6 (sibling/non-primary) pool NEVER carries its own
`status.iaasReadyIPs`/`status.iaasFailedIPs` — those fields, even if present
in its CRD schema (they are generic, additive fields on every `SpiderIPPool`),
MUST NOT be populated by the provider for a non-primary pool. Consequences:

- **No single-stack consumption of a paired pool.** A Pod MAY NOT allocate
  only the v4 or only the v6 side of a pool that carries a `pair-pool`
  annotation — allocation from a paired `iaas-pool` is always all-or-nothing
  for the pair (see "Allocation Flow" below). This eliminates any scenario
  where one side of a ready pair is in use while the other is not, which in
  turn means the two pools' own independent `status.allocatedIPs` occupancy
  is always symmetric for a given ledger entry: if the v4 address is
  allocated, the v6 address is *always* also allocated (to the same Pod), and
  vice versa.
- Because occupancy is always symmetric, each pool's own existing
  `spec.ips`-shrink-while-in-use webhook validation (`validateIPPoolIPInUse`)
  is, by itself, a complete and sufficient safety net for the "address is
  currently used by a Pod" case on **either** side — no cross-pool occupancy
  check, and no "downgrade to single-stack ledger entry" fallback, is needed.
  (An earlier draft of this contract proposed such a downgrade path for an
  "in-use v4 / unused-sibling v6" scenario; that scenario cannot arise under
  the no-single-stack-consumption rule above and the downgrade design is
  withdrawn.)

## Allocation Flow (primary-pool-driven, sibling pool passive)

For a Pod resolved to allocate from a paired `iaas-pool` (v4 as the primary,
requested directly or via pool-name resolution), Spiderpool's `AllocateIP`
MUST follow this flow:

1. Compute the normal `spec.ips`-derived candidate set for the **v4 (primary)
   pool only** (existing logic, unchanged), intersect it with
   `v4pool.status.iaasReadyIPs` (matching on `ipv4`), and pick the first
   candidate per existing ordering rules. Do **not** separately compute or
   consult a candidate set for the v6 sibling pool — the v6 pool's own
   `spec.ips`/`excludeIPs` are consulted only by the provider, during
   prewarm (see below), not by Spiderpool at allocation time.
2. The selected `IaasReadyIPAllocation` entry already carries the paired
   `ipv6` address (and `mac`/`vlanID`) — read it directly from the same
   entry; there is no separate v6-side selection step.
3. **Lightweight sanity check** before finalizing: confirm the `ipv6` address
   is still present in the sibling pool's current `spec.ips` and does not
   already appear in the sibling pool's `status.allocatedIPs`. This is not a
   new full candidate-set computation — it is the same two basic checks any
   allocation already performs, just applied to the sibling pool's own
   fields, and guards against the (rare, provider-reconcile-lag) case where
   the v6 address was just removed from `spec.ips` but the ledger entry
   hasn't been cleaned up yet. If the check fails, treat it the same as an
   ordinary candidate miss (retry selection against the remaining
   intersection, or return `ErrIPUsedOut`-class error if exhausted).
4. On success, write `status.allocatedIPs` on **both** the v4 pool (ipv4) and
   the v6 pool (ipv6) atomically, as today.

This means the provider's prewarm step, not Spiderpool's allocation step, is
responsible for guaranteeing that any address recorded in
`v4pool.status.iaasReadyIPs` is simultaneously valid/available in both pools'
`spec.ips` (excluding `excludeIPs`, reserved ranges, etc. on both sides) at
the time it is prewarmed — see the corresponding provider-side document
(`docs/develop/proposal-iaas-ip-provider.md`) for that responsibility.

## Paired-Address Reconcile Contract (provider-owned, event-driven only)

When an address that is part of an `iaasReadyIPs` ledger entry is removed
from its own pool's `spec.ips` — regardless of whether the edit was made on
the v4 (primary) pool or the v6 (sibling) pool — the provider MUST reconcile
the ledger. This is the ONLY entity responsible for this reconciliation, and
it is entirely out of Spiderpool's scope; Spiderpool performs no cross-pool
writes of its own.

- **Level-triggered, not edge-triggered**: on every reconcile invocation
  (triggered by a `spec.ips` change event on either the v4 or the v6 pool),
  the provider MUST recompute, fresh, which `iaasReadyIPs` entries on the
  primary (v4) pool have an address (`ipv4` and/or `ipv6`) no longer present
  in the corresponding pool's current `spec.ips` — it MUST NOT rely on a
  remembered "what changed" diff. This makes the algorithm identical
  regardless of which side (v4 or v6) was edited, and idempotent/restart-safe
  (no persisted "in-progress" markers needed).
- **Prefer re-pairing over deleting.** For an entry where exactly one side's
  address has become invalid (no longer in that pool's `spec.ips`) while the
  other side's address remains valid:
  1. The provider SHOULD first look for a replacement address on the invalid
     side: an address present in that pool's current `spec.ips`, not already
     used by another ledger entry, and not currently allocated. If found, the
     provider SHOULD call the cloud API to re-attach the existing sub-ENI (or
     equivalent physical resource) with the new address, then update only the
     invalidated field (`ipv4` or `ipv6`) of the existing ledger entry to the
     new address — the entry's identity, `mac`, and `vlanID` are otherwise
     unchanged. This preserves the underlying paired resource instead of
     discarding it.
  2. Only if no replacement candidate exists on the invalid side does the
     provider fall back to a degraded outcome: clear just the invalid side's
     field from the ledger entry (leaving a single-family record for the
     still-valid side), without calling any cloud release API (the still-
     valid side's resource is untouched and still backed by the same
     sub-ENI).
  3. An entry is only fully deleted from `status.iaasReadyIPs` **and**
     released via the cloud API (`ReleaseIP`, which tears down the whole
     sub-ENI) when **both** sides' addresses are absent from their
     respective pools' `spec.ips` — i.e. the pair is genuinely orphaned on
     both ends, not just one.
  Because of the no-single-stack-consumption rule above, an entry undergoing
  this reconciliation is guaranteed to not be currently in use by a Pod (a
  used entry's addresses cannot be removed from `spec.ips` in the first
  place, on either side, thanks to each pool's own
  `validateIPPoolIPInUse` webhook) — so no occupancy re-check at
  cloud-API-call time is required beyond what the webhook already guarantees
  at the `spec.ips` write itself.
- **Trigger model — event-driven only, no periodic scan.** The provider MUST
  implement this purely as a reaction to watch/informer events on
  `iaas-pool`-labeled pools whose `spec.ips` changed. The provider MUST NOT
  run a periodic/scheduled reconciliation loop that scans pools for orphaned
  or out-of-sync ledger entries — this is an explicit scope exclusion for
  this iteration, deferred rather than permanently excluded (see also the
  `IaasFailedIPs` no-periodic-retry rule below). If any periodic behavior is
  implemented, it MUST be limited to read-only observability of
  currently-failed (`iaasFailedIPs`) addresses (IPv4-focused), and MUST NOT
  perform any write/reconcile/retry action.
- **Self-terminating, bounded hops.** Because the ledger lives only on the
  primary (v4) pool, a `spec.ips` edit on the v6 (sibling) pool triggers a
  reconcile that reads/writes only the v4 pool's `status.iaasReadyIPs` (it
  does not write anything to the v6 pool's own status, since the v6 pool has
  none). A subsequent `spec.ips` edit on the v4 pool (e.g. as a side effect of
  a re-pairing attempt failing and falling back to degrade) triggers a
  reconcile that reads/writes the same ledger again; the "recompute fresh
  every time" rule (above) means a second pass over an already-consistent
  ledger is a no-op. This bounds convergence to at most two reconcile passes
  and requires no separate check-before-write guard beyond normal idempotency.
- **Prewarm-attempt retries are unaffected by the no-periodic-scan rule**:
  the exclusion above only rules out periodic/scheduled re-scanning of
  already-recorded outcomes (both `IaasFailedIPs` entries and orphaned-pair
  detection). It does NOT prohibit the provider from retrying a bounded
  number of times (e.g. up to 3 attempts) synchronously within a single
  prewarm/re-pairing attempt before giving up and recording the address into
  `iaasFailedIPs` or falling back to the degraded single-family outcome.

## Backward Compatibility Statement

Every field/annotation/label in this contract is additive and optional.
Existing `SpiderIPPool` objects, existing Helm-rendered manifests, and
existing controllers/clients that do not know about
`iaasReadyIPs`/`iaasFailedIPs`/`conditions` continue to function unchanged.
CRD schema changes MUST be validated with `make manifests generate-k8s-api`
producing only additive diffs to
`charts/spiderpool/crds/spiderpool.spidernet.io_spiderippools.yaml`.
