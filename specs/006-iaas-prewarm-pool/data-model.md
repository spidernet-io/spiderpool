# Phase 1 Data Model: IaaS Provider Prewarm IP Pool Support

All entities below are additive extensions of the existing
`SpiderIPPool` CRD (`pkg/k8s/apis/spiderpool.spidernet.io/v2beta1/spiderippool_types.go`).
No new CRD kind is introduced (constitution/proposal constraint). Field names
below are proposed Go/JSON names; final casing/ordering must satisfy
`make manifests generate-k8s-api` output review during implementation.

## 1. SpiderIPPool (extended)

### 1.1 New Annotations (metadata.annotations, user/operator or provider set)

| Annotation | Type | Required | Notes |
|---|---|---|---|
| `ipam.spidernet.io/iaas-pool` | string (`"true"`/`"false"`) | No | Marks the pool as IaaS-managed, i.e. addresses in `spec.ips` require provider prewarm confirmation before they may be allocated. Absence = pool behaves exactly as today (FR-006). Presence = allocation always intersects with `status.iaasReadyIPs`, regardless of whether that list is currently empty (see §1.3). |
| `ipam.spidernet.io/pair-pool` | string (pool name) | No | Names the dual-stack sibling `SpiderIPPool`. Only meaningful when `iaas-pool` is also set to `"true"`, though validation does not strictly require co-presence beyond the rules in §2. |

### 1.2 New Label (metadata.labels, system-managed)

| Label | Type | Managed by | Notes |
|---|---|---|---|
| `ipam.spidernet.io/iaas-pool` | string (mirrors annotation value) | Mutating webhook (`ippool_mutate.go`) | Annotation is authoritative; label is corrected to match on every create/update, following the `LabelIPPoolCIDR` precedent. Exists purely so an external watcher (the private provider) can use an efficient label-selector watch — spiderpool itself does not filter by this label. |

### 1.3 New Status Fields (status, subresource)

```go
// IPPoolStatus defines the observed state of SpiderIPPool.
type IPPoolStatus struct {
    AllocatedIPs     *string `json:"allocatedIPs,omitempty"`     // existing, unchanged
    TotalIPCount     *int64  `json:"totalIPCount,omitempty"`     // existing, unchanged
    AllocatedIPCount *int64  `json:"allocatedIPCount,omitempty"` // existing, unchanged

    // +kubebuilder:validation:Optional
    IaasReadyIPs []IaasReadyIPAllocation `json:"iaasReadyIPs,omitempty"` // NEW

    // +kubebuilder:validation:Optional
    IaasFailedIPs []IaasFailedIPAllocation `json:"iaasFailedIPs,omitempty"` // NEW

    // +kubebuilder:validation:Optional
    Conditions []metav1.Condition `json:"conditions,omitempty"` // NEW
}

// IaasReadyIPAllocation is one per-IP (or per-IP-pair, for paired pools)
// successfully-prewarmed ledger entry. Written by the external IaaS provider
// controller via Server-Side Apply with its own field manager; consumed
// read-only by Spiderpool's IPAM. Spiderpool never writes to this field.
//
// Presence in this list means the address is confirmed usable by the
// provider (interface/sub-ENI attached, MAC/VLAN assigned). There is no
// separate "Phase" — membership in IaasReadyIPs IS the ready state, and
// removal (by the provider) IS the only way an address stops being ready.
type IaasReadyIPAllocation struct {
    // +kubebuilder:validation:Optional
    IPv4 *string `json:"ipv4,omitempty"`

    // +kubebuilder:validation:Optional
    IPv6 *string `json:"ipv6,omitempty"`

    // +kubebuilder:validation:Optional
    MAC string `json:"mac,omitempty"`

    // +kubebuilder:validation:Optional
    VLANID *int32 `json:"vlanID,omitempty"`
}

// IaasFailedIPAllocation is one per-IP (or per-IP-pair) prewarm entry that
// the provider attempted and could not bring to Ready. It carries only
// address identity — no MAC/VLAN/error detail — since Spiderpool never acts
// on it beyond excluding it from allocation candidates; diagnostic detail
// belongs on the provider's own status/Conditions or logs, not here. There is
// currently no periodic retry of failed entries by the provider; recovery
// (if any) is out of scope for this iteration.
type IaasFailedIPAllocation struct {
    // +kubebuilder:validation:Optional
    IPv4 *string `json:"ipv4,omitempty"`

    // +kubebuilder:validation:Optional
    IPv6 *string `json:"ipv6,omitempty"`
}
```

