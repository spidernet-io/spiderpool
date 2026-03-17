# Feature Specification: VLAN CNI Support in Spiderpool

**Branch**: `001-vlan-cni` | **Created**: 2026-03-17 | **Status**: Draft  
**Spec File**: `specs/001-vlan-cni/spec.md`

## Overview

Add native VLAN CNI support to Spiderpool, allowing users to configure VLAN networks through `SpiderMultusConfig`, automatically translate to `NetworkAttachmentDefinition`, and validate via webhook.

### Feature Description

Spiderpool currently supports multiple CNI types (macvlan, ipvlan, sriov, etc.), but lacks native VLAN CNI support. This feature will:

1. Extend `SpiderMultusConfig` CRD with `cniType: vlan` type
2. Add `Vlan` configuration block supporting `master`, `vlanID`, `ippools`, `rdmaResourceName` fields
3. Implement automatic translation from VLAN configuration to NAD in the multusconfig informer
4. Add VLAN configuration validation logic in the webhook
5. Update usage documentation with VLAN CNI configuration examples

### Primary Goals

- Users can define VLAN CNI network configuration through `SpiderMultusConfig`
- VLAN configuration automatically translates to valid `NetworkAttachmentDefinition` CNI JSON
- Invalid VLAN configurations are rejected at the webhook stage
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

### Scenario 4: Configuration Validation Failure

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

### SC-3: Code Quality

| Metric | Target | Measurement Method |
|--------|--------|-------------------|
| Unit Test Coverage | > 80% | New code line coverage |
| Consistent with Existing Code Style | Yes | Code Review |
| Backward Compatible | Yes | Existing CNI type tests do not fail |

## Key Entities

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
| Multi-NIC Bond Configuration Complexity | Medium | Provide complete examples, E2E test coverage |
| Regression of Existing CNI Types | High | Full regression tests, maintain backward compatibility |
