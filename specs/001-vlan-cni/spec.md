# Feature Specification: VLAN CNI Support in Spiderpool

**Branch**: `001-vlan-cni` | **Created**: 2026-03-17 | **Status**: Draft  
**Spec File**: `specs/001-vlan-cni/spec.md`

## Overview

Add native VLAN CNI support to Spiderpool, allowing users to configure VLAN networks through `SpiderMultusConfig`, automatically translate to `NetworkAttachmentDefinition`, validate via webhook, and support namespace-scoped tenant default auto-injection for Multus networks.

### Feature Description

Spiderpool currently supports multiple CNI types (macvlan, ipvlan, sriov, etc.), but lacks native VLAN CNI support. This feature will:

1. Extend `SpiderMultusConfig` CRD with `cniType: vlan` type
2. Add `Vlan` configuration block supporting `master`, `vlanID`, `ippools`, `rdmaResourceName` fields
3. Implement automatic translation from VLAN configuration to NAD in the multusconfig informer
4. Add VLAN configuration validation logic in the webhook
5. Support namespace-scoped `cni.spidernet.io/network-resource-inject` defaults for tenant-style VLAN isolation
6. Make pod webhook auto-injection resolve `Pod` annotations first and fall back to `Namespace` annotations using cached reads
7. Update usage documentation with VLAN CNI configuration examples and namespace-scoped injection behavior

### Primary Goals

- Users can define VLAN CNI network configuration through `SpiderMultusConfig`
- VLAN configuration automatically translates to valid `NetworkAttachmentDefinition` CNI JSON
- Invalid VLAN configurations are rejected at the webhook stage
- A namespace can define the tenant-default `cni.spidernet.io/network-resource-inject` value for Pods inside that namespace
- Pod-level `cni.spidernet.io/network-resource-inject` overrides the namespace default when both are present
- Namespace lookup during Pod mutation avoids frequent direct APIServer calls by using the controller cache
- Documentation clearly explains the difference between VLAN CNI and macvlan+VlanID usage

### Out of Scope

- Implement new IPAM modes
- Replace or refactor existing macvlan/ipvlan implementations
- Automatic discovery and orchestration of VLAN ranges
- Add new controllers or APIs outside existing SpiderMultusConfig management flows

## User Scenarios

### Scenario 1: Basic VLAN Network Configuration

**Background**: Cluster administrator needs to create a VLAN 100 secondary network for a specific business line

**Trigger**: Administrator creates SpiderMultusConfig

**Scenario Flow**:

```yaml
apiVersion: spiderpool.spidernet.io/v2beta1
kind: SpiderMultusConfig
metadata:
  name: vlan-100-network
  namespace: production
spec:
  cniType: vlan
  vlan:
    master: ["eth0"]
    vlanID: 100
    ippools:
      ipv4: ["vlan100-pool"]
```

**Acceptance Criteria**:

- [ ] CRD accepts `cniType: vlan` and `vlan` fields
- [ ] Generated NAD contains valid VLAN CNI JSON configuration
- [ ] VLAN ID 100 is correctly passed to VLAN CNI plugin

### Scenario 2: Multi-NIC Bond VLAN Configuration

**Background**: User needs to create a VLAN network based on bond device for high-availability scenarios

**Trigger**: Administrator specifies multiple master interfaces and bond configuration

**Scenario Flow**:

```yaml
apiVersion: spiderpool.spidernet.io/v2beta1
kind: SpiderMultusConfig
metadata:
  name: vlan-bond-network
  namespace: production
spec:
  cniType: vlan
  vlan:
    master: ["eth0", "eth1"]
    vlanID: 200
    bond:
      name: "bond0"
      mode: 4
    ippools:
      ipv4: ["vlan200-pool"]
```

**Acceptance Criteria**:

- [ ] Multi-master configuration is accepted
- [ ] NAD contains correct plugin chain (ifacer + vlan)
- [ ] After bond device creation, VLAN CNI uses bond0 as master

### Scenario 3: RDMA-Accelerated VLAN Network

**Background**: AI/ML workloads require RDMA-accelerated VLAN network

