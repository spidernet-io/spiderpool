# Feature Specification: IaaS Provider Prewarm IP Pool Support

**Feature Branch**: `006-iaas-prewarm-pool`

**Created**: 2026-08-05

**Status**: Draft

**Input**: User description: "结合 docs/develop/proposal-iaas-ip-provider.md 和需求，实现 spiderpool 在 provider 模式支持预热 IP 池，并且 ipam 有一些调用修改"

## Clarifications

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

**Independent Test**: Create a `SpiderIPPool` with the `ipam.spidernet.io/iaas-pool: "true"` annotation (with and without a `pair-pool` annotation). Verify the pool is admitted, the corresponding label is synchronized automatically, and pairing annotation format/consistency is validated without needing any IaaS provider component present.

**Acceptance Scenarios**:

1. **Given** a new `SpiderIPPool` is created with annotation `ipam.spidernet.io/iaas-pool: "true"`, **When** the pool is admitted, **Then** Spiderpool automatically sets a matching label `ipam.spidernet.io/iaas-pool: "true"` on the pool so external watchers can filter by label selector.
2. **Given** an existing IaaS pool's annotation is later removed or changed, **When** the pool is updated, **Then** the synchronized label is updated to match the annotation (annotation is authoritative).
3. **Given** a pool has annotation `ipam.spidernet.io/pair-pool: <name>` pointing to a pool that does not yet exist, **When** the pool is created, **Then** creation is NOT blocked (the referenced pool may be created later).
4. **Given** two pools reference each other via `pair-pool` and both already exist, **When** either pool is created or updated, **Then** Spiderpool validates the v4 pool's static address capacity (`spec.ips` minus `excludeIPs`) is less than or equal to the v6 pool's static address capacity, rejecting the change if the v4 side has more capacity than the v6 side.
5. **Given** a pool has a `pair-pool` annotation, **When** a Pod that requests only a single IP family (e.g., IPv4-only, as determined by the cluster's `EnableIPv4`/`EnableIPv6` configuration and the Pod's actual per-family candidate pool list) is scheduled to use that pool, **Then** the Pod is allocated only the address of the requested family from a matched ledger entry — no error is raised, and the entry's other-family address is left available for a future dual-stack claim.
6. **Given** two paired pools, **When** either pool specifies `nodeName` or `podAffinity` selectors, **Then** Spiderpool validates both paired pools carry the same `nodeName` and `podAffinity` values, rejecting a mismatch.

---

### User Story 2 - Automatic dual-stack pool completion for Pod IP requests (Priority: P1)

A Pod's IPAM annotation specifies only a single-family pool (commonly IPv4) that happens to be paired with a sibling pool for the other family. When Spiderpool resolves the Pod's requested pools, it must automatically discover and include the paired pool for the missing address family, so operators/users do not need to hand-maintain both pool names in every Pod template.

**Why this priority**: This is the primary IPAM behavior change requested ("ipam 有一些调用修改") and unlocks dual-stack allocation without extra user-facing configuration. It must land alongside Story 1's pairing/validation semantics to be safe.

**Independent Test**: Configure a Pod (or its owning workload) to reference only a v4 pool that has a valid `pair-pool` annotation pointing to a v6 pool. Run IPAM pool resolution and confirm the resolved pool list includes both the v4 and paired v6 pool, while leaving pool resolution unchanged for pools without pairing annotations.

**Acceptance Scenarios**:

1. **Given** a Pod's IP pool annotation lists only a v4 pool with a valid `pair-pool` reference to a v6 pool, and no v6 pool is explicitly requested, **When** Spiderpool resolves the Pod's candidate pools, **Then** the paired v6 pool is automatically appended to the resolution result.
2. **Given** a Pod's IP pool annotation already explicitly specifies both the v4 pool and its paired v6 pool, **When** pool resolution runs, **Then** no duplicate entries are introduced and the explicit request is honored as-is.
3. **Given** a pool has no `pair-pool` annotation, **When** pool resolution runs, **Then** behavior is completely unchanged from current Spiderpool behavior (no auto-completion attempted).
4. **Given** a Pod's pool annotation uses a wildcard pattern that expands to a pool carrying a `pair-pool` annotation, **When** pool resolution runs, **Then** auto-completion still applies to each matched pool consistently.

