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

## Technical Dependencies Identified

| Dependency | Purpose | Source |
|------------|---------|--------|
| VLAN CNI Plugin | Creates VLAN interfaces in Pod namespace | External CNI plugin |
| Spiderpool IPAM | IP allocation for VLAN interfaces | Existing project component |
| Multus CNI | Multi-network attachment for Pods | Existing project dependency |
| ifacer Plugin | Bond device preparation for multi-NIC | Existing project component |

## Key Constraints

1. **Backward Compatibility**: VLAN feature must not impact existing CNI types (macvlan, ipvlan, sriov, etc.)
2. **Runtime Model Correctness**: VLAN CNI must directly create VLAN interfaces, not nest under macvlan
3. **Validation Consistency**: Webhook validation must follow same patterns as existing CNI types
4. **Documentation Parity**: VLAN documentation must match quality and structure of existing CNI documentation

## Unknowns Resolved

| Unknown | Resolution | Rationale |
|---------|------------|-----------|
| VLAN ID 0 validity | Valid | VLAN ID 0 represents untagged/native VLAN, commonly used |
| Bond mode default | Must specify | User must explicitly configure bond mode |
| MTU inheritance | Kernel default | If not specified, kernel determines MTU |

## No [NEEDS CLARIFICATION] Markers

All requirements from the feature specification have been resolved without needing user clarification. The spec is self-contained and ready for implementation planning.