**Trigger**: Administrator configures RDMA resource name

**Scenario Flow**:

```yaml
apiVersion: spiderpool.spidernet.io/v2beta1
kind: SpiderMultusConfig
metadata:
  name: vlan-rdma-network
  annotations:
    ipam.spidernet.io/pod-resource-inject: "true"
spec:
  cniType: vlan
  vlan:
    master: ["eth0"]
    vlanID: 300
    rdmaResourceName: "rdma_shared_device"
    ippools:
      ipv4: ["vlan300-pool"]
```

**Acceptance Criteria**:

- [ ] `rdmaResourceName` field takes effect
- [ ] Webhook validates RDMA resource configuration validity
- [ ] After resource injection annotation triggers, Pod resources include RDMA device

### Scenario 4: Namespace-Scoped Tenant Default Injection

**Background**: In the multi-tenant model, one VLAN corresponds to one tenant, and the tenant is mapped to a Kubernetes namespace.

**Trigger**: Administrator annotates the namespace with a tenant VLAN injection value, while a Pod in that namespace omits the Pod-level annotation.

**Scenario Flow**:

```yaml
apiVersion: v1
kind: Namespace
metadata:
  name: tenant-a
  annotations:
    cni.spidernet.io/network-resource-inject: vlan100
---
apiVersion: v1
kind: Pod
metadata:
  name: tenant-a-workload
  namespace: tenant-a
spec:
  containers:
  - name: app
    image: alpine
    command: ["sleep", "infinity"]
```

**Acceptance Criteria**:

- [ ] Pod webhook injects networks when the Pod lacks `cni.spidernet.io/network-resource-inject` but its namespace provides it
- [ ] The namespace annotation is treated as the tenant default and matched against `SpiderMultusConfig` labels
- [ ] Namespace resolution uses cached reads rather than per-request direct APIServer calls

### Scenario 5: Pod Annotation Overrides Namespace Default

**Background**: A namespace defines the tenant-default VLAN injection value, but a specific Pod requires an explicit override.

**Trigger**: Both the namespace and the Pod define `cni.spidernet.io/network-resource-inject` with different values.

**Scenario Flow**:

```yaml
apiVersion: v1
kind: Namespace
metadata:
  name: tenant-a
  annotations:
    cni.spidernet.io/network-resource-inject: vlan100
---
apiVersion: v1
kind: Pod
metadata:
  name: tenant-a-override
  namespace: tenant-a
  annotations:
    cni.spidernet.io/network-resource-inject: vlan200
spec:
  containers:
  - name: app
    image: alpine
    command: ["sleep", "infinity"]
```

**Acceptance Criteria**:

- [ ] Pod-level `cni.spidernet.io/network-resource-inject` takes precedence over the namespace annotation
- [ ] Namespace annotation is only consulted when the Pod-level annotation is absent
- [ ] The effective annotation source is deterministic and documented

### Scenario 6: Configuration Validation Failure

**Background**: User submitted invalid VLAN configuration

**Trigger**: VLAN ID out of range or missing required fields

**Scenario Flow**:

```yaml
# Error example: vlanID out of range
spec:
  cniType: vlan
  vlan:
    master: ["eth0"]
    vlanID: 5000  # Invalid: exceeds 0-4094 range
```

**Acceptance Criteria**:

- [ ] Webhook rejects invalid configuration
- [ ] Returns clear error message explaining the issue
- [ ] Invalid configuration does not enter informer processing flow

## Functional Requirements

### FR-0: Hierarchical and Deterministic Auto-Injected Multus Resolution

**Requirement**: When the Pod webhook auto-injects `k8s.v1.cni.cncf.io/networks` from matched `SpiderMultusConfig` objects, it must first resolve the effective `cni.spidernet.io/network-resource-inject` value using a hierarchical `Pod`-then-`Namespace` lookup, and then produce a deterministic injected network list independent of Kubernetes API list order.

**Annotation Source Resolution Rules**:

