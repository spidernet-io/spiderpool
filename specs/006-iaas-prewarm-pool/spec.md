# Feature Specification: IaaS Provider Prewarm IP Pool Support

**Feature Branch**: `006-iaas-prewarm-pool`

**Created**: 2026-08-05

**Status**: Draft

**Input**: User description: "结合 docs/develop/proposal-iaas-ip-provider.md 和需求，实现 spiderpool 在 provider 模式支持预热 IP 池，并且 ipam 有一些调用修改"

## Clarifications

### Session 2026-08-12 (metadata serialization and spec/status consistency)

- Q: Should `status.ipMetaData.metadata` remain a structured map? → A: No. It is stored as a JSON string whose decoded logical shape remains `map[primaryIP]IPMetadataEntry`. This avoids repeatedly deep-copying and structurally serializing a large CRD map. Spiderpool-agent parses the string when the authoritative metadata revision changes and keeps an immutable in-process map snapshot for allocation-time lookup; it MUST NOT unmarshal the full JSON string for every Pod allocation.
- Q: How does Spiderpool know that provider-written metadata corresponds to the current pool spec after an administrator changes `spec.ips` or another spec field? → A: Add provider-owned `status.ipMetaData.observedGeneration`. The provider advances it to the pool's current `metadata.generation` only when it atomically publishes the completed metadata/counters for that generation. IPAM MUST reject allocation whenever `observedGeneration != metadata.generation`. No `phase` field is required: generation mismatch is the authoritative updating/not-converged state.
- Q: Is the agent cache authoritative? → A: No. `SpiderIPPool.status` remains authoritative and survives restarts. The agent cache is a derived performance optimization keyed by pool UID and observed generation (and optionally metadata content hash). A cache miss/version mismatch fails closed; informer replay reconstructs it after restart.

### Session 2026-08-11 (design revision, aligned with proposal Draft v5)

- Q: Should the status ledger be named after the IaaS scenario (`iaasReadyIPs`/`iaasFailedIPs`)? → A: No. The status field is renamed to a cloud-neutral `status.ipMetaData` structure: a pool-level `parentNic`, a `metadata` map keyed by IP address (IPv4 for v4/primary pools; IPv6 only for a pure-v6 single-stack pool) whose values carry `ipv6`/`mac`/`vlan`, plus two provider-written counters `readyIPCount` (IPs with a metadata entry = prewarmed) and `unreadyIPCount` (spec.ips IPs without one = unready/failed). The failed-IP detail list is removed entirely — failure is expressed as absence from `metadata`.
- Q: Is a `status.conditions` field (e.g. `IaasReady`) needed? → A: No. Upstream `SpiderIPPool` has no conditions field and this feature does not add one. Allocation gating only checks whether an IP is present as a `metadata` key; prewarm health is observable via the two counters.
- Q: How is a pool marked as IaaS-managed? → A: Via `ipam.spidernet.io/iaas-provider: "<vendor>"` (annotation, mirrored to a label by the mutating webhook), replacing the boolean `ipam.spidernet.io/iaas-pool: "true"`. The validating webhook enforces a supported-vendor whitelist, currently exactly `huaweicloud`.

### Session 2026-08-05

- Q: FR-004 required rejecting a single-stack Pod's allocation from a paired pool. Where should this rule live, and is "reject" the right behavior? → A: Do not reject. A single-stack Pod requesting a pool that is paired should simply receive the single address family it asked for (from the ledger entry's matching address), with no error. Single-stack detection reuses the existing cluster `EnableIPv4`/`EnableIPv6` config plus the Pod's actual per-family candidate pool list (same mechanism already used in `pkg/ipam/pool_selections.go`); no new admission webhook is introduced for this check.
- Q: FR-003 required an "equal usable IP count" validation between paired pools — what counts as "usable", and must the counts be exactly equal? → A: Not an equality check. The static address capacity (from `spec.ips` minus `excludeIPs`, independent of current runtime allocation state) of the v4 pool MUST be less than or equal to that of the v6 pool (v6 pool may have equal or more capacity than its v4 pair, since a v6-only pairing surplus is harmless while a v4 surplus would leave v4 addresses that can never be paired).
- Q: Should the pool naming convention `node<X>-<app>-<v4|v6>` mentioned in the source proposal be enforced by validation? → A: No. It remains a documented operator convention only; it is not machine-enforced. Existing wildcard matching plus `nodeName`/`podAffinity` filtering already prevent cross-application/cross-node mismatches, so a naming-format check is unnecessary and would reduce operator flexibility.
- Q: How does IPAM determine whether a per-IP ledger entry is already claimed (and thus unavailable) versus free? → A: Reuse the pool's existing `status.allocatedIPs` allocation bookkeeping as the single source of truth for occupancy; no new "claimed" marker field is added to the ledger entry itself. A ledger entry is available for selection only if it is `ready` (per FR-009) AND its address(es) do not already appear in `status.allocatedIPs`.
- Q: When multiple ledger entries are simultaneously "ready" and unclaimed in a pool, what order should IPAM try them in? → A: Reuse Spiderpool's existing pool IP candidate ordering convention (the same sequential/ascending-address selection order already used for non-IaaS pools). No new selection strategy (e.g., round-robin, random) is introduced for this feature.