**Validation rules (spec-derived, enforced in Go code, not necessarily CRD
OpenAPI schema)**:

- An `IaasReadyIPAllocation`/`IaasFailedIPAllocation` MUST have at least one
  of `IPv4`/`IPv6` populated to be considered usable; entries with neither are
  treated as malformed and skipped (spec Edge Cases).
- Occupancy is NOT stored on `IaasReadyIPAllocation` — it is derived at read
  time by checking whether `IPv4`/`IPv6` already appear as keys in the pool's
  parsed `Status.AllocatedIPs` (existing `PoolIPAllocations` map, via
  `convert.UnmarshalIPPoolAllocatedIPs`). This is the single-writer principle
  from the proposal (§3.3) and the clarification answer on occupancy.
- `IaasFailedIPs` entries are purely exclusionary: an address appearing there
  is simply never selected. In practice this is usually redundant with "not
  present in IaasReadyIPs", since selection only ever looks at the Ready list
  intersected with `spec.ips`-derived candidates; the Failed list exists so
  the provider has a place to record attempted-but-unready addresses without
  needing per-entry status fields, for observability/debugging.

**Selection model (revised — intersection, not replacement)**: Unlike the
original design, a non-empty/empty `IaasReadyIPs` list is **not** what decides
whether ledger-gating applies. Instead:

- Whether a pool is IaaS-managed is decided solely by the `iaas-pool`
  label/annotation (see §1.1). `spec.ips` for an IaaS-managed pool is
  populated exactly like a normal pool — it is NOT left empty and is NOT a
  separate address space from what the provider prewarms; it simply names the
  full range the provider is expected to prewarm from.
- For an IaaS-managed pool, Spiderpool computes the normal candidate set first
  (`spec.ips` − `excludeIPs` − `reservedIPs` − already-allocated IPs, using the
  existing `spiderpoolip.FindAvailableIPs` logic unchanged), then **intersects**
  that candidate set with the addresses present in `status.iaasReadyIPs`. Only
  addresses in the intersection are allocatable.
- If the intersection is empty (e.g. a freshly-created pool where the
  provider hasn't prewarmed anything yet, or all ready addresses are already
  allocated), Spiderpool returns the same `ErrIPUsedOut` error as ordinary
  pool exhaustion — this is a normal, retry-friendly IPAM outcome, not a
  distinct error path.
- For a non-IaaS-managed pool (no `iaas-pool` label), behavior is completely
  unchanged from today: `IaasReadyIPs`/`IaasFailedIPs` are not consulted at
  all, even if somehow populated.
- Because intersection is computed against the already-range/exclusion-scoped
  candidate set, any `IaasReadyIPs` entry whose address falls outside
  `spec.ips`, or duplicates an already-allocated/excluded/reserved address, is
  automatically ignored — no separate "is this ledger entry well-formed
  relative to spec.ips" validation step is needed.
- When the selected address is drawn from the intersection, the matching
  `IaasReadyIPAllocation` entry's `MAC`/`VLANID` are copied onto the resulting
  `IPConfig` (`models.IPConfig.Mac`/`.Vlan`) for the Pod's interface, the same
  way a synchronous provider-allocate response is merged in for non-ledger
  IPs. (This closes a gap in the original design/implementation where
  `MAC`/`VLANID` were recorded on the ledger but never actually read.)

**Single-ledger-on-primary-pool ownership (authoritative)**: for a paired
(dual-stack) `iaas-pool` set, the ledger (`iaasReadyIPs`/`iaasFailedIPs`)
exists ONLY on the **primary pool**, which by convention is always the v4
pool. The v6 sibling pool's `iaasReadyIPs`/`iaasFailedIPs` are never
populated by the provider (the fields exist on every `SpiderIPPool` generically,
but are simply left empty for a non-primary pool). Consequences for
selection:

- **Allocation is driven entirely from the v4 (primary) pool's ledger.**
  `AllocateIP` computes the intersection (as above) using only the v4 pool's
  own `spec.ips`-derived candidate set and its own `iaasReadyIPs`. The
  selected `IaasReadyIPAllocation` entry already carries the paired `ipv6`
  address (and `mac`/`vlanID`) directly — there is no separate step that
  computes a v6-side candidate set or consults the v6 pool's own ledger
  (it has none). This keeps the allocation path's cost identical to a
  single-stack lookup; the only added step is a lightweight sanity check
  (see below) on the sibling pool's own already-in-memory fields.
