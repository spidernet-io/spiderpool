# Phase 0 Research: IaaS Provider Prewarm IP Pool Support

No `NEEDS CLARIFICATION` markers remain in the Technical Context (all resolved
directly from repository investigation and the `/speckit-clarify` session).
This document records the concrete decisions and the codebase evidence backing
them.

## 1. Annotation/label naming and constants placement

- **Decision**: Add `AnnoIPPoolIaas = AnnotationPre + "/iaas-pool"`,
  `AnnoIPPoolPairPool = AnnotationPre + "/pair-pool"`, and
  `LabelIPPoolIaas = AnnoIPPoolIaas` (label mirrors the annotation key/value,
  matching the existing `LabelIPPoolReclaimIPPool = AnnoSpiderSubnetReclaimIPPool`
  aliasing pattern) in `pkg/constant/k8s.go`.
- **Rationale**: `pkg/constant/k8s.go:63` already defines
  `AnnotationPre = "ipam.spidernet.io"` and all existing annotations/labels are
  built from it (`AnnoPodIPPool`, `AnnoSpiderSubnet`, `LabelIPPoolCIDR`, etc.).
  The proposal's annotation names (`ipam.spidernet.io/iaas-pool`,
  `ipam.spidernet.io/pair-pool`) already match this convention exactly, so no
  naming decision is required beyond adding the constants in the same place.
- **Alternatives considered**: A separate constants file for IaaS-specific
  names — rejected because every other pool-related annotation/label lives in
  `pkg/constant/k8s.go` and splitting would break the existing single-source-of
  -truth convention for annotation names.

## 2. Label sync mechanism (mutating webhook)

- **Decision**: Extend `mutateIPPool` in `pkg/ippoolmanager/ippool_mutate.go`
  with a block that mirrors the existing `LabelIPPoolCIDR` sync
  (`ippool_mutate.go:56-62`): if `ipPool.Annotations[AnnoIPPoolIaas]` is set,
  ensure `ipPool.Labels[LabelIPPoolIaas]` equals it; if the annotation is
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
  asked for, drawn from a ledger entry, leaving the entry's other-family
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

## 5. Per-IP ledger status shape & occupancy determination

- **Decision (revised)**: Split the ledger into two slices instead of one —
  `Status.IaasReadyIPs []IaasReadyIPAllocation` (IPv4/IPv6/MAC/VLANID) and
  `Status.IaasFailedIPs []IaasFailedIPAllocation` (IPv4/IPv6 only) — with no
  per-entry `Phase`/`LastError` field at all (finalized in `data-model.md`).
  Membership in `IaasReadyIPs` itself IS the ready state; there is no separate
  enum. Occupancy ("is this entry claimed?") is still derived — NOT stored as
  a new field — by checking whether the entry's address(es) already appear in
  the pool's existing `Status.AllocatedIPs` (parsed via
  `convert.UnmarshalIPPoolAllocatedIPs`, same helper already used in
  `AllocateIP`, `ippool_manager.go:167`). Also add `Status.Conditions
  []metav1.Condition` for the `IaasReady`-style summary condition, using the
  standard `k8s.io/apimachinery/pkg/api/meta.SetStatusCondition` helper (first
  use of this exact helper in this repo, but it is the standard Kubernetes API
  machinery pattern, not a bespoke addition). There is currently no periodic
  retry of `IaasFailedIPs` entries by the provider — an address either
  graduates into `IaasReadyIPs` once prewarmed or stays recorded in
  `IaasFailedIPs`, and reconciliation of that list (if any) is out of scope
  for this iteration.
- **Rationale**: A single `Phase` enum with values `Ready`/`NotReady`/
  `Releasing` conflated two different concerns — "did prewarm succeed or
  fail" (a one-time outcome the Ready/Failed list split now captures
  structurally) and "is this address currently allocatable" (which, after the
  intersection-model revision below, is answered purely by set membership,
  not by a status flag). Splitting into two lists removes an enum Spiderpool
  never needed to switch on beyond "== Ready", and removes `LastError`, which
  Spiderpool never read (diagnostic detail belongs on the provider's own
  status/Conditions/logs, not on a field Spiderpool must model and validate).
- **Alternatives considered**: A boolean/enum "claimed" field written directly
  on each ledger entry by Spiderpool — rejected per clarification (adds a
  second occupancy bookkeeping path that could drift from `AllocatedIPs`).
  Keeping the single `IaasIPAllocation`+`Phase` shape — rejected per user
  clarification: two lists with no enum is simpler to produce (provider just
  appends to the right list) and simpler to consume (Spiderpool only ever
  reads `IaasReadyIPs`).