## User Scenarios & Testing *(mandatory)*

### User Story 1 - Operator declares a node-pinned, paired prewarm pool (Priority: P1)

An operator (or the IaaS provider controller) manages `SpiderIPPool` resources that represent a "node × application" prewarm pool for underlay Pods whose IPs are backed by IaaS sub-ENIs. The operator marks a pool as an "IaaS pool" via annotation, and optionally links it to a dual-stack sibling pool. Spiderpool must recognize these pools, keep a synchronized label for efficient watching, and preserve their existing `nodeName`/`podAffinity` node-and-app pinning behavior unchanged.

**Why this priority**: Without a way to declare and correctly recognize IaaS-backed prewarm pools (and their dual-stack pairing), no other part of the design (readiness gating, IPAM allocation changes) has anything to operate on. This is the foundational data contract.

**Independent Test**: Create a `SpiderIPPool` with the `ipam.spidernet.io/iaas-provider: "huaweicloud"` annotation (with and without a `pair-pool` annotation). Verify the pool is admitted, the corresponding label is synchronized automatically, and pairing annotation format/consistency is validated without needing any IaaS provider component present.

**Acceptance Scenarios**:

1. **Given** a new `SpiderIPPool` is created with annotation `ipam.spidernet.io/iaas-provider: "huaweicloud"`, **When** the pool is admitted, **Then** Spiderpool automatically sets a matching label `ipam.spidernet.io/iaas-provider: "huaweicloud"` on the pool so external watchers can filter by label selector.
2. **Given** an existing IaaS pool's annotation is later removed or changed, **When** the pool is updated, **Then** the synchronized label is updated to match the annotation (annotation is authoritative).
3. **Given** a pool has annotation `ipam.spidernet.io/pair-pool: <name>` pointing to a pool that does not yet exist, **When** the pool is created, **Then** creation is NOT blocked (the referenced pool may be created later).
4. **Given** two pools reference each other via `pair-pool` and both already exist, **When** either pool is created or updated, **Then** Spiderpool validates the v4 pool's static address capacity (`spec.ips` minus `excludeIPs`) is less than or equal to the v6 pool's static address capacity, rejecting the change if the v4 side has more capacity than the v6 side.
5. **Given** a pool has a `pair-pool` annotation, **When** a Pod that requests only a single IP family (e.g., IPv4-only, as determined by the cluster's `EnableIPv4`/`EnableIPv6` configuration and the Pod's actual per-family candidate pool list) is scheduled to use that pool, **Then** the Pod is allocated only the address of the requested family from a matched metadata entry — no error is raised, and the entry's other-family address is left available for a future dual-stack claim.
6. **Given** two paired pools, **When** either pool specifies `nodeName` or `podAffinity` selectors, **Then** Spiderpool validates both paired pools carry the same `nodeName` and `podAffinity` values, rejecting a mismatch.

---

### User Story 2 - Single-annotation dual-stack allocation from paired pools (Priority: P1)

A Pod's IPAM annotation specifies only the v4 primary pool of a paired IaaS pool set. Spiderpool allocates both address families together from that pool's metadata (the metadata entry carries the paired v6 address), so operators/users do not need to hand-maintain both pool names in every Pod template. The sibling v6 pool never serves allocations on its own: it exists to carry the v6 `spec.ips` definition and to record the v6 side of each allocation in its `status.allocatedIPs`.

**Why this priority**: This is the primary IPAM behavior change requested ("ipam 有一些调用修改") and unlocks dual-stack allocation without extra user-facing configuration. It must land alongside Story 1's pairing/validation semantics to be safe.