---

### User Story 3 - Per-IP readiness gating and atomic IP-pair allocation (Priority: P1)

An external IaaS provider component prewarms individual IPs within an IaaS pool asynchronously and records per-IP readiness state (ready / not-ready / releasing) plus pairing details (e.g., matched v4/v6 address, MAC, VLAN) in the pool's status. When Spiderpool's IPAM allocates an IP from such a pool, it must only pick IPs that are marked ready and unclaimed, and — for paired pools — must allocate the whole matched IP pair atomically as one indivisible unit rather than selecting the v4 and v6 addresses independently.

**Why this priority**: This is the core allocation-path change that makes prewarming useful — the point of prewarming is to expose partial readiness (some IPs ready, some not) without making the entire pool unusable, and to guarantee that a Pod's v4/v6 addresses always come from the same underlying network attachment for paired pools.

**Independent Test**: Populate an IaaS pool's status with a mix of ready and not-ready per-IP entries (including paired entries referencing a sibling pool). Trigger IP allocation for a Pod and verify: (a) only ready, unclaimed entries are ever chosen; (b) for a paired pool, the v4 and v6 addresses returned always come from the same ledger entry; (c) pools without this per-IP status data continue to allocate via the existing mechanism unaffected.

**Acceptance Scenarios**:

1. **Given** an IaaS pool has per-IP status entries where some are ready and some are not-ready, **When** IPAM allocates an IP from that pool, **Then** only a ready, unclaimed entry is selected; not-ready entries are never returned to a Pod.
2. **Given** an IaaS pool has a per-IP entry marked "releasing", **When** IPAM allocates an IP from that pool, **Then** that entry is skipped (treated as unavailable) even though it is not yet deleted.
3. **Given** a paired IaaS pool where a ledger entry contains both a ready v4 and its matched v6 address, **When** a dual-stack Pod requests an IP from the pool, **Then** Spiderpool allocates both addresses from the SAME ledger entry together, never mixing addresses from two different entries.
4. **Given** a pool has no per-IP readiness status populated (i.e., not an IaaS-managed pool, or status not yet written), **When** IPAM allocates an IP, **Then** existing allocation behavior is used unchanged.
5. **Given** all per-IP entries in an IaaS pool are either not-ready, releasing, or already claimed, **When** IPAM attempts allocation from that pool, **Then** allocation from that pool fails with a clear "IP pool exhausted / no ready IP available" error, consistent with existing out-of-IP error handling, allowing normal candidate-pool fallback/retry behavior to proceed.
6. **Given** the cluster has the pre-existing synchronous IaaS-provider allocation call enabled (cluster-wide IaaS client integration), **When** a Pod's IP is obtained from a `ready` per-IP ledger entry, **Then** that pre-existing synchronous call is skipped for this allocation (its cloud-side binding already exists from prewarming); **When** a Pod's IP is obtained via the pre-existing non-ledger allocation mechanism, **Then** the synchronous call behaves exactly as it does today.

---

### Edge Cases

- What happens when a pool's `pair-pool` annotation points to itself, or to a pool of the same IP family (e.g., v4 pointing to v4)? → Rejected at validation time as an invalid pairing.
- What happens when a paired pool is deleted while its sibling still exists and has active allocations? → Existing allocations on the surviving pool are unaffected; new allocations that would require the (now-missing) pair are not auto-completed for the deleted side, and any pairing consistency validation involving the deleted pool is skipped for that pool going forward.
- How does the system handle a Pod requesting IPs from an IaaS pool that has zero ready entries (fully cold, prewarming still pending)? → Allocation fails using the existing "no available IP" error path; no new blocking or long-wait behavior is introduced into the Pod creation critical path.
- What happens if per-IP status data is malformed or inconsistent (e.g., a paired entry missing its counterpart address)? → That entry is treated as not usable for atomic pair allocation and is skipped; it does not fail the whole pool.
- What happens when a non-IaaS pool (no relevant annotation/status) is processed by the modified allocation and pool-resolution code paths? → Zero behavior change; this is a hard requirement (FR-006).

## Requirements *(mandatory)*

### Functional Requirements

