# Feature Specification: WorkloadEndpoint Query API with MAC Address

**Feature Branch**: `002-we-api-mac-vlan`  
**Created**: 2026-04-27  
**Status**: Draft  
**Input**: User description: "Implement GetWorkloadendpoint API for external Unix Socket access to query Pod IP allocation details (IP/VLAN/MAC) from SpiderEndpoint, applicable for public cloud scenarios where MAC/VLAN fields are conditionally populated"

## Overview

Extend the Spiderpool Agent WorkloadEndpoint query API to support retrieving complete network allocation details (IP / VLAN / MAC) via Unix Socket. This API provides a standard interface for external systems to query Pod network configuration, enabling scenarios such as CNI plugins retrieving IP allocation results (including VLAN and MAC) after IPAM delegates to external cloud providers.

### Feature Description

The current `SpiderEndpoint` CRD records Pod IP allocation details including interface names, IPv4/IPv6 addresses, VLAN IDs, and IPPool sources. However, it lacks MAC address information, and the existing `/workloadendpoint` API endpoint lacks a complete response schema definition, making it difficult for external systems (such as cloud provider network components) to query directly.

This feature will:

1. Extend the `IPAllocationDetail` structure with a `mac` field to record interface MAC addresses
2. Complete the `/workloadendpoint` GET API with request parameters and response schema
3. Support direct Unix Socket access for querying Pod network allocation status
4. Maintain backward compatibility without affecting existing IPAM allocation flows

**API Design**: The `mac` and `vlan` fields are optional. When these values are not available (e.g., standard on-premise deployments without MAC assignment), the fields are omitted from API responses. In scenarios like public cloud integration where IPAM delegates to external providers returning MAC/VLAN, these fields are populated and included in responses.

### Primary Goals

- External systems can query complete Pod network allocation details via Unix Socket
- API returns for each interface: IP address, and optionally VLAN ID and MAC address when available
- Support precise querying by Pod Namespace + Pod Name
- Clean response format suitable for various integration scenarios including public cloud

### Out of Scope

- Modifying existing IPAM allocation logic for standard scenarios
- Adding new persistent storage or controllers
- Webhook or admission control changes
- Mandating MAC/VLAN collection for all deployment modes

## User Scenarios & Testing

### User Story 1 - Query Pod Network Allocation Details (Priority: P1)

External systems need to query complete Pod network configuration including IP addresses, VLAN IDs, and MAC addresses via Unix Socket API.

**Why this priority**: This is the core functionality enabling external CNI plugins and cloud provider integrations to retrieve allocation details needed for network interface configuration.

**Independent Test**: Can be tested by querying the API for an existing Pod and verifying the response contains expected interface details with IP, optional VLAN, and optional MAC fields.

**Acceptance Scenarios**:

1. **Given** a Pod exists with IP allocation recorded in SpiderEndpoint, **When** external system queries `/v1/workloadendpoint?podNamespace=X&podName=Y`, **Then** response contains complete interface details including IP, VLAN (if set), and MAC (if set)
2. **Given** a Pod does not exist, **When** API is queried, **Then** return 404 with clear error message
3. **Given** query parameters are missing, **When** API is called, **Then** return 400 with validation error

---

### User Story 2 - MAC Address Recording for External Integration (Priority: P2)

When IPAM delegates allocation to external systems (e.g., public cloud providers) that return MAC addresses, these should be recorded and made available through the API.

**Why this priority**: Critical for public cloud integration scenarios where the cloud provider assigns MAC addresses during IP allocation, and the local CNI needs to retrieve this information.

**Independent Test**: Can be tested by simulating an IPAM request with MAC provided, then verifying the MAC appears in subsequent API queries.

**Acceptance Scenarios**:

1. **Given** IPAM receives allocation request with MAC in parameters, **When** SpiderEndpoint is created/updated, **Then** MAC field is recorded in IPAllocationDetail
2. **Given** MAC was recorded for a Pod interface, **When** API is queried, **Then** response includes the MAC address for that interface
3. **Given** no MAC was provided during allocation, **When** API is queried, **Then** MAC field is omitted from response (not null/empty)

---

### User Story 3 - Backward Compatibility (Priority: P3)

