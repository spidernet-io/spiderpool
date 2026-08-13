# Phase 0 Research: IaaS Provider Prewarm IP Pool Support

No `NEEDS CLARIFICATION` markers remain in the Technical Context (all resolved
directly from repository investigation and the `/speckit-clarify` session).
This document records the concrete decisions and the codebase evidence backing
them.

## 1. Annotation/label naming and constants placement

- **Decision (revised — v5)**: Add
  `AnnoIPPoolIaasProvider = AnnotationPre + "/iaas-provider"`,
  `AnnoIPPoolPairPool = AnnotationPre + "/pair-pool"`, and
  `LabelIPPoolIaasProvider = AnnoIPPoolIaasProvider` (label mirrors the
  annotation key/value, matching the existing
  `LabelIPPoolReclaimIPPool = AnnoSpiderSubnetReclaimIPPool`
  aliasing pattern) in `pkg/constant/k8s.go`, plus a supported-vendor list
  (currently only `huaweicloud`) used by the validating webhook. The
  annotation value names the vendor rather than being a boolean.
- **Rationale**: `pkg/constant/k8s.go:63` already defines
  `AnnotationPre = "ipam.spidernet.io"` and all existing annotations/labels are
  built from it (`AnnoPodIPPool`, `AnnoSpiderSubnet`, `LabelIPPoolCIDR`, etc.).
  The proposal's annotation names (`ipam.spidernet.io/iaas-provider`,
  `ipam.spidernet.io/pair-pool`) already match this convention exactly, so no
  naming decision is required beyond adding the constants in the same place.
- **Alternatives considered**: A separate constants file for IaaS-specific
  names — rejected because every other pool-related annotation/label lives in
  `pkg/constant/k8s.go` and splitting would break the existing single-source-of
  -truth convention for annotation names.

## 2. Label sync mechanism (mutating webhook)

- **Decision**: Extend `mutateIPPool` in `pkg/ippoolmanager/ippool_mutate.go`
  with a block that mirrors the existing `LabelIPPoolCIDR` sync
  (`ippool_mutate.go:56-62`): if `ipPool.Annotations[AnnoIPPoolIaasProvider]`
  is set, ensure `ipPool.Labels[LabelIPPoolIaasProvider]` equals it; if the annotation is
  absent/changed, the label is corrected/removed to match (annotation is
  authoritative, per spec FR-001).
- **Rationale**: This is a direct structural precedent already in the file —
  same "read spec/annotation, ensure label consistency, log if changed"
  pattern — so this feature introduces no new mutating-webhook mechanism.
- **Alternatives considered**: A separate dedicated webhook for IaaS pools —
  rejected; `IPPoolWebhook` already centralizes all `SpiderIPPool`
  create/update mutation and validation (`ippool_webhook.go:49-128`), and
  splitting would duplicate webhook registration/manager wiring for no benefit.

## 3. Pairing validation rules & where they live

- **Decision**: Add validation functions alongside existing ones in
  `pkg/ippoolmanager/ippool_validate.go`, invoked from the same
  `ValidateCreate`/`ValidateUpdate` entrypoints in `ippool_webhook.go`. Rules
  (from spec FR-002/FR-003, refined by clarification):
  - Reject `pair-pool` self-reference and same-IP-version pairing immediately
    (no API lookup needed).
  - If the referenced pool exists, compute static capacity via the same
    `AssembleTotalIPs`/`IPsDiffSet`-based pattern already used in
    `validateIPPoolAvailableIPs` (`ippool_validate.go:97-156`) and
    `ippool_subnet_validate.go:42-90`, and require v4 capacity <= v6 capacity
    (not equality — per clarification).
  - Require identical `nodeName`/`podAffinity` between paired pools when the
    pair exists.
  - If the referenced pool does not exist yet, skip capacity/selector checks
    (do not reject) — pairing convergence happens later when the second pool
    is created (spec FR-003, proposal §4.2 timing note).
- **Rationale**: Reuses the existing `field.Invalid`/`field.Forbidden` +
  `apierrors.NewInvalid` error-construction style already established in this
  file, and reuses existing IP-set arithmetic helpers instead of introducing
  new ones.
- **Alternatives considered**: Enforcing exact IP-count equality — rejected per
  `/speckit-clarify` answer (v4 <= v6 capacity is the actual constraint, since
  a v6 surplus is harmless while a v4 surplus can never be paired).

## 4. Single-stack Pod requesting a paired pool

