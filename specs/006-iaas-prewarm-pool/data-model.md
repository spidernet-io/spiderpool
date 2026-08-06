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
| `ipam.spidernet.io/iaas-pool` | string (`"true"`/`"false"`) | No | Marks the pool as IaaS-managed. Absence = pool behaves exactly as today (FR-006). |
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
    IaasIPs []IaasIPAllocation `json:"iaasIPs,omitempty"` // NEW

    // +kubebuilder:validation:Optional
    Conditions []metav1.Condition `json:"conditions,omitempty"` // NEW
}

// IaasIPAllocation is one per-IP (or per-IP-pair, for paired pools) prewarm
// ledger entry. Written by the external IaaS provider controller via
// Server-Side Apply with its own field manager; consumed read-only by
// Spiderpool's IPAM. Spiderpool never writes to this field.
type IaasIPAllocation struct {
    // +kubebuilder:validation:Optional
    IPv4 *string `json:"ipv4,omitempty"`

    // +kubebuilder:validation:Optional
    IPv6 *string `json:"ipv6,omitempty"`

    // +kubebuilder:validation:Optional
    MAC string `json:"mac,omitempty"`

    // +kubebuilder:validation:Optional
    VLANID *int32 `json:"vlanID,omitempty"`

    // +kubebuilder:validation:Enum=Ready;NotReady;Releasing
    // +kubebuilder:validation:Required
    Phase string `json:"phase"`

    // +kubebuilder:validation:Optional
    LastError string `json:"lastError,omitempty"`
}
```

**Validation rules (spec-derived, enforced in Go code, not necessarily CRD
OpenAPI schema)**:

- An `IaasIPAllocation` MUST have at least one of `IPv4`/`IPv6` populated to be
  considered usable; entries with neither are treated as malformed and skipped
  (spec Edge Cases).
- `Phase` values other than `Ready` make the entry unavailable for allocation
  regardless of other fields (spec FR-009).
- Occupancy is NOT stored on `IaasIPAllocation` — it is derived at read time
  by checking whether `IPv4`/`IPv6` already appear as keys in the pool's
  parsed `Status.AllocatedIPs` (existing `PoolIPAllocations` map, via
  `convert.UnmarshalIPPoolAllocatedIPs`). This is the single-writer principle
  from the proposal (§3.3) and the clarification answer on occupancy.

**Ordering / selection**: `IaasIPs` is treated as an unordered set by this
feature; when multiple entries qualify, Spiderpool tries them using the same
ascending-address order convention already applied to non-ledger pools (via
`spiderpoolip.FindAvailableIPs`-equivalent ordering over each entry's primary
address), per clarification Q5 — no new ordering/priority field is introduced.

### 1.4 State Transitions (informational — transitions are driven by the
external provider, not by Spiderpool; documented here only so Spiderpool's
consumption logic has a complete picture)

```text
(absent) -> NotReady -> Ready -> Releasing -> (removed from IaasIPs)
                 ^          |
                 └──────────┘ (retry after failure)
```

Spiderpool's only required behavior across these transitions: always treat
`Ready` + not-in-`AllocatedIPs` as the sole "available" condition (FR-009);
every other phase/state is unavailable. Spiderpool does not drive, validate,
or race against these transitions — it observes a snapshot at allocation time.

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
   │  status.iaasIPs[]                      (per-IP/per-pair ledger, provider-owned)
   │  status.conditions[]                   (IaasReady summary, provider-owned)
   │
   └── pairs with ──> SpiderIPPool (node1-app-a-v6)
                          same shape, opposite spec.ipVersion,
                          pair-pool=node1-app-a-v4 (back-reference)

Pod
   │ annotation: ipam.spidernet.io/ippool: {"ipv4": ["node1-app-a-v4"]}
   │
   └── IPAM resolves candidates ──> [node1-app-a-v4] ──(auto-complete)──> [node1-app-a-v4, node1-app-a-v6]
              └── selectByPod (nodeName/podAffinity, unchanged) ──> final pool(s)
                     └── AllocateIP (per-pool) ──> ledger-aware pick if IaasIPs populated,
                                                     else existing behavior
```
