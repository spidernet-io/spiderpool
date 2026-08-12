# Phase 1 Data Model: IaaS Provider Prewarm IP Pool Support

All entities below are additive extensions of the existing
`SpiderIPPool` CRD (`pkg/k8s/apis/spiderpool.spidernet.io/v2beta1/spiderippool_types.go`).
No new CRD kind is introduced (constitution/proposal constraint). Field names
below are proposed Go/JSON names; final casing/ordering must satisfy
`make manifests generate-k8s-api` output review during implementation.

> **Revision note (v5 alignment)**: this data model was revised to match
> proposal Draft v5. The former `status.iaasReadyIPs`/`status.iaasFailedIPs`
> list ledgers and `status.conditions` are replaced by a single cloud-neutral
> `status.ipMetaData` structure (per-IP metadata map + pool-level parent NIC
> + two provider-written counters), and the `ipam.spidernet.io/iaas-pool`
> marker annotation is replaced by `ipam.spidernet.io/iaas-provider: "<vendor>"`.
>
> **Revision note (v6, 2026-08-12)**: `metadata` is serialized as a JSON
> string rather than a structural CRD map. Its decoded logical type remains
> `map[string]IPMetadataEntry`. `observedGeneration` is added so allocation can
> prove that provider output corresponds to the current spec. Agent processes
> maintain an immutable parsed-map cache; the cache is derived and disposable,
> while status remains authoritative.

## 1. SpiderIPPool (extended)

### 1.1 New Annotations (metadata.annotations, user/operator or provider set)

| Annotation | Type | Required | Notes |
|---|---|---|---|
| `ipam.spidernet.io/iaas-provider` | string (vendor name, e.g. `"huaweicloud"`) | No | Marks the pool as IaaS-managed by the named provider. Addresses require a current-generation entry in the decoded `status.ipMetaData.metadata` JSON before allocation. |
| `ipam.spidernet.io/pair-pool` | string (pool name) | No | Names the dual-stack sibling `SpiderIPPool`. Only meaningful when `iaas-provider` is also set, though validation does not strictly require co-presence beyond the rules in §2. |

### 1.2 New Label (metadata.labels, system-managed)

| Label | Type | Managed by | Notes |
|---|---|---|---|
| `ipam.spidernet.io/iaas-provider` | string (mirrors annotation value, e.g. `huaweicloud`) | Mutating webhook (`ippool_mutate.go`) | Annotation is authoritative; label is corrected to match on every create/update, following the `LabelIPPoolCIDR` precedent. Exists purely so an external watcher (the private provider) can use an efficient label-selector watch (optionally filtered by vendor value) — spiderpool itself does not filter by this label. |

### 1.3 New Status Field (status, subresource)