- **Decision**: No rejection anywhere (webhook or IPAM). A single-stack Pod
  requesting a paired pool is simply allocated the single address family it
  asked for, drawn from a metadata entry, leaving the entry's other-family
  address available for a future dual-stack claim. Single-stack detection
  reuses the existing `i.config.EnableIPv4`/`EnableIPv6` gates and per-family
  candidate-list splitting already present in
  `pkg/ipam/pool_selections.go:139-182` — no new detection logic is needed.
- **Rationale**: Direct clarification answer overriding the original proposal
  text ("禁止单栈 Pod 使用配对池"); simplifest implementation, avoids adding a
  new Pod-facing admission webhook, and keeps the change confined to the IPAM
  allocation path per Constitution Principle I (reuse existing package
  boundaries) and the spec's explicit "no new admission webhook" requirement.
- **Alternatives considered**: New Pod validating webhook (rejected — adds a
  new admission surface for a check the user decided is unnecessary); dual
  enforcement in both places (rejected — same reason, plus redundant code
  paths increase maintenance risk without a stated benefit).

## 5. Per-IP metadata status shape, serialization & occupancy determination

- **Decision (revised — v6)**: A single cloud-neutral structure,
  `Status.IPMetaData`, containing: `ParentNic string` (pool-level parent NIC),
  `Metadata *string` containing JSON whose decoded type is
  `map[string]IPMetadataEntry` (key = primary-family address; value carries
  `IPv6`/`MAC`/`VLAN`), `ObservedGeneration *int64`, and two provider-written
  observational counters `ReadyIPCount`/`UnreadyIPCount`. Presence of an
  address in the decoded map IS the ready state; there
  is no separate enum, no failed-IP list (failure = absence, counted in
  `UnreadyIPCount`), and NO `Status.Conditions` field. Occupancy ("is this
  entry claimed?") is still derived — NOT stored as a new field — by checking
  whether the entry's address(es) already appear in the pool's existing
  `Status.AllocatedIPs` (parsed via `convert.UnmarshalIPPoolAllocatedIPs`,
  same helper already used in `AllocateIP`). There is currently no periodic
  retry of failed addresses by the provider — an address either graduates
  into `Metadata` once prewarmed or stays absent, and reconciliation of that
  set (if any) is out of scope for this iteration.
- **Rationale**: The field naming is deliberately cloud-neutral (`ipMetaData`,
  not `iaasReadyIPs`): it stores generic per-IP link-layer/pairing metadata
  that any future feature could reuse. The decoded map still provides O(1)
  direct lookup, but storing it as a string avoids Kubernetes/controller
  machinery deep-copying and structurally serializing every entry whenever
  Spiderpool updates the unrelated high-frequency `status.allocatedIPs`.
  Agent informer handling parses each distinct authoritative metadata
  revision once and installs an immutable map snapshot; allocation never
  unmarshals the JSON per Pod.
- **Benchmark evidence (2026-08-12)**: local tests with the real
  `IPMetadataEntry` shape measured simulated allocation-cycle cost
  (DeepCopy + lookup/parse + outer status marshal). At 64 entries:
  structured map ~120 µs, JSON string without cache ~346 µs, JSON string with
  parsed cache ~39 µs. At 1000 entries: ~2.11 ms, ~5.29 ms, and ~0.54 ms
  respectively. Therefore string-without-cache was rejected; string plus
  parsed cache is one indivisible design decision. Kubernetes wire JSON for
  the string was ~15% larger due to escaping, while rendered YAML was ~20%
  smaller; performance motivation is CPU/allocation reduction, not wire size.
- **Alternatives considered**: A boolean/enum "claimed" field written directly
  on each metadata entry by Spiderpool — rejected per clarification (adds a
  second occupancy bookkeeping path that could drift from `AllocatedIPs`).
  The earlier two-list shape (`IaasReadyIPs`/`IaasFailedIPs` + `Conditions`)
  — superseded by this revision: IaaS-specific naming leaked scenario
  semantics into a generic CRD, the failed list carried data nobody consumed,
  and conditions duplicated what two counters express more cheaply.
  Keeping the structural map was also considered: it provides excellent
  direct lookup but incurs O(N) DeepCopy and structural serialization on
  every pool status update. JSON string without an agent cache was rejected
  because repeated full unmarshal made the hot path 2.5x slower at 1000
  entries.

## 5.1 Spec/status convergence without a phase field

- **Decision**: Add provider-owned `ObservedGeneration` under
  `status.ipMetaData`. Allocation requires
  `ObservedGeneration == metadata.generation`. The provider publishes the
  metadata string, counters, and observed generation atomically after a
  complete, trustworthy evaluation of that generation. Individual IP
  failures remain a valid completed result: successful entries are published
  and failed entries are absent/count unready.
- **Rationale**: After an administrator edits `spec.ips`, Spiderpool and the
  provider both receive the spec Update. Spiderpool may temporarily observe
  new spec with old status. Generation mismatch identifies this window
  immediately and deterministically, so stale metadata cannot be used.
  Spiderpool's existing IPPool informer enqueues all Update events, including
  pure status updates, and therefore sees the provider's final publication.
- **Alternatives considered**: A webhook-written `phase=Updating` was
  rejected because status-subresource fields are not reliably mutated by a
  normal object admission update and a stale `Ready` window would remain
  before the provider writes Updating. A provider-written phase plus
  generation would be redundant for allocation safety. Agent-only version
  state was rejected because it is lost on restart and cannot establish which
  spec generation the provider actually completed.

## 5. Propagating ledger-origin to skip the existing synchronous provider call (FR-015)

- **Decision**: Change the `IPPoolManager.AllocateIP` interface method and its
  implementation (interface at `pkg/ippoolmanager/ippool_manager.go:36`,
  implementation at `ippool_manager.go:96`) to return an additional `bool`
  result — `(ip *models.IPConfig, fromIaasLedger bool, err error)` — set to
  `true` only when the returned IP was selected via the readiness intersection
  against the current-generation decoded metadata snapshot. Add a matching
  `FromIaasLedger bool` field to `types.AllocationResult`
  (`pkg/types/ip.go:12-16`) and set it at the single call site that builds
  `types.AllocationResult` from this return value
  (`pkg/ipam/allocate.go:621-629`, inside `allocateIPFromCandidate`). Then, in
  `pkg/ipam/allocate.go:439-444` (`allocateInStandardMode`), before invoking
  `i.callIaaSAllocate(ctx, pod, results)`, filter `results` to exclude any
  entry with `FromIaasLedger == true`; call the existing synchronous provider
  API only with the remaining (non-ledger) results, and skip the call
  entirely if none remain.
- **Rationale**: `pkg/ipam/allocate.go:439` already gates the existing
  synchronous `callIaaSAllocate` (private provider API call) behind
  `i.config.IaaSClient != nil` — a cluster-wide toggle from the prior
  `003-iaas-provider-integration` feature. Left unchanged, every Pod would
  still incur a synchronous cloud API call even when its IP was already
  prewarmed, directly defeating the "关键路径无云 API" goal that motivates this
  entire feature. Changing the interface signature (rather than smuggling the
  flag through `models.IPConfig`, which is an OpenAPI-generated type under
  `api/`) keeps this feature's data flow entirely inside hand-written Go types
  and avoids any OpenAPI spec/codegen changes. This is a discovered
  integration requirement (confirmed with the user) and is captured as spec
  FR-015 / User Story 3 Acceptance Scenario 6.
- **Alternatives considered**: Leaving `callIaaSAllocate` unconditional —
  rejected, reintroduces the exact scaling problem (proposal §1.1) this
  feature exists to solve. Adding the flag to `models.IPConfig` instead —
  rejected because that type is OpenAPI-generated (`api/v1/**/openapi.yaml` ->
  generated client models) and mutating it would require an unrelated OpenAPI
  regeneration for a purely internal signal. Gating by pool annotation
  (`iaas-provider`) instead of per-result metadata origin — rejected because a
  paired pool can serve a single-stack Pod (per clarification #1) that only
  partially touches metadata, and non-metadata IPs from the same pool (e.g.,
  before prewarming completes, or pools without any metadata populated yet)
  must still go through the existing synchronous path for backward
  compatibility (FR-011).

## 6. Allocation-path integration point & selection order (revised — intersection model)

- **Decision**: Whether metadata-gating applies to a pool is decided solely by
  the `iaas-provider` label (§1.1), not by whether `Status.IPMetaData` happens
  to be empty. For an IaaS-labeled pool, allocation first requires
  `ObservedGeneration == metadata.generation` and a matching immutable decoded
  cache snapshot. `AllocateIP`
  (`pkg/ippoolmanager/ippool_manager.go`, around `ippool_manager.go:167-238`)
  still computes the normal candidate set exactly as today — `spec.ips` minus
  `excludeIPs`/`reservedIPs`/already-`usedIPs`, via the existing
  `spiderpoolip.FindAvailableIPs` call, in the same ascending-address order —
  and then intersects that candidate set with keys in the decoded snapshot.
  The first candidate that is also present is selected; its entry's
  `MAC`/`VLAN` are copied onto the resulting `IPConfig`. If the intersection is empty, the function returns the same
  `constant.ErrIPUsedOut`-class error used for ordinary pool exhaustion — not
  a distinct error path. For a pool without the `iaas-provider` label, the
  `IPMetaData` field is never consulted, regardless of
  content, and behavior is byte-for-byte unchanged from before this feature.
- **Rationale**: The original "non-empty ledger replaces spec.ips entirely"
  design (see history below) created an awkward window: a freshly created
  IaaS pool with empty metadata would either have to leave `spec.ips`
  empty (unusual/confusing for an otherwise-normal-looking pool object) or
  silently fall back to un-prewarmed static allocation (defeats the feature).
  The user's clarification resolves this: `spec.ips` for an IaaS pool is
  populated normally, exactly like any other pool, and the label alone (not
  metadata emptiness) decides whether the extra readiness intersection applies.
  This also means `Metadata` entries never need their own "is this
  address within spec.ips" validation — intersecting against the
  already-range/exclusion-scoped candidate set does that for free, and any
  stale/out-of-range/duplicate metadata entry is silently ignored rather than
  needing to be rejected.
- **Alternatives considered (superseded)**: The originally-implemented
  approach — `len(ipPool.Status.IaasIPs) > 0` as the sole gate, selecting
  directly from the ledger and never consulting `spec.ips` when the ledger is
  non-empty — is now superseded by the intersection model above; it is kept
  here only as historical context for why the revision was needed. A separate
  `AllocateIaasIP` function selected via caller branching remains rejected for
  the same reason as before: it would duplicate the
  `allocatedRecords`/`usedIPs` computation and the Pod-UID-reuse fast path
  (`ippool_manager.go:172-180`), risking divergence between the two code
  paths over time.

## 7. Dual-stack pairing without candidate auto-completion (pair-or-nothing)

- **Decision**: Do NOT auto-complete the sibling v6 pool into the Pod's
  candidate lists. The Pod declares only the v4 primary pool; when dual-stack
  is enabled, a paired IaaS v4 primary candidate is served by
  `AllocateIPPair` (`pkg/ippoolmanager/ippool_manager.go`), which selects one
  metadata entry whose BOTH sides are currently available and commits the v4
  and v6 allocation records to the two pools' statuses. `selectByPod`
  (`pkg/ipam/allocate.go`) filters the sibling v6 pool out of standalone v6
  candidacy so its addresses are only ever allocated through the primary
  pool.
- **Rationale**: v4/v6 candidates are otherwise allocated by independent
  concurrent goroutines; selecting each family separately can pair addresses
  from two different metadata entries when v4→v6 mappings are not
  order-aligned (violating SC-004). Selecting both sides from one entry at
  one point eliminates cross-entry mixing by construction, and per-entry
  convergence on retry is guaranteed by the Pod-UID fast path.
- **Alternatives considered**: (a) Candidate auto-completion in
  `getPoolCandidates` plus independent per-family allocation — rejected
  because it cannot guarantee same-entry pairing without cross-goroutine
  coordination; (b) sharing the selected entry between the two goroutines via
  synchronization — rejected as far more invasive to the existing concurrent
  allocation pipeline.

## 8. Generated artifacts

- **Decision**: Any change to `SpiderIPPoolSpec`/`SpiderIPPoolStatus` Go types
  must be followed by `make manifests generate-k8s-api` before the feature is
  considered complete, regenerating
  `charts/spiderpool/crds/spiderpool.spidernet.io_spiderippools.yaml` and
  `pkg/k8s/apis/spiderpool.spidernet.io/v2beta1/zz_generated.deepcopy.go`.
- **Rationale**: Required by Constitution Principle V (Generated Artifacts
  Follow Source Definitions); manual edits to generated files are prohibited.
- **Alternatives considered**: Manually hand-editing the CRD YAML — rejected,
  explicitly disallowed by the constitution.

## 9. Documentation

- **Decision (revised — v5)**: Keep the user-facing
  `docs/usage/iaas-network-provider.md` page high-level: it may mention the
  `iaas-provider`/`pair-pool` annotations, but MUST NOT expose prewarm
  internals (`status.ipMetaData` shape, readiness-intersection mechanics) —
  those live in this spec/contract and the provider proposal only. A separate
  node-pool-oriented feature doc may be added later. Any doc change updates
  the `zh_CN` counterpart in the same change, per repo-wide docs convention
  (`docs/` bilingual requirement).
- **Rationale**: Constitution Principle III requires docs updates for
  user-facing changes (new annotations/status fields are user/operator
  facing).