- **FR-001**: System MUST recognize a `SpiderIPPool` as an "IaaS pool" via a dedicated annotation (`ipam.spidernet.io/iaas-pool`) and automatically synchronize a matching label of the same key/value onto the pool whenever the annotation is created, updated, or removed, with the annotation always taking precedence over any manually-edited label.
- **FR-002**: System MUST support an optional pairing annotation (`ipam.spidernet.io/pair-pool`) on a `SpiderIPPool` that references another pool intended to be its dual-stack (opposite IP family) counterpart.
- **FR-003**: System MUST validate pairing annotations at admission time: reject self-referential pairing, reject pairing between two pools of the same IP version, and — when the referenced pool already exists — validate that the v4 pool's static address capacity (`spec.ips` minus `excludeIPs`) does not exceed the v6 pool's static address capacity, and that both paired pools have identical `nodeName`/`podAffinity` selectors. A pairing reference to a not-yet-existing pool MUST NOT be rejected.
- **FR-004**: System MUST allow a Pod that requests only a single IP family from a pool carrying a `pair-pool` annotation to be allocated that single family's address (drawn from a matched ledger entry) without error; the request MUST NOT be rejected, and the ledger entry's other-family address remains available for a future dual-stack allocation. Single-stack detection reuses existing cluster `EnableIPv4`/`EnableIPv6` configuration and the Pod's per-family candidate pool resolution, introducing no new admission webhook.
- **FR-005**: System MUST, during Pod IP pool resolution, automatically append the paired pool of the opposite IP family when a Pod's pool selection includes a paired pool but does not already explicitly include its pair, without introducing duplicate pool entries and without altering resolution for pools lacking a pairing annotation.
- **FR-006**: System MUST preserve existing Spiderpool API, CRD, Helm, annotation, and webhook behavior unless an explicit compatibility exception is documented; specifically, pools without the IaaS-pool annotation/status MUST allocate IPs exactly as they do today.
- **FR-007**: System MUST expose new user/operator-facing annotation names, label names, status field names, and validation error messages consistently with existing Spiderpool naming and formatting conventions.
- **FR-008**: System MUST extend `SpiderIPPool` status with a per-IP ledger recording, for each prewarmed entry, at minimum: the IP address(es) covered (single address, or a matched v4/v6 pair for paired pools), a readiness phase (ready / not-ready / releasing), and an optional error/reason message for non-ready entries.
- **FR-009**: System MUST, during IP candidate selection for a pool that has the per-IP ledger populated, only consider an entry available if it is in the "ready" phase AND its address(es) do not already appear in the pool's existing `status.allocatedIPs` occupancy record; entries that are "not-ready", "releasing", or already recorded as allocated MUST be treated as unavailable. No separate/new "claimed" marker is added to the ledger entry — occupancy is derived solely from existing allocation bookkeeping. When multiple entries are simultaneously available, System MUST try them in the same order as Spiderpool's existing (non-IaaS) pool IP candidate ordering convention, without introducing a new selection strategy.
- **FR-010**: System MUST, for a paired pool with ledger entries describing matched v4/v6 pairs, allocate both addresses of a chosen entry together as a single atomic operation, never combining the v4 address from one ledger entry with the v6 address of a different entry.
- **FR-011**: System MUST continue to use the existing (pre-change) allocation mechanism, unaffected by this feature, for any pool that does not have the per-IP ledger status populated.
- **FR-012**: System MUST NOT introduce any new synchronous external/network call on the Pod creation critical path as part of this feature; all per-IP ledger data is expected to already be present on the pool's status when IPAM reads it.
- **FR-013**: System MUST report a clear, existing-style "no available IP" failure when a pool (or its resolved paired pool) has no ready, unclaimed ledger entries, allowing normal multi-pool candidate fallback to proceed rather than blocking or retrying internally.
- **FR-014**: System MUST NOT require any new CRD kind to implement pairing, readiness gating, or atomic pair allocation; all new fields MUST be additions to the existing `SpiderIPPool` status/annotations/labels.
- **FR-015**: System MUST skip the existing synchronous IaaS-provider allocation call (the pre-existing per-Pod cloud API call made when the cluster's IaaS-client integration is enabled) for any IP address obtained from a `ready` per-IP ledger entry, since that address's cloud-side sub-ENI/binding was already established during prewarming; the existing synchronous call path remains unchanged for any IP obtained via the pre-existing (non-ledger) allocation mechanism.

### Key Entities

- **SpiderIPPool (extended)**: Existing pool custom resource. New annotation `ipam.spidernet.io/iaas-pool` marks it as IaaS-managed; new annotation `ipam.spidernet.io/pair-pool` names its dual-stack sibling pool; new synchronized label mirrors the IaaS-pool annotation for watch filtering; new status ledger field records per-IP (or per-IP-pair) readiness state.
- **Per-IP Ledger Entry**: A record within a pool's status representing one prewarmed address (or, for paired pools, one matched v4/v6 address pair) along with its readiness phase and optional error detail. This is the atomic unit of allocation for IaaS pools. It carries no independent occupancy/claim flag — whether an entry is currently claimed is derived by cross-referencing its address(es) against the pool's existing `status.allocatedIPs` record.
- **Pod IP Pool Selection**: The existing Pod-facing mechanism (annotation-driven, wildcard-expandable) by which a Pod names candidate pools; extended so that a selected paired pool automatically pulls in its sibling for the other IP family.

## Success Criteria *(mandatory)*

### Measurable Outcomes

- **SC-001**: Operators can declare a node-pinned, dual-stack prewarm pool pairing using only existing `SpiderIPPool` fields plus two new annotations — no new CRD or custom resource type is required.
- **SC-002**: A Pod that requests a single-family pool with a valid pairing gets both address families allocated automatically, with 100% of the resolved pool set containing the correct paired pool in test scenarios, without any change to the Pod's own annotations.
- **SC-003**: When a prewarm pool has partial readiness (some IPs ready, some not), Pods can still successfully obtain IPs from the ready subset; a pool is never made fully unusable by a minority of not-ready entries.
- **SC-004**: For paired pools, 100% of allocated dual-stack IP pairs originate from the same underlying ledger entry (no cross-entry mixing) across test scenarios.
- **SC-005**: Pools without any IaaS-pool annotation or per-IP ledger status show zero measurable behavior change in allocation success rate, latency, or selection outcome compared to pre-feature behavior — no performance-sensitive path outside IaaS pools is affected, and the Pod creation critical path incurs no new external calls.
- **SC-006**: New annotation names, label names, status field names, and validation error messages are documented once and used identically across CRD status, webhook validation messages, and any related examples/docs.

## Assumptions

- This specification covers only the Spiderpool (open-source) side of the design described in `docs/develop/proposal-iaas-ip-provider.md`; the private IaaS provider controller that performs actual cloud sub-ENI creation/binding and writes the per-IP ledger status is an external dependency and out of scope for this feature's implementation, though its expected status shape is treated as a contract this feature must consume correctly.
- "IaaS pool" pools are node-pinned via existing `nodeName` and app-pinned via existing `podAffinity` fields; no new selector mechanism is introduced.
- Physical NIC reporting by spiderpool-agent (used by the external provider to resolve parent ports) and any CLI drift-detection tooling described in the proposal (`iaasnetctl`) are separate, lower-priority capabilities not required for the P0 scope of this feature; they may be addressed in a follow-up feature if needed.
- Per-IP ledger status fields are written by an external actor (the provider) using server-side apply with its own field manager; this feature only needs to read/consume that status, not write it, except for the existing `status.allocatedIPs`-style allocation bookkeeping Spiderpool already owns.
- Webhook mutating (annotation→label sync) and validating (pairing format/consistency, single-stack-Pod-on-paired-pool rejection) behavior described here follow the same admission webhook mechanism Spiderpool already uses for `SpiderIPPool`.
- Pool naming convention (e.g., `node<X>-<app>-<v4|v6>`) is a documented operator best practice for wildcard matching hygiene, not a system-enforced rule; existing `nodeName`/`podAffinity` filtering already prevents cross-application mismatches.
- TTL-based release, cross-node migration, and the `iaasnetctl` drift-correction CLI (proposal sections 7.2, 7.1, and 8) are explicitly out of scope for this feature (proposal stage P1/P2/P3); only P0 scope (annotations, pairing validation, per-IP ledger consumption, IPAM allocation changes) is covered.