```go
// IPPoolStatus defines the observed state of SpiderIPPool.
type IPPoolStatus struct {
    AllocatedIPs     *string `json:"allocatedIPs,omitempty"`     // existing, unchanged
    TotalIPCount     *int64  `json:"totalIPCount,omitempty"`     // existing, unchanged
    AllocatedIPCount *int64  `json:"allocatedIPCount,omitempty"` // existing, unchanged

    // +kubebuilder:validation:Optional
    IPMetaData *IPMetaData `json:"ipMetaData,omitempty"` // NEW
}

// IPMetaData carries per-IP link-layer/pairing metadata written by an
// external controller (e.g. an IaaS provider) via Server-Side Apply with its
// own field manager; consumed read-only by Spiderpool's IPAM. Spiderpool
// never writes to this field. The naming is cloud-neutral on purpose: it
// stores generic IP metadata (MAC, VLAN, paired IPv6, parent NIC), and IaaS
// prewarming is merely its first consumer.
//
// Presence of an address as a key in Metadata means the address is confirmed
// usable by the provider (interface/sub-ENI attached, MAC/VLAN assigned).
// There is no separate "Phase" or ready/failed list — membership in the
// Metadata map IS the ready state, and removal (by the provider) IS the only
// way an address stops being ready. A prewarm failure is simply the address's
// ABSENCE from Metadata (counted in UnreadyIPCount); per-IP failure detail
// lives in the provider's own logs, not in this CRD.
type IPMetaData struct {
    // ParentNic is the pool-level parent NIC name on the node this
    // (nodeName-scoped) pool is bound to. All sub-interfaces prewarmed for
    // this pool hang off this NIC, so it is not repeated per IP.
    // +kubebuilder:validation:Optional
    ParentNic string `json:"parentNic,omitempty"`

    // Metadata is a JSON-encoded map[string]IPMetadataEntry. The key is the
    // pool's primary-family address: IPv4 for a v4/primary pool; IPv6 only
    // for a pure IPv6 single-stack pool. It is a string to prevent Kubernetes
    // machinery from structurally deep-copying/validating a large map on
    // every unrelated status.allocatedIPs update.
    // +kubebuilder:validation:Optional
    Metadata *string `json:"metadata,omitempty"`

    // ObservedGeneration is the pool metadata.generation for which the
    // provider completed a trustworthy full evaluation. Partial per-IP
    // failures are valid: successful entries are published in Metadata and
    // failures are reflected by absence plus UnreadyIPCount.
    // +kubebuilder:validation:Minimum=0
    // +kubebuilder:validation:Optional
    ObservedGeneration *int64 `json:"observedGeneration,omitempty"`

    // ReadyIPCount is the number of IPs that have a Metadata entry
    // (= successfully prewarmed). Written by the provider together with
    // Metadata in the same SSA apply; observational only.
    // +kubebuilder:validation:Minimum=0
    // +kubebuilder:validation:Optional
    ReadyIPCount *int64 `json:"readyIPCount,omitempty"`

    // UnreadyIPCount is the number of IPs in spec.ips that have NO Metadata
    // entry (= not yet prewarmed, or prewarm failed). Written by the
    // provider; observational only — it never gates allocation.
    // +kubebuilder:validation:Minimum=0
    // +kubebuilder:validation:Optional
    UnreadyIPCount *int64 `json:"unreadyIPCount,omitempty"`
}

// DecodedIPMetadata is NOT a CRD field. It is the logical payload encoded in
// IPMetaData.Metadata and the agent-local cache value.
type DecodedIPMetadata map[string]IPMetadataEntry

// IPMetadataEntry is the metadata attached to one (possibly paired) IP.
type IPMetadataEntry struct {
    // IPv6 is the paired IPv6 address for a dual-stack pair (present only
    // on the primary/v4 pool's entries). Empty for single-stack entries.
    // +kubebuilder:validation:Optional
    IPv6 *string `json:"ipv6,omitempty"`

    // +kubebuilder:validation:Optional
    MAC string `json:"mac,omitempty"`

    // +kubebuilder:validation:Optional
    VLAN *int32 `json:"vlan,omitempty"`
}
```

**Deliberately absent fields**:

- **No `status.conditions`**: upstream `SpiderIPPool` has no conditions field
  and this feature does not add one. Allocation gating never needs it (an IP
  is allocatable iff it is a `Metadata` key), and prewarm health is directly
  observable from `ReadyIPCount`/`UnreadyIPCount`.
- **No failed-IP detail list**: the former `iaasFailedIPs` is removed.
  Failure = absence from `Metadata`, counted in `UnreadyIPCount` only.
- **No `phase`**: pool convergence is derived without an independent state
  machine. `observedGeneration == metadata.generation` means the provider has
  published a complete result for the current spec; mismatch means updating,
  failed, or otherwise not converged and allocation fails closed.

**Validation rules (spec-derived, enforced in Go code, not necessarily CRD
OpenAPI schema)**:

- `Metadata` MUST decode as a JSON object whose values satisfy
  `IPMetadataEntry`. A decoded key MUST parse as a valid IP address of the pool's own
  `spec.ipVersion`; entries whose key does not parse are treated as malformed
  and skipped rather than failing the whole pool (spec Edge Cases).
- Occupancy is NOT stored on `IPMetadataEntry` — it is derived at read time by
  checking whether the key (and, for pairs, the entry's `IPv6`) already appear
  as keys in the pool's parsed `Status.AllocatedIPs` (existing
  `PoolIPAllocations` map, via `convert.UnmarshalIPPoolAllocatedIPs`). This is
  the single-writer principle from the proposal (§3.3).
- `ReadyIPCount`/`UnreadyIPCount` are provider-written observational counters;
  Spiderpool neither computes nor validates them and never consults them on
  the allocation path.
- The provider MUST publish `Metadata`, both counters, and
  `ObservedGeneration=current metadata.generation` atomically after a complete,
  trustworthy evaluation. Per-IP failures are a valid completed result:
  successful entries are present and failed entries are absent/count unready.
  It MUST NOT advance `ObservedGeneration` if reconciliation aborts or cannot
  form and persist a trustworthy full snapshot.

**Selection model (intersection, not replacement)**:

- Whether a pool is IaaS-managed is decided solely by the `iaas-provider`
  label/annotation (see §1.1). `spec.ips` for an IaaS-managed pool is
  populated exactly like a normal pool — it is NOT left empty and is NOT a
  separate address space from what the provider prewarms; it simply names the
  full range the provider is expected to prewarm from.
- Before candidate computation for an IaaS-managed pool, Spiderpool requires
  `status.ipMetaData.observedGeneration == metadata.generation` and a parsed
  cache snapshot for that same generation. Mismatch, malformed JSON, or cache
  miss fails closed with a retryable metadata-not-ready error.
