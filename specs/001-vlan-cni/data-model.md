# Data Model: VLAN CNI Support

**Generated**: 2026-03-17  
**Feature**: VLAN CNI Support (specs/001-vlan-cni/spec.md)

## Entity Overview

```
┌─────────────────────────────────┐
│     SpiderMultusConfig          │
│     (Existing CRD)              │
├─────────────────────────────────┤
│  spec.cniType: "vlan"           │
│  spec.vlan: SpiderVlanCniConfig │
└──────────┬──────────────────────┘
           │
           │ generates
           ▼
┌─────────────────────────────────┐
│  NetworkAttachmentDefinition    │
│  (Managed by Informer)          │
├─────────────────────────────────┤
│  spec.config: CNI JSON          │
│    - vlan plugin config         │
│    - ipam config (spiderpool)   │
│    - ifacer plugin (if bond)    │
└─────────────────────────────────┘
```

## Entity: SpiderVlanCniConfig

**Purpose**: Configuration block for VLAN CNI within SpiderMultusConfig

**Fields**:

| Field | Type | Required | Validation | Description |
|-------|------|----------|------------|-------------|
| master | []string | Yes | len >= 1 | Parent network interface names |
| vlanID | int32 | Yes | 0 <= x <= 4094 | VLAN identifier |
| mtu | *int32 | No | >= 0 | Interface MTU, 0 = kernel default |
| bond | *BondConfig | Conditional | Required if len(master) >= 2 | Bond configuration for multi-NIC |
| rdmaResourceName | *string | No | Valid resource name format | RDMA device plugin resource name |
| ippools | *SpiderpoolPools | No | Valid pool references | Default IPAM pool configuration |

**Relationships**:
- Belongs to: `SpiderMultusConfig` (via `spec.vlan`)
- Uses: `BondConfig` (when multi-NIC)
- Uses: `SpiderpoolPools` (for IPAM defaults)

## Entity: VlanNetConf (Internal CNI Config Structure)

**Purpose**: Internal representation of VLAN CNI configuration for NAD generation

**Fields**:

| Field | Type | JSON Tag | Description |
|-------|------|----------|-------------|
| Type | string | `type` | Always "vlan" |
| Master | string | `master` | Parent interface (eth0 or bond0) |
| VlanID | int32 | `vlanId` | VLAN identifier |
| MTU | *int32 | `mtu,omitempty` | Interface MTU |
| IPAM | *IPAMConfig | `ipam,omitempty` | Spiderpool IPAM configuration |

**Validation Rules**:
- Master must not be empty
- VlanID must be in range [0, 4094]
- If IPAM is present, type must be "spiderpool"

## Entity: IfacerNetConf (For Multi-NIC Bond)

**Purpose**: Preparatory plugin configuration for bond creation

**Fields**:

| Field | Type | JSON Tag | Description |
|-------|------|----------|-------------|
| Type | string | `type` | Always "ifacer" |
| Interfaces | []string | `interfaces` | List of physical interfaces |
| Bond | *BondConfig | `bond,omitempty` | Bond device configuration |
| VlanID | int | `vlanId,omitempty` | Not used for pure bond creation |

**Usage Context**:
- Only generated when `len(master) >= 2`
- Plugin appears BEFORE vlan plugin in NAD plugin chain
- Creates bond device that vlan plugin uses as master

## Entity: BondConfig

**Purpose**: Configuration for bond device creation

**Fields**:

| Field | Type | Required | Validation | Description |
|-------|------|----------|------------|-------------|
| Name | string | Yes | Non-empty | Bond device name (e.g., "bond0") |
| Mode | int32 | Yes | 0 <= x <= 6 | Bonding mode (0=balance-rr, 1=active-backup, 4=802.3ad) |
| Options | *string | No | Valid bonding options | Additional bonding parameters |

**Validation Rules**:
- Name must be unique and valid Linux interface name
- Mode must be valid Linux bonding mode
- Required when len(SpiderVlanCniConfig.master) >= 2

## State Transitions: SpiderMultusConfig

```
┌─────────────┐    Create/Update    ┌─────────────┐
│   Draft     │ ─────────────────▶│  Validated  │
│  (User)     │                     │  (Webhook)  │
└─────────────┘                     └──────┬──────┘
                                           │
                                           │ Informer Sync
                                           ▼
                                    ┌─────────────┐
                                    │    NAD      │
                                    │   Created   │
                                    │  (Informer) │
                                    └─────────────┘
```

**States**:

1. **Draft**: User creates/updates SpiderMultusConfig
   - Validation: Kubernetes API + OpenAPI schema validation
   - Exit: Webhook validation