**Independent Test**: Configure a Pod (or its owning workload) to reference only a v4 pool that has a valid `pair-pool` annotation pointing to a v6 pool. Run IPAM allocation with dual-stack enabled and confirm the Pod receives both a v4 and a v6 address from the same metadata entry, that both pools' `status.allocatedIPs` record their respective side, and that pool resolution is unchanged for pools without pairing annotations.

**Acceptance Scenarios**:

1. **Given** a Pod's IP pool annotation lists only a v4 pool with a valid `pair-pool` reference to a v6 pool and dual-stack is enabled, **When** Spiderpool allocates for the Pod, **Then** the Pod receives both the v4 and v6 addresses of one metadata entry, and the v6 address is recorded in the sibling pool's `status.allocatedIPs`.
2. **Given** the sibling v6 pool of a paired IaaS pool set appears in a Pod's v6 candidate pools (explicitly or via defaults), **When** pool candidates are filtered, **Then** that sibling pool is excluded from standalone v6 candidacy (its addresses are only ever allocated through the v4 primary pool), while other v6 candidates still work as fallbacks.
3. **Given** a pool has no `pair-pool` annotation, **When** pool resolution runs, **Then** behavior is completely unchanged from current Spiderpool behavior.
4. **Given** a Pod's pool annotation uses a wildcard pattern that expands to a paired v4 primary pool, **When** allocation runs, **Then** pair allocation applies to each matched pool consistently.

---

### User Story 3 - Per-IP readiness gating and atomic IP-pair allocation (Priority: P1)

An external IaaS provider component prewarms individual IPs within an IaaS pool asynchronously and records per-IP metadata (paired v6 address, MAC, VLAN, pool-level parent NIC) in the pool's `status.ipMetaData`; `metadata` is a JSON string encoding the per-IP map and presence of an entry in its decoded map IS the readiness state. The provider also publishes `observedGeneration` so Spiderpool can prove that the metadata corresponds to the current spec. When Spiderpool's IPAM allocates an IP from such a pool, it must only proceed when `observedGeneration == metadata.generation`, then pick only entries that are ready and unclaimed; for paired pools it must allocate the whole matched IP pair atomically.

**Why this priority**: This is the core allocation-path change that makes prewarming useful — the point of prewarming is to expose partial readiness (some IPs ready, some not) without making the entire pool unusable, and to guarantee that a Pod's v4/v6 addresses always come from the same underlying network attachment for paired pools.

**Independent Test**: Populate an IaaS pool's `status.ipMetaData.metadata` covering only a subset of `spec.ips` (including paired entries carrying a sibling pool's v6 address). Trigger IP allocation for a Pod and verify: (a) only addresses with a metadata entry, and unclaimed, are ever chosen; (b) for a paired pool, the v4 and v6 addresses returned always come from the same metadata entry; (c) pools without the IaaS annotation continue to allocate via the existing mechanism unaffected.

**Acceptance Scenarios**:

1. **Given** an IaaS pool where only some `spec.ips` addresses have `ipMetaData.metadata` entries, **When** IPAM allocates an IP from that pool, **Then** only an address with a metadata entry (and not already allocated) is selected; addresses without metadata are never returned to a Pod.
2. **Given** the provider removes an address's metadata entry (e.g., pending release), **When** IPAM allocates an IP from that pool, **Then** that address is skipped (treated as unavailable) even though it is still listed in `spec.ips`.
3. **Given** a paired IaaS pool where a metadata entry's key is a ready v4 address and its value carries the matched v6 address, **When** a dual-stack Pod requests an IP from the pool, **Then** Spiderpool allocates both addresses from the SAME metadata entry together, never mixing addresses from two different entries.
4. **Given** a pool has no `ipMetaData` populated and no IaaS annotation, **When** IPAM allocates an IP, **Then** existing allocation behavior is used unchanged.
5. **Given** all metadata-backed addresses in an IaaS pool are already claimed (or the metadata map is empty), **When** IPAM attempts allocation from that pool, **Then** allocation from that pool fails with a clear "IP pool exhausted / no ready IP available" error, consistent with existing out-of-IP error handling, allowing normal candidate-pool fallback/retry behavior to proceed.
6. **Given** the cluster has the pre-existing synchronous IaaS-provider allocation call enabled (cluster-wide IaaS client integration), **When** a Pod's IP is obtained from a metadata entry, **Then** that pre-existing synchronous call is skipped for this allocation (its cloud-side binding already exists from prewarming); **When** a Pod's IP is obtained via the pre-existing non-metadata allocation mechanism, **Then** the synchronous call behaves exactly as it does today.
7. **Given** an administrator updates an IaaS pool's spec and its `metadata.generation` advances before the provider has completed reconciliation, **When** IPAM receives an allocation request, **Then** allocation from that pool is rejected with a retryable "metadata not reconciled" error until the provider atomically publishes metadata and `observedGeneration` matching the new generation.
8. **Given** an agent receives a pure status Update that publishes matching `observedGeneration` and new metadata, **When** its pool informer processes the event, **Then** it parses the JSON string once, atomically replaces the immutable metadata cache snapshot, and subsequent Pod retries use the new snapshot.