- Spiderpool then computes the normal candidate set first
  (`spec.ips` − `excludeIPs` − `reservedIPs` − already-allocated IPs, using the
  existing `spiderpoolip.FindAvailableIPs` logic unchanged), then **intersects**
  that candidate set with keys in the cached decoded metadata. Only
  addresses in the intersection are allocatable.
- If the intersection is empty (e.g. a freshly-created pool where the
  provider hasn't prewarmed anything yet, or all ready addresses are already
  allocated), Spiderpool returns the same `ErrIPUsedOut` error as ordinary
  pool exhaustion — this is a normal, retry-friendly IPAM outcome, not a
  distinct error path.
- For a non-IaaS-managed pool (no `iaas-provider` label), behavior is
  completely unchanged from today: `ipMetaData` is not consulted at all, even
  if somehow populated.
- Because intersection is computed against the already-range/exclusion-scoped
  candidate set, any `Metadata` key that falls outside `spec.ips`, or
  duplicates an already-allocated/excluded/reserved address, is automatically
  ignored — no separate "is this metadata entry well-formed relative to
  spec.ips" validation step is needed.
- When the selected address is drawn from the intersection, the matching
  `IPMetadataEntry`'s `MAC`/`VLAN` are copied onto the resulting `IPConfig`
  (`models.IPConfig.Mac`/`.Vlan`) for the Pod's interface, the same way a
  synchronous provider-allocate response is merged in for non-metadata IPs.

**Single-metadata-on-primary-pool ownership (authoritative)**: for a paired
(dual-stack) IaaS pool set, `ipMetaData` exists ONLY on the **primary pool**,
which by convention is always the v4 pool. The v6 sibling pool's `ipMetaData`
is never populated by the provider (the field exists on every `SpiderIPPool`
generically, but is simply left empty for a non-primary pool). Consequences
for selection:

- **Allocation is driven entirely from the v4 (primary) pool's metadata.**
  `AllocateIP` computes the intersection (as above) using only the v4 pool's
  own `spec.ips`-derived candidate set and its own `Metadata` keys. The
  selected entry already carries the paired `ipv6` address (and `mac`/`vlan`)
  directly — there is no separate step that computes a v6-side candidate set
  or consults the v6 pool's own metadata (it has none). This keeps the
  allocation path's cost identical to a single-stack lookup; the only added
  step is a lightweight sanity check (see below) on the sibling pool's own
  already-in-memory fields.
- **No single-stack consumption of a paired pool.** Allocation from a paired
  IaaS pool is always all-or-nothing for the pair — a Pod may not take only
  the v4 or only the v6 side. This guarantees the two pools' own independent
  `status.allocatedIPs` occupancy stays symmetric for every metadata entry
  (v4 address allocated ⇔ v6 address allocated, always to the same Pod),
  which in turn means each pool's own existing
  `spec.ips`-shrink-while-in-use webhook validation is sufficient, on its
  own, to protect an in-use entry from either side being removed — no
  cross-pool occupancy check or "downgrade" fallback is required.
- **Lightweight sibling sanity check.** Before finalizing an allocation,
  Spiderpool confirms the selected entry's `ipv6` address is still present in
  the sibling (v6) pool's current `spec.ips` and not already in the sibling
  pool's `status.allocatedIPs`. This guards against the narrow
  provider-reconcile-lag race where the v6 address was just removed from
  `spec.ips` but the metadata entry hasn't been cleaned up yet. It is not a
  new full candidate-set computation — just the same two basic checks any
  allocation already performs, applied once more to the sibling pool's
  already-fetched fields — so it adds no meaningful overhead.
- **Provider owns full validity of prewarmed pairs.** Because Spiderpool's
  allocation path only checks candidacy against the v4 pool's own
  `spec.ips`/`excludeIPs`, the provider MUST ensure, at prewarm time, that
  any address pair written into `Metadata` is simultaneously valid on BOTH
  pools' `spec.ips` (respecting each side's own `excludeIPs`/reserved
  ranges) — this is a prewarm-time responsibility, not an allocation-time
  one. See the provider-side proposal for the corresponding design.

**Ordering / selection**: `Metadata` is an unordered map; Spiderpool relies
on the ascending-address order already produced when computing the normal
candidate set (`spiderpoolip.FindAvailableIPs`) and picks the first candidate
that is also present as a `Metadata` key, per clarification Q5 — no new
ordering/priority field is introduced.

### 1.4 Lifecycle (informational — transitions are driven by the external
provider, not by Spiderpool; documented here only so Spiderpool's consumption
logic has a complete picture)