| Source | Condition | Result |
|--------|-----------|--------|
| Pod annotation | Pod contains `cni.spidernet.io/network-resource-inject` | Use Pod annotation value and stop lookup |
| Namespace annotation | Pod annotation absent and Namespace contains `cni.spidernet.io/network-resource-inject` | Use Namespace annotation value as tenant default |
| None | Neither Pod nor Namespace contains the annotation | Do not inject networks based on this annotation |

**Precedence Rules**:

- Pod annotation has higher priority than Namespace annotation.
- Namespace annotation is only consulted when the Pod does not define `cni.spidernet.io/network-resource-inject`.
- Namespace reads during admission should use the controller cache via the existing manager abstraction, avoiding per-request direct APIServer reads.

**Ordering Rules**:

| CNI Type | Sort Key | Notes |
|----------|----------|-------|
| macvlan | First `master` interface name | Sort ascending by lexical order of the resolved master interface name |
| ipvlan | First `master` interface name | Sort ascending by lexical order of the resolved master interface name |
| sriov | `resourceName`/master affinity name used by the config | Must produce a deterministic ascending order aligned with the target NIC identity documented for SR-IOV auto injection |
| multi-master macvlan/ipvlan/vlan | `bond.name` | When multiple master interfaces are configured and a bond is used, sort by `bond.name` instead of the unordered master list |

**Tie-breakers**:

- If two `SpiderMultusConfig` objects resolve to the same primary sort key, use namespace/name lexical order as a stable secondary sort key.

- The sorting logic must not change the set of injected networks, only their order in the resulting Multus annotation string.

**Acceptance Criteria**:

- [ ] Pod webhook resolves `cni.spidernet.io/network-resource-inject` from Pod first, then Namespace
- [ ] Pod-level annotation overrides Namespace-level annotation when both exist
- [ ] Namespace lookup uses a cached read path instead of a direct live APIServer request for each admission
- [ ] Auto-injected Multus annotation order is deterministic across repeated webhook calls
- [ ] Sorting does not rely on Kubernetes list return order
- [ ] macvlan, ipvlan, and sriov auto-injection flows all follow the same deterministic ordering principle
- [ ] Multi-master configs sort by `bond.name` rather than raw `master` slice order
- [ ] Existing user-facing docs clearly describe the deterministic ordering behavior

### FR-1: CRD Extension for VLAN CNI Type

**Requirement**: `SpiderMultusConfig` must support `cniType: vlan`.

**Acceptance Criteria**:

- [ ] `CniType` enum includes `vlan` value
- [ ] `MultusCNIConfigSpec` contains `Vlan *SpiderVlanCniConfig` field

### FR-2: Complete VLAN Configuration Fields

**Requirement**: VLAN configuration block must support complete field set.

**Field List**:

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| master | []string | Yes | Parent NIC list, at least one |
| vlanID | int32 | Yes | VLAN ID, range 0-4094 |
| mtu | *int32 | No | MTU value, default determined by kernel |
| bond | *BondConfig | No | Bond configuration for multi-NIC |
| rdmaResourceName | *string | No | RDMA resource name for resource injection |
| ippools | *SpiderpoolPools | No | Default IPAM pool configuration |

**Acceptance Criteria**:

- [ ] All fields have correct OpenAPI validation annotations in CRD
- [ ] Kubernetes API rejects creation when required fields are missing

### FR-3: NAD Auto-Translation - Single NIC Scenario

**Requirement**: Single master configuration should directly generate VLAN CNI configuration.

**Acceptance Criteria**:

- [ ] Generated NAD configuration uses parent NIC as master in VLAN plugin
- [ ] vlanId field correctly passed
- [ ] IPAM configuration correctly injected (when IPAM not disabled)

**Expected CNI JSON**:

```json
{
  "type": "vlan",
  "master": "eth0",
  "vlanId": 100,
  "ipam": {
    "type": "spiderpool",
    "defaultIPv4IPPool": ["vlan100-pool"]
  }
}
```

### FR-4: NAD Auto-Translation - Multi-NIC Scenario

**Requirement**: Multi-master configuration should support bond pre-creation before using VLAN.

**Acceptance Criteria**:

- [ ] NAD plugin chain order is correct: ifacer (create bond) → vlan (create VLAN interface)
- [ ] VLAN CNI uses bond device name as master
- [ ] Inter-plugin configuration references are correct

**Expected CNI JSON**:

```json
[
  {
    "type": "ifacer",
    "interfaces": ["eth0", "eth1"],
    "bond": {
      "name": "bond0",
      "mode": 4
    }
  },
  {
    "type": "vlan",
    "master": "bond0",
    "vlanId": 200,
    "mtu": 1500,
    "ipam": {
      "type": "spiderpool"
    }
  }
]
```

### FR-5: Semantic Distinction from macvlan+VlanID

**Requirement**: VLAN CNI must correctly reflect native VLAN behavior, not macvlan nested VLAN behavior.

**Key Differences**:

| Feature | macvlan + vlanID | VLAN CNI |
|---------|------------------|----------|
| Creation Order | First create host VLAN subinterface, then create macvlan on it | Directly create VLAN subinterface for Pod |
| Interface Used by Pod | macvlan subinterface | VLAN subinterface itself |
| Applicable Scenarios | Scenarios requiring macvlan features | Pure VLAN isolation scenarios |

**Acceptance Criteria**:

- [ ] Documentation clearly explains differences between the two modes
- [ ] Code comments explain NAD generation logic differences
- [ ] Generated CNI JSON does not contain incorrect nesting relationships

### FR-6: Webhook Validation

**Requirement**: Invalid VLAN configurations should be rejected at webhook stage.

**Validation Items**:

| Validation Item | Description |
|-----------------|-------------|
| vlan Existence | Must provide `vlan` when `cniType: vlan` |
| vlanID Range | Must be within [0, 4094] range |
| master Non-Empty | Must provide at least one parent NIC |
| master/bond Consistency | Must provide bond configuration for multi-master |
| Configuration Conflict | Prohibit mixing with other CNI type configurations |
| RDMA Resource | Validate rdmaResourceName if resource injection annotation exists |

**Acceptance Criteria**:

- [ ] All validation items implemented in `multusconfig_validate.go`
- [ ] Validation failures return clear field.Error messages
- [ ] Boundary value test coverage

### FR-7: Documentation Updates

**Requirement**: Usage documentation must include VLAN CNI configuration instructions and examples.

**Documentation Scope**:

- [ ] `docs/usage/spider-multus-config.md` (English)
- [ ] `docs/usage/spider-multus-config-zh_CN.md` (Chinese)

**Documentation Content Requirements**:

- [ ] VLAN CNI supported fields description
- [ ] Single NIC configuration example
- [ ] Multi-NIC bond configuration example
- [ ] RDMA acceleration configuration example
- [ ] Differences explanation from macvlan+VlanID
- [ ] Common troubleshooting

## Success Criteria

### SC-1: Functional Completeness

| Metric | Target | Measurement Method |
|--------|--------|-------------------|
| VLAN CNI Configuration Success Rate | > 95% | E2E test pass rate |
| NAD Generation Success Rate | 100% | Single/Multi-master scenario tests |
| Webhook Invalid Config Interception | 100% | Boundary value test coverage |

### SC-2: User Experience

| Metric | Target | Measurement Method |
|--------|--------|-------------------|
| Configuration Examples Runnable | Yes | All example YAML can be directly applied |
| Error Messages Clear | Yes | Manual review of error messages |
| Consistent with Existing CNI | Yes | API design and documentation style unified |
| Namespace Tenant Default Injection Works | Yes | Repeated admissions with Namespace annotation and no Pod annotation yield stable injected result |
| Auto-injected Network Order Stable | Yes | Repeated webhook injection produces identical annotation order |

### SC-3: Code Quality

| Metric | Target | Measurement Method |
|--------|--------|-------------------|
| Unit Test Coverage | > 80% | New code line coverage |
| Consistent with Existing Code Style | Yes | Code Review |
| Backward Compatible | Yes | Existing CNI type tests do not fail |

## Key Entities

### AutoInjectionAnnotationResolution

Internal derived value used by pod webhook mutation to determine the effective `cni.spidernet.io/network-resource-inject` source before listing matching `SpiderMultusConfig` resources.