---

### Edge Cases

- What happens when a pool's `pair-pool` annotation points to itself, or to a pool of the same IP family (e.g., v4 pointing to v4)? → Rejected at validation time as an invalid pairing.
- What happens when a paired pool is deleted while its sibling still exists and has active allocations? → Existing allocations on the surviving pool are unaffected; new allocations that would require the (now-missing) pair are not auto-completed for the deleted side, and any pairing consistency validation involving the deleted pool is skipped for that pool going forward.
- How does the system handle a Pod requesting IPs from an IaaS pool that has zero ready entries (fully cold, prewarming still pending)? → Allocation fails using the existing "no available IP" error path; no new blocking or long-wait behavior is introduced into the Pod creation critical path.
- What happens if per-IP status data is malformed or inconsistent (e.g., a paired entry missing its counterpart address)? → That entry is treated as not usable for atomic pair allocation and is skipped; it does not fail the whole pool.
- What happens when a non-IaaS pool (no relevant annotation/status) is processed by the modified allocation and pool-resolution code paths? → Zero behavior change; this is a hard requirement (FR-006).
- What happens while the provider is reconciling a newly edited pool spec? → `metadata.generation` is greater than `status.ipMetaData.observedGeneration`, so IPAM fails closed and kubelet retries the Pod. Existing metadata is never treated as valid for the new generation.
- What happens when the metadata JSON string is malformed or the agent has no parsed snapshot for the current observed generation? → Allocation fails closed with an explicit retryable/internal-data error; Spiderpool MUST NOT fall back to un-prewarmed allocation or silently use an older snapshot.

## Requirements *(mandatory)*

### Functional Requirements

