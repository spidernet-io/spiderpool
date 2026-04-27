# Research: VLAN CNI Support in Spiderpool

**Generated**: 2026-03-17  
**Feature**: VLAN CNI Support (specs/001-vlan-cni/spec.md)

## Decision 1: Use Dedicated `Vlan` Block in `SpiderMultusConfig`

**Decision**: Introduce a new `Vlan` field in `MultusCNIConfigSpec` and extend the `cniType` enum with `vlan`.

**Rationale**: 

- Existing SpiderMultusConfig organizes support through explicit per-CNI config blocks (macvlan, ipvlan, sriov, ovs, ipoib, etc.)
- A dedicated block maintains consistency with validation, defaulting, and NAD translation patterns
- Prevents confusion with macvlan's VLAN mode which has different runtime semantics

**Alternatives Considered**:

- Reuse `macvlan` config for VLAN: **REJECTED** - Runtime model differs significantly
- Store VLAN config in `customCNI`: **REJECTED** - Loses typed validation and automatic NAD generation

## Decision 2: Reuse macvlan-style API Shape, Not macvlan Runtime Semantics

**Decision**: Reuse field shape and ergonomics from `macvlan` where it improves consistency, but generate native VLAN CNI config instead of treating VLAN as a macvlan variant.

**Rationale**:

- The feature spec explicitly requires distinguishing `macvlan + vlanID` from native `vlan` CNI
- In macvlan mode, VLAN is only part of host-side parent preparation; in VLAN CNI mode, the Pod uses the VLAN interface directly
- API consistency reduces cognitive load while runtime correctness ensures proper behavior

**Alternatives Considered**:

- Mirror macvlan generation exactly: **REJECTED** - Misrepresents target runtime behavior
- Create wholly different API unrelated to macvlan: **REJECTED** - Reduces consistency for SpiderMultusConfig users

## Decision 3: VLAN Plugin as Primary Plugin in NAD

**Decision**: The generated NAD config for `cniType: vlan` should use a VLAN plugin as the main plugin, with Spiderpool IPAM attached when IPAM is enabled.

**Rationale**:

- Matches the target runtime behavior
- Keeps informer's responsibility aligned with other built-in CNI types
- Enables correct VLAN interface creation in Pod namespace

**Alternatives Considered**:

- Use ifacer to fully create VLAN interface and skip native VLAN plugin: **REJECTED** - Pushes VLAN ownership into wrong layer
- Emit custom JSON only: **REJECTED** - First-class translation is a core SpiderMultusConfig goal

## Decision 4: Separate Bond Preparation from VLAN Creation for Multi-NIC

**Decision**: For multiple masters, use a preparatory step to create or prepare a bond device before VLAN CNI uses that bond as parent.

**Rationale**:

- Preserves semantics that VLAN creation belongs to VLAN plugin
- Supports multi-master ergonomics already present in macvlan/ipvlan integration
- Plugin chain order: ifacer (bond) → vlan (VLAN on bond)

**Alternatives Considered**:

- Reuse current ifacer VLAN path unchanged: **REJECTED** - Would imply VLAN creation owned by preparatory step
- Disallow multi-master for VLAN: **REJECTED** - Would unnecessarily diverge from existing capabilities

## Decision 5: Mirror Existing Typed CNI Validation Patterns

**Decision**: Add VLAN-specific validation to `multusconfig_validate.go`, including checks for required config presence, VLAN ID range, master/bond consistency, and incompatible mixed config blocks.

**Rationale**:

- Existing CNI types use centralized typed validation model
- Most maintainable place to enforce consistency
- Early validation provides faster feedback than informer-time rejection

**Alternatives Considered**:

- Validate only via kubebuilder tags: **REJECTED** - Cross-field rules already live in webhook logic
- Defer validation until informer time: **REJECTED** - Slower feedback, complicates reconciliation

## Decision 6: Extend Existing SpiderMultusConfig Usage Docs

**Decision**: Update both `docs/usage/spider-multus-config.md` and `docs/usage/spider-multus-config-zh_CN.md` to include VLAN examples and runtime explanation.

**Rationale**:

- User specifically requested usage docs under existing SpiderMultusConfig documentation area
- Users already look there for built-in CNI examples
- Maintains documentation consistency

**Alternatives Considered**:

- Create standalone VLAN-only docs first: **REJECTED** - Would fragment usage story
- Document only in `.specs`: **REJECTED** - `.specs` is planning material, not user-facing

## Decision 7: No Contracts Artifact Required

**Decision**: Do not generate `/contracts/*` for this plan.

**Rationale**:

- Feature extends internal CRD/controller/webhook/doc flows
- Does not expose new external service API, CLI command schema, or endpoint contract
- NAD JSON is implementation output, not stable public contract in this workflow

**Alternatives Considered**:

- Add pseudo-contract for NAD JSON: **REJECTED** - Not a stable public contract for this feature type

## Decision 8: Sort Auto-Injected Multus Networks by Stable NIC-Oriented Keys

**Decision**: When the pod webhook auto-injects `k8s.v1.cni.cncf.io/networks` from matched `SpiderMultusConfig` resources, sort the matched configs before constructing the annotation string.

**Rationale**:

- Current implementation in `pkg/podmanager/utils.go` iterates directly over `SpiderMultusConfigList.Items`, so the resulting network order depends on Kubernetes list return order.
- The injected network order determines the Pod interface attachment order, which is significant for GPU/NIC affinity scenarios and documentation examples.
- Sorting by a NIC-oriented key matches user mental models better than creation timestamp or object name.

**Chosen Sort Strategy**:

- For single-master `macvlan`, `ipvlan`, and `vlan`, use the first master interface name.
- For multi-master `macvlan`, `ipvlan`, and `vlan`, use `bond.name` instead of the raw master list.
- For `sriov`, use the stable NIC identity exposed by the config, currently `spec.sriov.resourceName`.
- Use `namespace/name` as a stable secondary key when the primary key collides.

**Alternatives Considered**:

- Preserve API list order: **REJECTED** - Nondeterministic and user-visible
- Sort by object name only: **REJECTED** - Does not reflect intended NIC order when names are arbitrary
- Sort by creation timestamp: **REJECTED** - Stable only accidentally and unrelated to desired interface order
- Sort multi-master configs by serialized master slice: **REJECTED** - Sensitive to declaration order and does not express the actual bonded interface identity

## Decision 9: Support Namespace-Scoped Tenant Default Injection for `network-resource-inject`

**Decision**: Extend pod webhook auto-injection so `cni.spidernet.io/network-resource-inject` can be sourced from the Pod first, and if absent, from the Pod's Namespace as a tenant default.

**Rationale**:

- The target multi-tenant model maps one VLAN to one tenant, and one tenant is naturally represented by one Namespace.
- Requiring every Pod to repeat the same injection annotation is operationally noisy and error-prone.
- Namespace-scoped defaults preserve the existing Pod annotation behavior while enabling tenant-level policy.

**Alternatives Considered**:

- Keep Pod-only annotations: **REJECTED** - Too repetitive for tenant-per-namespace environments.
- Make Namespace annotation override Pod annotation: **REJECTED** - Removes workload-level escape hatch and is less intuitive than explicit Pod override.
- Introduce a new custom CRD for tenant defaults: **REJECTED** - Adds API surface and controller complexity for a simple inheritance rule.

## Decision 10: Pod Annotation Takes Precedence Over Namespace Annotation

**Decision**: When both the Pod and the Namespace define `cni.spidernet.io/network-resource-inject`, the Pod value wins and Namespace is only used as a fallback.

**Rationale**:

- Explicit workload intent should override inherited tenant defaults.
- This mirrors common Kubernetes override patterns where lower-scope object settings beat higher-scope defaults.
- The precedence rule is simple to explain, test, and debug.

**Alternatives Considered**:

- Namespace-first precedence: **REJECTED** - Surprising to users and blocks per-Pod exceptions.
- Merge both annotation values: **REJECTED** - Ambiguous semantics and risks over-injection.

## Decision 11: Use Cached Namespace Reads During Admission

**Decision**: Resolve Namespace annotations through the existing controller-runtime cache path, using the namespace manager abstraction with `constant.UseCache`, rather than issuing a live API read for every webhook request.

**Rationale**:

- Pod admission can be high-frequency, so direct live reads would add avoidable APIServer pressure.
- The project already has a `NamespaceManager` abstraction supporting cached and uncached reads.
- The webhook runs inside the controller manager process, which already maintains the informer-backed cache needed for Namespace objects.

**Alternatives Considered**:

- Use direct APIReader lookups in the webhook: **REJECTED** - Higher APIServer load and unnecessary given existing cache support.
- Cache Namespace annotations in a custom in-memory map: **REJECTED** - Duplicates controller-runtime cache responsibilities and adds invalidation complexity.

## Technical Dependencies Identified

| Dependency | Purpose | Source |
|------------|---------|--------|
| VLAN CNI Plugin | Creates VLAN interfaces in Pod namespace | External CNI plugin |
| Spiderpool IPAM | IP allocation for VLAN interfaces | Existing project component |
| Multus CNI | Multi-network attachment for Pods | Existing project dependency |
| ifacer Plugin | Bond device preparation for multi-NIC | Existing project component |
| NamespaceManager cache path | Read Namespace annotations without live per-request API calls | Existing project component |

## Key Constraints

1. **Backward Compatibility**: VLAN feature must not impact existing CNI types (macvlan, ipvlan, sriov, etc.)
2. **Runtime Model Correctness**: VLAN CNI must directly create VLAN interfaces, not nest under macvlan
3. **Validation Consistency**: Webhook validation must follow same patterns as existing CNI types
4. **Documentation Parity**: VLAN documentation must match quality and structure of existing CNI documentation
5. **Deterministic Pod Mutation**: Auto-injected Multus annotation ordering must be reproducible across repeated reconciliations and admissions
6. **Hierarchical Annotation Resolution**: `cni.spidernet.io/network-resource-inject` must resolve by `Pod` first, then `Namespace`
7. **Admission Efficiency**: Namespace annotation reads must avoid direct per-request APIServer traffic where the existing cache can be used

## Unknowns Resolved

| Unknown | Resolution | Rationale |
|---------|------------|-----------|
| VLAN ID 0 validity | Valid | VLAN ID 0 represents untagged/native VLAN, commonly used |
| Bond mode default | Must specify | User must explicitly configure bond mode |
| MTU inheritance | Kernel default | If not specified, kernel determines MTU |
| Auto-injection sort key for multi-master configs | `bond.name` | Best represents the interface identity that will appear to consumers |
| Pod vs Namespace annotation precedence | `Pod` wins, `Namespace` falls back | Preserves explicit workload intent while supporting tenant defaults |
| Namespace read mode during webhook mutation | Use controller cache via namespace manager | Minimizes APIServer load and matches existing project patterns |

## No [NEEDS CLARIFICATION] Markers

All requirements from the feature specification have been resolved without needing user clarification. The spec is self-contained and ready for implementation planning.
