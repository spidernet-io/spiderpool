# Data Model: WorkloadEndpoint Query API

**Feature**: 002-workloadendpoint-query-api  
**Date**: 2026-04-27

## Entity Overview

```
┌─────────────────────┐     ┌─────────────────────────┐     ┌──────────────────┐
│   Pod (Kubernetes)  │────▶│   SpiderEndpoint CRD    │────▶│  API Response    │
│                     │     │                         │     │  (JSON)          │
└─────────────────────┘     └─────────────────────────┘     └──────────────────┘
                                      │
                                      │ contains
                                      ▼
                              ┌─────────────────────────┐
                              │   IPAllocationDetail    │
                              │   (per interface)       │
                              └─────────────────────────┘
```

## Entities

### SpiderEndpoint (Kubernetes CRD)

**Purpose**: Persist Pod IP allocation details

**Structure**:
```yaml
apiVersion: spiderpool.spidernet.io/v2beta1
kind: SpiderEndpoint
metadata:
  name: my-pod
  namespace: default
status:
  current:
    uid: "pod-uid-123"
    node: "node-1"
    ips:
      - interface: "eth0"
        ipv4: "10.244.1.10/24"
        ipv6: "fd00:10:244::10/64"
        mac: "aa:bb:cc:dd:ee:ff"      # NEW: optional
        vlan: 100                       # existing, optional
        ipv4Pool: "default-v4-pool"
        ipv6Pool: "default-v6-pool"
        ipv4Gateway: "10.244.1.1"
        ipv6Gateway: "fd00:10:244::1"
        routes:
          - dst: "0.0.0.0/0"
            gw: "10.244.1.1"
  ownerControllerType: "ReplicaSet"
  ownerControllerName: "my-app-xyz"
```

### IPAllocationDetail (CRD Sub-resource)

**Purpose**: Single interface allocation record

**Fields**:

| Field | Type | Required | Validation | Description |
|-------|------|----------|------------|-------------|
| interface | string | Yes | non-empty | NIC name (e.g., "eth0", "net1") |
| ipv4 | string | No | CIDR format | IPv4 address with prefix |
| ipv6 | string | No | CIDR format | IPv6 address with prefix |
| mac | string | No | `^([0-9A-Fa-f]{2}:){5}[0-9A-Fa-f]{2}$` | **NEW** MAC address |
| vlan | int64 | No | 0-4094 | VLAN ID |
| ipv4Pool | string | No | DNS label | Source IPv4 IPPool name |
| ipv6Pool | string | No | DNS label | Source IPv6 IPPool name |
| ipv4Gateway | string | No | IPv4 | IPv4 gateway address |
| ipv6Gateway | string | No | IPv6 | IPv6 gateway address |
| routes | []Route | No | - | Static routes |
| cleanGateway | bool | No | - | Clean gateway flag |

### WorkloadEndpointStatus (API Response)

**Purpose**: API response structure for GET /v1/workloadendpoint

**Fields**:

| Field | Type | Source | Description |
|-------|------|--------|-------------|
| podNamespace | string | query param | Pod namespace |
| podName | string | query param | Pod name |
| podUID | string | SpiderEndpoint.Status.Current.UID | Pod unique ID |
| node | string | SpiderEndpoint.Status.Current.Node | Node name |
| interfaces | []InterfaceDetail | SpiderEndpoint.Status.Current.IPs | Interface list |

### InterfaceDetail (API Response Sub-structure)

**Purpose**: Single interface in API response

**Fields**:

| Field | Type | Source | Required | Description |
|-------|------|--------|----------|-------------|
| interface | string | IPAllocationDetail.NIC | Yes | NIC name |
| ipv4 | string | IPAllocationDetail.IPv4 | No | IPv4 address |
| ipv6 | string | IPAllocationDetail.IPv6 | No | IPv6 address |
| mac | string | IPAllocationDetail.MAC | No | **NEW** MAC (omitted if unset) |
| vlan | int64 | IPAllocationDetail.Vlan | No | VLAN ID (omitted if unset/0) |
| ipv4Pool | string | IPAllocationDetail.IPv4Pool | No | Pool name |
| ipv6Pool | string | IPAllocationDetail.IPv6Pool | No | Pool name |
| ipv4Gateway | string | IPAllocationDetail.IPv4Gateway | No | Gateway |
| ipv6Gateway | string | IPAllocationDetail.IPv6Gateway | No | Gateway |
| routes | []Route | IPAllocationDetail.Routes | No | Routes |

## Field Visibility Rules

| Field | When Included | When Omitted |
|-------|---------------|--------------|
| mac | Present in SpiderEndpoint | Not provided during allocation |
| vlan | Present and non-zero | Not set or zero |
| ipv4/ipv6 | Always if allocated | N/A (should always have at least one) |

## State Transitions

N/A - This is a read-only query API. State changes come from IPAM allocation/deallocation.

## Validation Rules

1. **MAC Format**: Must match `^([0-9A-Fa-f]{2}:){5}[0-9A-Fa-f]{2}$`
2. **Query Parameters**: Both `podNamespace` and `podName` are required
3. **Existence**: Return 404 if SpiderEndpoint does not exist for given Pod

## OpenAPI Schema

See `contracts/openapi.yaml` for full API contract.