## 5. Propagating ledger-origin to skip the existing synchronous provider call (FR-015)

- **Decision**: Change the `IPPoolManager.AllocateIP` interface method and its
  implementation (interface at `pkg/ippoolmanager/ippool_manager.go:36`,
  implementation at `ippool_manager.go:96`) to return an additional `bool`
  result — `(ip *models.IPConfig, fromIaasLedger bool, err error)` — set to
  `true` only when the returned IP was selected via the readiness intersection
  against `status.iaasReadyIPs`. Add a matching `FromIaasLedger bool` field to `types.AllocationResult`
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
  (`iaas-pool`) instead of per-result ledger origin — rejected because a
  paired pool can serve a single-stack Pod (per clarification #1) that only
  partially touches ledger data, and non-ledger IPs from the same pool (e.g.,
  before prewarming completes, or pools without any ledger populated yet)
  must still go through the existing synchronous path for backward
  compatibility (FR-011).

## 6. Allocation-path integration point & selection order



## 6. Allocation-path integration point & selection order (revised — intersection model)

- **Decision**: Whether ledger-gating applies to a pool is decided solely by
  the `iaas-pool` label (§1.1), not by whether `Status.IaasReadyIPs` happens
  to be empty. For an `iaas-pool`-labeled pool, `AllocateIP`
  (`pkg/ippoolmanager/ippool_manager.go`, around `ippool_manager.go:167-238`)
  still computes the normal candidate set exactly as today — `spec.ips` minus
  `excludeIPs`/`reservedIPs`/already-`usedIPs`, via the existing
  `spiderpoolip.FindAvailableIPs` call, in the same ascending-address order —
  and then intersects that candidate set with the addresses present in
  `Status.IaasReadyIPs`. The first candidate that is also present in
  `IaasReadyIPs` is selected; its `MAC`/`VLANID` are copied onto the resulting
  `IPConfig`. If the intersection is empty, the function returns the same
  `constant.ErrIPUsedOut`-class error used for ordinary pool exhaustion — not
  a distinct error path. For a pool without the `iaas-pool` label, the
  `IaasReadyIPs`/`IaasFailedIPs` fields are never consulted, regardless of
  content, and behavior is byte-for-byte unchanged from before this feature.
- **Rationale**: The original "non-empty ledger replaces spec.ips entirely"
  design (see history below) created an awkward window: a freshly created
  `iaas-pool` with an empty ledger would either have to leave `spec.ips`
  empty (unusual/confusing for an otherwise-normal-looking pool object) or
  silently fall back to un-prewarmed static allocation (defeats the feature).
  The user's clarification resolves this: `spec.ips` for an `iaas-pool` is
  populated normally, exactly like any other pool, and the label alone (not
  ledger emptiness) decides whether the extra readiness intersection applies.
  This also means `IaasReadyIPs` entries never need their own "is this
  address within spec.ips" validation — intersecting against the
  already-range/exclusion-scoped candidate set does that for free, and any
  stale/out-of-range/duplicate ledger entry is silently ignored rather than
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

## 7. Pool candidate auto-completion for dual-stack pairing

- **Decision**: Extend pool candidate resolution in
  `pkg/ipam/pool_selections.go` (in `getPoolCandidates`, near the v4/v6
  splitting logic at lines 139-182) so that, after the existing
  annotation/wildcard resolution produces a candidate pool set, any candidate
  pool carrying `AnnoIPPoolPairPool` whose opposite-family pool is not already
  present in the resolved set gets that paired pool appended to the
  corresponding IPv4/IPv6 candidate list.
- **Rationale**: This is the natural single integration point already
  responsible for producing the final v4/v6 candidate lists per spec FR-005;
  it runs once per Pod IPAM request and needs no cloud API calls.
- **Alternatives considered**: Doing auto-completion inside `selectByPod`
  (`pkg/ipam/allocate.go:705-780`) — rejected because that function's job is
  narrowing an already-resolved candidate list by node/namespace/pod-affinity,
  not expanding it; expanding there would require re-running the earlier
  wildcard/annotation resolution logic redundantly.

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

- **Decision**: Add/update one English usage/concepts doc page describing the
  `iaas-pool`/`pair-pool` annotations, the synchronized label, and the
  `status.iaasReadyIPs`/`status.iaasFailedIPs`/`status.conditions` shape, plus the synchronized
  `zh_CN` counterpart in the same change, per repo-wide docs convention
  (`docs/` bilingual requirement).
- **Rationale**: Constitution Principle III requires docs updates for
  user-facing changes (new annotations/status fields are user/operator
  facing).