- **No single-stack consumption of a paired pool.** Allocation from a paired
  `iaas-pool` is always all-or-nothing for the pair — a Pod may not take only
  the v4 or only the v6 side. This guarantees the two pools' own independent
  `status.allocatedIPs` occupancy stays symmetric for every ledger entry
  (v4 address allocated ⇔ v6 address allocated, always to the same Pod),
  which in turn means each pool's own existing
  `spec.ips`-shrink-while-in-use webhook validation is sufficient, on its
  own, to protect an in-use ledger entry from either side being removed —
  no cross-pool occupancy check or "downgrade" fallback is required.
- **Lightweight sibling sanity check.** Before finalizing an allocation,
  Spiderpool confirms the selected entry's `ipv6` address is still present in
  the sibling (v6) pool's current `spec.ips` and not already in the sibling
  pool's `status.allocatedIPs`. This guards against the narrow
  provider-reconcile-lag race where the v6 address was just removed from
  `spec.ips` but the ledger entry hasn't been cleaned up yet. It is not a
  new full candidate-set computation — just the same two basic checks any
  allocation already performs, applied once more to the sibling pool's
  already-fetched fields — so it adds no meaningful overhead.
- **Provider owns full validity of prewarmed pairs.** Because Spiderpool's
  allocation path only checks candidacy against the v4 pool's own
  `spec.ips`/`excludeIPs`, the provider MUST ensure, at prewarm time, that
  any address pair written into `iaasReadyIPs` is simultaneously valid on
  BOTH pools' `spec.ips` (respecting each side's own `excludeIPs`/reserved
  ranges) — this is a prewarm-time responsibility, not an allocation-time
  one. See `docs/develop/proposal-iaas-ip-provider.md` for the corresponding
  provider-side design.

**Ordering / selection**: `IaasReadyIPs` is treated as an unordered set;
Spiderpool relies on the ascending-address order already produced when
computing the normal candidate set (`spiderpoolip.FindAvailableIPs`) and picks
the first candidate that is also present in `IaasReadyIPs`, per clarification
Q5 — no new ordering/priority field is introduced.

### 1.4 Lifecycle (informational — transitions are driven by the external
provider, not by Spiderpool; documented here only so Spiderpool's consumption
logic has a complete picture)

```text
(absent from both lists) -> appended to IaasReadyIPs (prewarm succeeded)
                          -> appended to IaasFailedIPs (prewarm attempted, failed,
                                                          up to 3 in-attempt retries)
IaasReadyIPs entry -> re-paired in place (one side's spec.ips membership lost,
                       replacement found on that side, mac/vlanID unchanged)
                   -> degraded to single-family (one side lost, no replacement
                       found; no cloud release call)
                   -> removed + cloud-released (BOTH sides lost from their
                       respective spec.ips)
```

There is no per-entry `Phase` field and no periodic retry of failed entries in
this iteration: an address either graduates straight into `IaasReadyIPs` once
prewarmed, or is recorded in `IaasFailedIPs` and left alone (this exclusion is
a deferred scope decision, not permanent — see contract doc). Spiderpool's
only required behavior: always treat "present in `IaasReadyIPs` AND not
already allocated" as the sole "available" condition for IaaS-managed pools;
it does not drive, validate, or race against these transitions — it observes
a snapshot at allocation time, plus the lightweight sibling sanity check
described in §1.3 above. The re-pair/degrade/remove reconcile logic itself is
entirely provider-owned — see
`contracts/spiderippool-iaas-extension.md`'s "Paired-Address Reconcile
Contract" for the full state-transition rules and their triggers.

## 2. Pairing Relationship (SpiderIPPool <-> SpiderIPPool)

A pairing is a bidirectional reference between two `SpiderIPPool` objects of
opposite `spec.ipVersion`, declared via `ipam.spidernet.io/pair-pool` on (at
least) one side, ideally both (validated when both exist).