**Field Mapping**:

| Field | Type | Description |
|-------|------|-------------|
| Key | string | Annotation key being resolved, initially `cni.spidernet.io/network-resource-inject` |
| EffectiveValue | string | Final annotation value used to match `SpiderMultusConfig` labels |
| Source | string | One of `Pod`, `Namespace`, or `None` |
| NamespaceLookupMode | string | Expected to be `cache` for webhook-time namespace reads |

**Resolution Rules**:

- If Pod annotation exists, set `Source=Pod` and do not consult Namespace.
- If Pod annotation is absent and Namespace annotation exists, set `Source=Namespace`.
- If neither exists, set `Source=None` and skip network injection for this annotation path.
- Namespace lookup should be performed through the cached manager path.

### Auto-Injected Network Ordering Key

Internal derived value used by pod webhook mutation when constructing `k8s.v1.cni.cncf.io/networks` from matched `SpiderMultusConfig` resources.

**Field Mapping**:

| CNI Type | Primary Key Source | Secondary Key Source |
|----------|--------------------|----------------------|
| macvlan | `spec.macvlan.master[0]` or `spec.macvlan.bond.name` | `metadata.namespace + "/" + metadata.name` |
| ipvlan | `spec.ipvlan.master[0]` or `spec.ipvlan.bond.name` | `metadata.namespace + "/" + metadata.name` |
| sriov | `spec.sriov.resourceName` | `metadata.namespace + "/" + metadata.name` |
| vlan | `spec.vlan.master[0]` or `spec.vlan.bond.name` | `metadata.namespace + "/" + metadata.name` |

### SpiderVlanCniConfig (CRD Structure)

Core configuration entity containing all VLAN CNI configuration parameters.

**Field Mapping**:

| Spec Field | CRD Field Path | Type |
|------------|----------------|------|
| master | spec.vlan.master | []string |
| vlanID | spec.vlan.vlanID | int32 |
| mtu | spec.vlan.mtu | *int32 |
| bond | spec.vlan.bond | *BondConfig |
| rdmaResourceName | spec.vlan.rdmaResourceName | *string |
| ippools | spec.vlan.ippools | *SpiderpoolPools |

### VLAN CNI Runtime Model

**Single Master Flow**:

```
SpiderMultusConfig (master: ["eth0"], vlanID: 100)
    ↓
NAD Config: {type: "vlan", master: "eth0", vlanId: 100}
    ↓
Pod startup creates eth0.100 and moves into Pod netns
```

**Multi Master Flow**:

```
SpiderMultusConfig (master: ["eth0", "eth1"], vlanID: 200, bond: {...})
    ↓
NAD Config: [
    {type: "ifacer", interfaces: ["eth0", "eth1"], bond: {...}},
    {type: "vlan", master: "bond0", vlanId: 200}
]
    ↓
First create bond0, then create bond0.200 and move into Pod netns
```

## Assumptions

1. Target cluster has VLAN CNI plugin binary installed
2. Spiderpool controller and agent are deployed
3. Users are familiar with Kubernetes networking and Multus CNI basic concepts
4. Parent NICs (master) are properly configured on nodes and support VLAN
5. In the target multi-tenant model, one namespace can represent one tenant default VLAN selection

## Dependencies

- VLAN CNI plugin (available on nodes)
- Spiderpool IPAM system (existing)
- Multus CNI (existing)
- ifacer plugin (for multi-NIC bond scenarios, existing)

## Risks & Mitigations

| Risk | Impact | Mitigation |
|------|--------|------------|
| VLAN CNI Plugin Version Compatibility | High | Document supported version ranges |
| Confusion with macvlan VLAN Mode | Medium | Clearly distinguish in documentation, code comments |
| Namespace Annotation Resolution Adds Admission Complexity | Medium | Keep precedence simple (`Pod` first, `Namespace` fallback) and use cached namespace reads |
| Multi-NIC Bond Configuration Complexity | Medium | Provide complete examples, E2E test coverage |
| Regression of Existing CNI Types | High | Full regression tests, maintain backward compatibility |