- **FR-001**: System MUST recognize a `SpiderIPPool` as an "IaaS pool" via a dedicated annotation (`ipam.spidernet.io/iaas-provider`, whose value names a supported vendor — currently `huaweicloud`) and automatically synchronize a matching label of the same key/value onto the pool whenever the annotation is created, updated, or removed, with the annotation always taking precedence over any manually-edited label. Unsupported vendor values MUST be rejected at admission time.
- **FR-002**: System MUST support an optional pairing annotation (`ipam.spidernet.io/pair-pool`) on a `SpiderIPPool` that references another pool intended to be its dual-stack (opposite IP family) counterpart.
- **FR-003**: System MUST validate pairing annotations at admission time: reject self-referential pairing, reject pairing between two pools of the same IP version, and — when the referenced pool already exists — validate that the v4 pool's static address capacity (`spec.ips` minus `excludeIPs`) does not exceed the v6 pool's static address capacity, and that both paired pools have identical `nodeName`/`podAffinity` selectors. A pairing reference to a not-yet-existing pool MUST NOT be rejected.
- **FR-004**: System MUST allow a Pod that requests only a single IP family from a pool carrying a `pair-pool` annotation to be allocated that single family's address (drawn from a matched metadata entry) without error; the request MUST NOT be rejected, and the metadata entry's other-family address remains available for a future dual-stack allocation. Single-stack detection reuses existing cluster `EnableIPv4`/`EnableIPv6` configuration and the Pod's per-family candidate pool resolution, introducing no new admission webhook.
- **FR-005**: System MUST allocate both address families of a paired pool set from the v4 primary pool's metadata in a single pair-or-nothing operation when dual-stack is enabled: the Pod declares only the v4 primary pool, the selected metadata entry supplies both addresses, and the sibling v6 pool's `status.allocatedIPs` records the v6 side. The sibling v6 pool MUST be excluded from standalone v6 candidacy during pool selection, without altering resolution for pools lacking a pairing annotation. If a NIC's resolved candidates combine a paired v4 primary pool with any separately-configured v6 pool, the v6 pool configuration MUST be ignored with a warning (the pair allocation already supplies the IPv6 address; honoring the extra pool would leak a duplicate IPv6).
- **FR-006**: System MUST preserve existing Spiderpool API, CRD, Helm, annotation, and webhook behavior unless an explicit compatibility exception is documented; specifically, pools without the IaaS-pool annotation/status MUST allocate IPs exactly as they do today.
- **FR-007**: System MUST expose new user/operator-facing annotation names, label names, status field names, and validation error messages consistently with existing Spiderpool naming and formatting conventions.
- **FR-008**: System MUST extend `SpiderIPPool` status with a single cloud-neutral metadata structure (`status.ipMetaData`) recording: a pool-level parent NIC name; a `metadata` JSON string whose decoded logical shape is a map keyed by the pool's primary-family address (value carrying the paired IPv6 address, MAC, and VLAN); provider-owned `observedGeneration`; and two observational counters (`readyIPCount` = IPs with a decoded metadata entry, `unreadyIPCount` = spec.ips IPs without one). Presence of an entry in the decoded map IS the per-IP readiness state; there is no phase field, failed-IP list, or conditions field.
- **FR-009**: System MUST, during IP candidate selection for a pool carrying the IaaS-provider annotation, first require `status.ipMetaData.observedGeneration == metadata.generation`, then only consider an address available if it is present as a key in the decoded metadata map AND does not already appear in the pool's existing `status.allocatedIPs` occupancy record. A generation mismatch, malformed JSON, or current-generation cache miss MUST fail closed. No separate/new "claimed" marker is added to the metadata entry; occupancy remains derived solely from existing allocation bookkeeping.
- **FR-010**: System MUST, for a paired pool with metadata entries describing matched v4/v6 pairs, allocate both addresses of a chosen entry together as a single atomic operation, never combining the v4 address from one metadata entry with the v6 address of a different entry.
- **FR-011**: System MUST continue to use the existing (pre-change) allocation mechanism, unaffected by this feature, for any pool that does not carry the IaaS-provider annotation.
- **FR-012**: System MUST NOT introduce any new synchronous external/network call on the Pod creation critical path as part of this feature; all per-IP metadata is expected to already be present on the pool's status when IPAM reads it.
- **FR-013**: System MUST report a clear, existing-style "no available IP" failure when a pool (or its resolved paired pool) has no ready, unclaimed metadata-backed addresses, allowing normal multi-pool candidate fallback to proceed rather than blocking or retrying internally.
- **FR-014**: System MUST NOT require any new CRD kind to implement pairing, readiness gating, or atomic pair allocation; all new fields MUST be additions to the existing `SpiderIPPool` status/annotations/labels.
- **FR-015**: System MUST skip the existing synchronous IaaS-provider allocation call (the pre-existing per-Pod cloud API call made when the cluster's IaaS-client integration is enabled) for any IP address obtained from a metadata entry, since that address's cloud-side sub-ENI/binding was already established during prewarming; the existing synchronous call path remains unchanged for any IP obtained via the pre-existing (non-metadata) allocation mechanism.
- **FR-016**: The provider MUST atomically publish `metadata`, `readyIPCount`, `unreadyIPCount`, and `observedGeneration`, and MUST advance `observedGeneration` to the current pool generation after a complete, trustworthy evaluation of that generation. Partial per-IP failures are a valid completed result: successful IPs appear in `metadata`, failed IPs are absent and counted by `unreadyIPCount`. An aborted reconcile, an inability to form a trustworthy full snapshot, or a failed status write MUST leave `observedGeneration` behind the current generation.
- **FR-017**: Spiderpool-agent MUST parse metadata at most once per distinct authoritative metadata revision and keep an immutable in-process decoded snapshot keyed by pool UID and observed generation. Informer status updates MUST rebuild/replace this snapshot atomically; allocation MUST never use a snapshot from another generation. `resourceVersion` changes caused only by `allocatedIPs` MUST NOT force metadata reparsing when the metadata content and observed generation are unchanged.

### Key Entities

- **SpiderIPPool (extended)**: Existing pool custom resource. New annotation `ipam.spidernet.io/iaas-provider` marks it as IaaS-managed by a named vendor; new annotation `ipam.spidernet.io/pair-pool` names its dual-stack sibling pool; new synchronized label mirrors the IaaS-provider annotation for watch filtering; new `status.ipMetaData` field records per-IP (or per-IP-pair) metadata whose presence expresses readiness.
- **Per-IP Metadata Entry**: A logical key/value record encoded inside the `status.ipMetaData.metadata` JSON string, representing one prewarmed address (or matched v4/v6 pair with MAC/VLAN). It carries no independent occupancy/claim flag.
- **Parsed Metadata Snapshot**: An agent-local immutable decoded map derived from the authoritative JSON string. It is a disposable acceleration layer, not API state, and is valid only for its recorded pool UID and observed generation.
- **Pod IP Pool Selection**: The existing Pod-facing mechanism (annotation-driven, wildcard-expandable) by which a Pod names candidate pools; extended so that a selected paired pool automatically pulls in its sibling for the other IP family.

## Success Criteria *(mandatory)*

### Measurable Outcomes

- **SC-001**: Operators can declare a node-pinned, dual-stack prewarm pool pairing using only existing `SpiderIPPool` fields plus two new annotations — no new CRD or custom resource type is required.
- **SC-002**: A Pod that requests only the v4 primary pool of a valid pairing gets both address families allocated automatically from one metadata entry, with 100% of test scenarios recording the v6 side in the sibling pool's `status.allocatedIPs`, without any change to the Pod's own annotations.
- **SC-003**: When a prewarm pool has partial readiness (some IPs with metadata entries, some without), Pods can still successfully obtain IPs from the ready subset; a pool is never made fully unusable by a minority of unready addresses.
- **SC-004**: For paired pools, 100% of allocated dual-stack IP pairs originate from the same underlying metadata entry (no cross-entry mixing) across test scenarios.
- **SC-005**: Pools without any IaaS-provider annotation show zero measurable behavior change in allocation success rate, latency, or selection outcome compared to pre-feature behavior — no performance-sensitive path outside IaaS pools is affected, and the Pod creation critical path incurs no new external calls.
- **SC-006**: New annotation names, label names, status field names, and validation error messages are documented once and used identically across CRD status, webhook validation messages, and any related examples/docs.
- **SC-007**: After any IaaS pool spec update, 100% of allocation attempts are rejected until provider-published `observedGeneration` matches the new generation; no allocation uses metadata from a prior generation.
- **SC-008**: For a 1000-entry metadata ledger, allocation uses a pre-parsed cache snapshot rather than unmarshalling the full JSON per Pod, and status-only changes unrelated to metadata do not trigger reparsing.

## Assumptions

- This specification covers only the Spiderpool (open-source) side of the design described in the provider proposal; the private IaaS provider controller that performs actual cloud sub-ENI creation/binding and writes the per-IP metadata status is an external dependency and out of scope for this feature's implementation, though its expected status shape is treated as a contract this feature must consume correctly.
- "IaaS pool" pools are node-pinned via existing `nodeName` and app-pinned via existing `podAffinity` fields; no new selector mechanism is introduced.
- Physical NIC reporting by spiderpool-agent (used by the external provider to resolve parent ports) and any CLI drift-detection tooling described in the proposal (`iaasnetctl`) are separate, lower-priority capabilities not required for the P0 scope of this feature; they may be addressed in a follow-up feature if needed.
- Per-IP metadata status fields are written by an external actor (the provider) using server-side apply with its own field manager; this feature only needs to read/consume that status, not write it, except for the existing `status.allocatedIPs`-style allocation bookkeeping Spiderpool already owns.
- Webhook mutating (annotation→label sync) and validating (pairing format/consistency, single-stack-Pod-on-paired-pool rejection) behavior described here follow the same admission webhook mechanism Spiderpool already uses for `SpiderIPPool`.
- Pool naming convention (e.g., `node<X>-<app>-<v4|v6>`) is a documented operator best practice for wildcard matching hygiene, not a system-enforced rule; existing `nodeName`/`podAffinity` filtering already prevents cross-application mismatches.
- TTL-based release, cross-node migration, and the `iaasnetctl` drift-correction CLI (proposal sections 7.2, 7.1, and 8) are explicitly out of scope for this feature (proposal stage P1/P2/P3); only P0 scope (annotations, pairing validation, per-IP metadata consumption, IPAM allocation changes) is covered.