Existing systems using the `/workloadendpoint` API must continue to function without modification when new fields are added.

**Why this priority**: Ensures smooth migration and no disruption to existing Spiderflat and other components that depend on this API.

**Independent Test**: Can be tested by verifying existing API consumers parse responses correctly without changes.

**Acceptance Scenarios**:

1. **Given** existing client queries API, **When** response includes new optional fields, **Then** client parses successfully ignoring unknown fields
2. **Given** legacy Spiderflat component, **When** it queries workloadendpoint, **Then** it continues to function normally

---

### Edge Cases

- What happens when MAC address format is invalid? System should validate and reject or normalize
- How does system handle concurrent queries for the same Pod? Should handle gracefully with proper locking or caching
- What happens when SpiderEndpoint CRD exists but IP allocation is incomplete? API should return partial data or appropriate error
- How does system handle very large numbers of interfaces per Pod? API should handle and return all interfaces efficiently

## Requirements

### Functional Requirements

- **FR-001**: System MUST extend `IPAllocationDetail` structure with optional `mac` field to record interface MAC addresses when provided
- **FR-002**: System MUST accept MAC address in IPAM request parameters and record it to SpiderEndpoint when present
- **FR-003**: System MUST complete `/v1/workloadendpoint` GET API with proper request parameters (`podNamespace`, `podName`) and response schema
- **FR-004**: System MUST expose API via Unix Socket for external system access with configurable socket path
- **FR-005**: System MUST return complete Pod network allocation details including IP, and optionally VLAN and MAC when available
- **FR-006**: System MUST omit optional fields (`mac`, `vlan`) from API responses when not populated, rather than returning null or empty values
- **FR-007**: System MUST maintain backward compatibility with existing API consumers by ensuring new fields are optional and unknown fields are ignored
- **FR-008**: System MUST validate MAC address format when provided, rejecting or normalizing invalid formats

### Key Entities

- **SpiderEndpoint**: Kubernetes CRD storing Pod IP allocation details including interfaces, IPs, pools, VLAN, and MAC
- **IPAllocationDetail**: Structure within SpiderEndpoint recording details for a single network interface including NIC name, IPs, gateway, routes, VLAN ID, and MAC address
- **WorkloadEndpointStatus**: API response structure containing Pod network allocation summary including namespace, name, UID, node, and interface details
- **InterfaceDetail**: Sub-entity within WorkloadEndpointStatus representing a single network interface with its configuration

## Success Criteria

### Measurable Outcomes

- **SC-001**: External systems can query Pod network allocation via Unix Socket API with latency under 10 milliseconds per request
- **SC-002**: MAC addresses provided by external sources are recorded successfully in over 95% of allocation operations
- **SC-003**: API responses correctly omit optional fields (MAC, VLAN) when not populated, verified by automated tests
- **SC-004**: Existing API consumers (Spiderflat, etc.) continue to function without modification when new fields are added
- **SC-005**: API handles at least 100 concurrent queries per second without performance degradation

## Assumptions

1. Calling external systems have permission to access the Unix Socket file
2. CNI plugins can obtain interface MAC addresses before calling IPAM
3. For SR-IOV scenarios, VF MAC addresses can be correctly read
4. MAC address changes are infrequent (no real-time synchronization required)
5. Deployments can choose to integrate with external systems that provide MAC/VLAN data

## Dependencies

- Spiderpool Agent HTTP server (existing, generated via go-swagger)
- SpiderEndpoint CRD (existing, requires adding mac field)
- Unix Socket communication mechanism (existing)

## Risks & Mitigations

| Risk | Impact | Mitigation |
|------|--------|------------|
| CNI plugin cannot obtain MAC (interface not yet created) | Medium | Allow MAC to be empty; coordinator phase can update later |
| MAC address format inconsistency (different CNI plugins) | Low | Use netlink for unified reading, standardize format |
| Unix Socket permission issues | Medium | Document required permissions, provide configuration options |
| Backward compatibility issues | High | New fields are optional and omitted when not set; thoroughly test old clients |
| Performance impact when MAC not used | Low | Skip MAC handling when not provided; no extra overhead for standard flows |