```text
(spec generation G, observedGeneration G)  -- spec edit -->
(spec generation G+1, observedGeneration G; allocation blocked)
  -- provider completes and atomically publishes metadata/counters -->
(spec generation G+1, observedGeneration G+1; allocation enabled)

(absent from Metadata) -> key inserted into Metadata (prewarm succeeded)
                       -> stays absent, counted in UnreadyIPCount (prewarm
                          attempted, failed, up to 3 in-attempt retries;
                          detail in provider logs only)
Metadata entry -> re-paired in place (one side's spec.ips membership lost,
                   replacement found on that side, mac/vlan unchanged; a v4-side
                   replacement means the map key itself changes)
               -> degraded to single-family (one side lost, no replacement
                   found; no cloud release call; a lost v4 side re-keys the
                   entry by its IPv6 address)
               -> removed + cloud-released (BOTH sides lost from their
                   respective spec.ips)
```

There is no pool or per-entry `Phase` field and no periodic retry of failed addresses
in this iteration: an address either graduates straight into `Metadata` once
prewarmed, or stays absent and is counted in `UnreadyIPCount` (this exclusion
is a deferred scope decision, not permanent — see contract doc). Spiderpool's
only required behavior: always treat "present as a `Metadata` key AND not
already allocated" as the sole "available" condition for IaaS-managed pools;
it consumes an immutable parsed snapshot only when its observed generation
matches the spec, plus the lightweight sibling sanity check described in §1.3
above. The re-pair/degrade/remove reconcile logic itself is
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
| Supported vendor | Validating webhook | Reject an `iaas-provider` annotation whose value is not in the supported vendor list (currently `huaweicloud`) |

This relationship has no separate Go struct/entity — it is purely a
cross-object annotation reference plus validation rules, per proposal's
zero-new-CRD principle.

**Reconcile of paired addresses on `spec.ips` change** (provider-owned,
event-driven only — see `contracts/spiderippool-iaas-extension.md`
§"Paired-Address Reconcile Contract" for the full rules): the metadata lives
only on the primary (v4) pool. When a `spec.ips` shrink removes an address
that is part of a `Metadata` entry — whether the edit happened on the v4
or the v6 pool — the provider reconciles that entry: it prefers to
**re-pair** the invalidated side with a fresh replacement address (keeping
the same underlying sub-ENI), falling back to a degraded single-family record
only if no replacement exists, and only fully deletes the entry (with a
cloud `ReleaseIP` call) once BOTH sides are absent from their respective
`spec.ips`. This is driven purely by watch events (never a periodic scan) and
is naturally bounded to at most two reconcile passes because every reconcile
recomputes metadata validity fresh rather than diffing against remembered
state. Because paired-pool allocation is always all-or-nothing (no
single-stack consumption — see §1.3), an entry undergoing this reconciliation
is guaranteed not to be in-use by a Pod, since each pool's own
`spec.ips`-shrink-while-in-use webhook already independently blocks removing
an address that's part of an active allocation, on either side. Spiderpool
itself performs no cross-pool writes for this — it remains a read-only
metadata consumer.

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
   │  annotation: iaas-provider=huaweicloud  (this pool is IaaS-managed by that vendor)
   │  annotation: pair-pool=node1-app-a-v6   (points to sibling)  ── optional
   │  label: iaas-provider=huaweicloud       (synced from annotation)
   │  status.ipMetaData                      (PRIMARY POOL ONLY, provider-owned)
   │     ├── parentNic                       (pool-level parent NIC)
   │     ├── metadata: JSON string encoding {ipv4 -> {ipv6,mac,vlan}}
   │     ├── observedGeneration              (must equal metadata.generation)
   │     └── readyIPCount / unreadyIPCount   (observational counters)
   │
   └── pairs with ──> SpiderIPPool (node1-app-a-v6, the sibling — NO metadata of its own)
                          same shape, opposite spec.ipVersion,
                          pair-pool=node1-app-a-v4 (back-reference)

Pod
   │ annotation: ipam.spidernet.io/ippool: {"ipv4": ["node1-app-a-v4"]}
   │
   └── IPAM resolves candidates ──> [node1-app-a-v4] ──(auto-complete)──> [node1-app-a-v4, node1-app-a-v6]
              └── selectByPod (nodeName/podAffinity, unchanged) ──> final pool(s)
                     └── AllocateIP driven from v4 (primary) pool's metadata only:
                            require observedGeneration == pool.generation
                              ──> immutable decoded-cache snapshot
                              ──> normal candidate(v4) ∩ decoded metadata keys
                              ──> selected entry carries ipv6/mac/vlan directly
                              ──> lightweight sibling sanity check (ipv6 still in
                                  v6pool.spec.ips, not already allocated there)
                              ──> write status.allocatedIPs on BOTH pools atomically
                         (non-IaaS pools: existing behavior unchanged)
```