2. **Validated**: Webhook accepts configuration
   - Validation: Field-level validation, cross-field consistency
   - Exit: Informer reconciliation

3. **NAD Created**: NetworkAttachmentDefinition generated
   - Validation: CNI JSON validity
   - NAD spec.config contains complete plugin chain

## Validation Rules Summary

### Field-Level Validation (Kubebuilder + Webhook)

| Entity.Field | Rule | Error Message |
|--------------|------|---------------|
| SpiderVlanCniConfig.master | Required, len >= 1 | "master cannot be empty" |
| SpiderVlanCniConfig.vlanID | 0 <= x <= 4094 | "vlanID must be in range [0, 4094]" |
| SpiderVlanCniConfig.mtu | >= 0 if specified | "MTU must be >= 0" |
| SpiderVlanCniConfig.bond | Required if len(master) >= 2 | "bond config required for multiple masters" |
| BondConfig.name | Non-empty | "bond name cannot be empty" |
| BondConfig.mode | 0 <= x <= 6 | "bond mode must be in range [0, 6]" |

### Cross-Field Validation (Webhook Only)

| Condition | Rule | Error Message |
|-----------|------|---------------|
| cniType=vlan | vlan must be present | "vlan required when cniType is vlan" |
| Multi-master | bond must be non-nil | "bond configuration required when using multiple masters" |
| Mixed CNI types | Only one CNI config block allowed | "cannot mix vlan config with other CNI types" |
| RDMA resource | if inject annotation present, validate rdmaResourceName | "rdmaResourceName required when resource injection enabled" |

## Data Flow: Config to NAD

```
SpiderMultusConfig
    ├── spec.cniType = "vlan"
    └── spec.vlan
        ├── master: ["eth0"]
        ├── vlanID: 100
        ├── mtu: 1500 (optional)
        └── ippools: {ipv4: ["pool1"]}
                │
                ▼
        ┌─────────────────────────────────┐
        │   generateVlanCNIConf()         │
        │   (multusconfig_informer.go)    │
        └──────────┬──────────────────────┘
                   │
                   ▼
        ┌─────────────────────────────────┐
        │   VlanNetConf JSON:             │
        │   {                             │
        │     "type": "vlan",             │
        │     "master": "eth0",           │
        │     "vlanId": 100,              │
        │     "mtu": 1500,                │
        │     "ipam": {                   │
        │       "type": "spiderpool",       │
        │       "defaultIPv4IPPool":        │
        │         ["pool1"]               │
        │     }                             │
        │   }                             │
        └──────────┬──────────────────────┘
                   │
                   ▼
        ┌─────────────────────────────────┐
        │   NetworkAttachmentDefinition   │
        │   spec.config                   │
        └─────────────────────────────────┘
```

## Multi-NIC Data Flow (with Bond)

```
SpiderMultusConfig
    ├── spec.cniType = "vlan"
    └── spec.vlan
        ├── master: ["eth0", "eth1"]
        ├── vlanID: 200
        ├── mtu: 1500
        ├── bond: {name: "bond0", mode: 4}
        └── ippools: {ipv4: ["pool2"]}
                │
                ▼
        ┌─────────────────────────────────┐
        │   generateNetAttachDef()        │
        │   Plugin chain:                 │
        │   1. ifacer (create bond)       │
        │   2. vlan (VLAN on bond)        │
        └──────────┬──────────────────────┘
                   │
                   ▼
        ┌─────────────────────────────────┐
        │   CNI Plugin Chain JSON:        │
        │   [                             │
        │     {                           │
        │       "type": "ifacer",         │
        │       "interfaces":             │
        │         ["eth0","eth1"],        │
        │       "bond": {                 │
        │         "name": "bond0",        │
        │         "mode": 4               │
        │       }                         │
        │     },                          │
        │     {                           │
        │       "type": "vlan",           │
        │       "master": "bond0",       │
        │       "vlanId": 200,            │
        │       "mtu": 1500,              │
        │       "ipam": {...}            │
        │     }                           │
        │   ]                             │
        └─────────────────────────────────┘
```

## No State Persistence Required

VLAN CNI support does not introduce new persistent state beyond existing SpiderMultusConfig → NAD flow. The feature operates entirely through:

1. CRD schema extension (SpiderMultusConfig)
2. Validation-time checks (Webhook)
3. Translation-time generation (Informer)
4. Runtime CNI execution (VLAN CNI plugin)

No new CRDs, no stateful controllers, no custom resource lifecycle management required.