| Rule | Enforcement point | Behavior |
|---|---|---|
| No self-reference | Validating webhook | Reject if `pair-pool` value == pool's own name |
| No same-IP-version pairing | Validating webhook | Reject if referenced pool exists and `spec.ipVersion` matches this pool's `spec.ipVersion` |
| v4 static capacity <= v6 static capacity | Validating webhook (only when both pools exist) | Compute capacity as `spec.ips` minus `spec.excludeIPs` (existing `AssembleTotalIPs`/`IPsDiffSet` helpers); reject if v4 pool's capacity > v6 pool's capacity |
| Identical `nodeName`/`podAffinity` | Validating webhook (only when both pools exist) | Reject if the two pools' `spec.nodeName` or `spec.podAffinity` differ |
| Reference to not-yet-existing pool | Validating webhook | MUST NOT be rejected — allowed, convergence happens once the second pool is created |

This relationship has no separate Go struct/entity — it is purely a
cross-object annotation reference plus validation rules, per proposal's
zero-new-CRD principle.

**Reconcile of paired addresses on `spec.ips` change** (provider-owned,
event-driven only — see `contracts/spiderippool-iaas-extension.md`
§"Paired-Address Reconcile Contract" for the full rules): the ledger lives
only on the primary (v4) pool. When a `spec.ips` shrink removes an address
that is part of an `iaasReadyIPs` entry — whether the edit happened on the v4
or the v6 pool — the provider reconciles that entry: it prefers to
**re-pair** the invalidated side with a fresh replacement address (keeping
the same underlying sub-ENI), falling back to a degraded single-family record
only if no replacement exists, and only fully deletes the entry (with a
cloud `ReleaseIP` call) once BOTH sides are absent from their respective
`spec.ips`. This is driven purely by watch events (never a periodic scan) and
is naturally bounded to at most two reconcile passes because every reconcile
recomputes ledger validity fresh rather than diffing against remembered
state. Because paired-pool allocation is always all-or-nothing (no
single-stack consumption — see §1.3), an entry undergoing this reconciliation
is guaranteed not to be in-use by a Pod, since each pool's own
`spec.ips`-shrink-while-in-use webhook already independently blocks removing
an address that's part of an active allocation, on either side. Spiderpool
itself performs no cross-pool writes for this — it remains a read-only
ledger consumer.

## 3. Pod IP Pool Selection (extended, no new entity)

The existing Pod-facing selection flow
(`ipam.spidernet.io/ippool`/`ippools` annotation ->
`getPoolCandidates`/`ParseWildcardPoolNameList` -> `selectByPod`) is extended
by one rule with no new data structures:

> If a resolved candidate pool carries `pair-pool` and the opposite IP family
> candidate list does not already contain that paired pool, append it.

This is a pure list-transformation step inserted into the existing candidate
resolution pipeline (`pkg/ipam/pool_selections.go`), not a new entity.

## 4. Relationships Summary

```text
SpiderIPPool (IaaS pool, e.g. node1-app-a-v4)
   │  annotation: iaas-pool=true          (this pool is IaaS-managed)
   │  annotation: pair-pool=node1-app-a-v6 (points to sibling)  ── optional
   │  label: iaas-pool=true                (synced from annotation)
   │  status.iaasReadyIPs[]                 (prewarmed pair ledger — PRIMARY POOL ONLY, provider-owned)
   │  status.iaasFailedIPs[]                (attempted-but-unready addresses — PRIMARY POOL ONLY, provider-owned)
   │  status.conditions[]                   (IaasReady summary, provider-owned)
   │
   └── pairs with ──> SpiderIPPool (node1-app-a-v6, the sibling — NO ledger of its own)
                          same shape, opposite spec.ipVersion,
                          pair-pool=node1-app-a-v4 (back-reference)

Pod
   │ annotation: ipam.spidernet.io/ippool: {"ipv4": ["node1-app-a-v4"]}
   │
   └── IPAM resolves candidates ──> [node1-app-a-v4] ──(auto-complete)──> [node1-app-a-v4, node1-app-a-v6]
              └── selectByPod (nodeName/podAffinity, unchanged) ──> final pool(s)
                     └── AllocateIP driven from v4 (primary) pool's ledger only:
                            normal candidate(v4) ∩ v4pool.iaasReadyIPs
                              ──> selected entry carries ipv6/mac/vlanID directly
                              ──> lightweight sibling sanity check (ipv6 still in
                                  v6pool.spec.ips, not already allocated there)
                              ──> write status.allocatedIPs on BOTH pools atomically
                         (non-iaas-pool pools: existing behavior unchanged)
```
