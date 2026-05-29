# Specification: IaaS Network Provider Integration

**Branch Name**: `003-iaas-provider-integration`  
**Short Name**: `iaas-provider-integration`  
**Feature Number**: `003`

---

## 1. User Story

**As a** cluster administrator using Spiderpool with a third-party cloud provider  
**I want** Spiderpool to integrate with the cloud provider's IaaS IP management API  
**So that** IP allocations in Spiderpool are synchronized with the cloud provider's network infrastructure, ensuring proper MAC address assignment and VLAN configuration for Pods

---

## 2. User Scenarios

### Scenario 1: Pod IP Allocation with IaaS Integration

**Given**: A Pod is being created with Spiderpool IPAM  
**And**: The `iaas-network-provider` URL is configured in Spiderpool Agent  
**When**: Spiderpool Agent allocates IPs for the Pod  
**Then**: 
- Spiderpool Agent calls the IaaS provider's `/v1/iaas.network.io/ipam/allocate-ips` API
- The request includes node name, IP addresses, subnets, and parent NIC MAC
- The IaaS provider returns MAC addresses and VLAN IDs for the allocated IPs
- Spiderpool Agent stores the returned MAC addresses in SpiderEndpoint
- The Pod receives the complete network configuration (IP, MAC, VLAN)

### Scenario 2: Pod IP Release with IaaS Integration

**Given**: A Pod is being deleted or its IPs are being released  
**And**: The `iaas-network-provider` URL is configured  
**When**: Spiderpool Agent releases IPs for the Pod  
**Then**:
- Spiderpool Agent calls the IaaS provider's `/v1/iaas.network.io/ipam/release-ip` API
- The IaaS provider releases the corresponding IaaS resources
- The IP release in Spiderpool is completed successfully

### Scenario 3: IP Garbage Collection with IaaS Integration

**Given**: The Spiderpool Controller is performing IP garbage collection  
**And**: Orphaned IPs are identified for Pods that no longer exist  
**And**: The `iaas-network-provider` URL is configured in Spiderpool Controller  
**When**: Spiderpool Controller releases the orphaned IPs  
**Then**:
- Spiderpool Controller calls the IaaS provider's release API for each orphaned IP
- The IaaS resources are properly cleaned up
- The IPs are released from Spiderpool

### Scenario 4: Disabled IaaS Integration

**Given**: The `iaas-network-provider` URL is empty or not configured  
**When**: Spiderpool performs IP allocation or release  
**Then**: Spiderpool operates normally without calling any IaaS provider API

---

## 3. Functional Requirements

### FR1: Configuration

| ID | Requirement | Priority |
|----|-------------|----------|
| FR1.1 | Spiderpool Helm values must support `iaasNetworkProvider.url` configuration | Must |
| FR1.2 | The URL format should be `host:port` (e.g., `iaas-network-provider:444`) | Must |
| FR1.3 | Both Spiderpool Agent and Controller must read the IaaS provider URL from configuration | Must |
| FR1.4 | If the URL is empty, IaaS integration is disabled | Must |

### FR2: IaaS API Client (Phase 2 - Future)

| ID | Requirement | Priority |
|----|-------------|----------|
| FR2.1 | Implement an HTTP client for communicating with the IaaS provider | Must |
| FR2.2 | The client must support the allocate API: `POST /v1/iaas.network.io/ipam/allocate-ips` | Must |
| FR2.3 | The client must support the release API: `POST /v1/iaas.network.io/ipam/release-ip` | Must |
| FR2.4 | The client must have configurable timeout and retry logic | Should |
| FR2.5 | The client must handle API errors gracefully with proper logging | Must |
| FR2.6 | The client must support mTLS using mounted certificates | Must |

### FR3: Configuration Infrastructure (Phase 1)

| ID | Requirement | Priority |
|----|-------------|----------|
| FR3.1 | Helm chart supports `iaasNetworkProvider.url` configuration | Must |
| FR3.2 | Helm chart supports `iaasNetworkProvider.tlsSecret.name` configuration | Must |
| FR3.3 | Helm chart supports `iaasNetworkProvider.tlsSecret.namespace` configuration | Must |
| FR3.4 | Secret containing `tls.crt` and `tls.key` is mounted into Agent pod | Must |
| FR3.5 | Secret containing `tls.crt` and `tls.key` is mounted into Controller pod | Must |
| FR3.6 | Configuration is passed to Agent via environment variables | Must |
| FR3.7 | Configuration is passed to Controller via environment variables | Must |
| FR3.8 | Agent validates secret existence at startup (when URL is configured) | Should |
| FR3.9 | Controller validates secret existence at startup (when URL is configured) | Should |

### FR4: IP Allocation Flow (Phase 2 - Future)

| ID | Requirement | Priority |
|----|-------------|----------|
| FR4.1 | After Spiderpool allocates IPs, if IaaS provider is configured, call the allocate API | Must |
| FR4.2 | The allocate request must include: `nodeName` (required), `iaasIPsAllocationRequest` array (required) | Must |
| FR4.3 | Each allocation request item must include: `ipAddress`, `subnet`, `parentNicMac` (all required) | Must |
| FR4.4 | Optional fields may include: `podName`, `podNamespace`, `podUID` | Should |
| FR4.5 | On successful response, extract `macAddress` and `vlanId` from the response | Must |
| FR4.6 | Store the returned `macAddress` in SpiderEndpoint's `IPAllocationDetail.MAC` field | Must |
| FR4.7 | Store the returned `vlanId` in SpiderEndpoint's `IPAllocationDetail.Vlan` field | Must |
| FR4.8 | If the IaaS API call fails, the IP allocation should fail with appropriate error | Must |

### FR5: IP Release Flow (Phase 2 - Future)

| ID | Requirement | Priority |
|----|-------------|----------|
| FR5.1 | Before or during Spiderpool IP release, if IaaS provider is configured, call the release API | Must |
| FR5.2 | The release request must include information to identify the IPs being released | Must |
| FR5.3 | The release flow should proceed even if IaaS API call fails (with logging) | Should |
| FR5.4 | IP Garbage Collection in Controller must also trigger IaaS release calls | Must |

### FR6: Error Handling and Observability (Phase 2 - Future)

| ID | Requirement | Priority |
|----|-------------|----------|
| FR6.1 | All IaaS API calls must be logged with context (Pod info, IPs, result) | Must |
| FR6.2 | Metrics should track IaaS API call latency and success/failure rates | Should |
| FR6.3 | Clear error messages when IaaS integration fails | Must |
| FR6.4 | Support for configuring API timeouts to avoid blocking IPAM operations | Should |

---

## 4. Data Model

### IaaS IP Allocation Request

```json
{
  "podName": "p1",
  "podNamespace": "ns1",
  "podUID": "1234567890",
  "nodeName": "worker-01",
  "iaasIPsAllocationRequest": [
    {
      "ipAddress": "10.0.0.10",
      "subnet": "10.0.0.0/24",
      "parentNicMac": "fa:16:3e:xx:xx:xx"
    }
  ]
}
```

### IaaS IP Allocation Response

```json
{
  "podName": "p1",
  "podNamespace": "ns1",
  "nodeName": "worker-01",
  "iaasIPsAllocationResponse": [
    {
      "parentNicMac": "fa:16:3e:xx:xx:xx",
      "subnet": "10.251.0.0/24",
      "ipAddress": "10.0.0.10",
      "macAddress": "fa:16:3e:xx:xx:xx",
      "vlanId": 100
    }
  ]
}
```

### Configuration Schema

```yaml
iaasNetworkProvider:
  # URL of the IaaS provider service (host:port)
  # If empty, IaaS integration is disabled
  url: "iaas-network-provider:444"
  
  # TLS certificate configuration for mTLS authentication
  # Secret must exist and contain tls.crt and tls.key
  tlsSecret:
    name: "iaas-provider-client-cert"  # Kubernetes secret name
    namespace: "spiderpool"            # Secret namespace
```

### Phase 1: Configuration and Secret Mounting (Current)

This phase implements the Helm configuration and Kubernetes secret mounting infrastructure:

1. **Helm Values**: Support `iaasNetworkProvider` configuration block
2. **Secret Mounting**: Mount specified secrets as volumes into Agent and Controller pods
3. **Config Propagation**: Pass configuration to Agent and Controller via environment variables or config files
4. **Validation**: Validate secret existence and format at startup

**Note**: The actual IaaS API client implementation and calling logic will be implemented in Phase 2.

### Phase 2: API Implementation (Future)

This phase will implement the actual IaaS API integration:

1. **API Client**: HTTP client with mTLS support
2. **Allocation Hook**: Call IaaS allocate API after Spiderpool IP allocation
3. **Release Hook**: Call IaaS release API during IP release
4. **MAC Storage**: Store returned MAC addresses in SpiderEndpoint

---

## 5. Success Criteria

| ID | Criterion | Measurement |
|----|-----------|-------------|
| SC1 | IP allocation with IaaS integration completes within 5 seconds (including IaaS API call) | 95th percentile latency < 5s |
| SC2 | IaaS-allocated MAC addresses are correctly stored and retrievable via WorkloadEndpoint API | 100% of allocations have MAC stored |
| SC3 | IP release properly triggers IaaS cleanup | Zero orphaned IaaS resources after Pod deletion |
| SC4 | When IaaS provider is disabled, Spiderpool operates with no performance impact | Latency difference < 10ms vs enabled |
| SC5 | IaaS integration is transparent to CNI plugins | No changes required in CNI configuration |

### Phase 1 Success Criteria

| ID | Criterion | Measurement |
|----|-----------|-------------|
| SC-P1-1 | Helm values support complete IaaS provider configuration | All config options renderable via Helm |
| SC-P1-2 | Secrets are mounted into Agent and Controller pods | Secret files accessible at configured paths |
| SC-P1-3 | Configuration is passed to components | Environment variables or config files populated |
| SC-P1-4 | Components start successfully with configuration | No startup errors when IaaS config is provided |

---

## 6. Assumptions and Dependencies

### Assumptions

- The IaaS provider API is available and responsive
- The IaaS provider returns valid MAC addresses in standard format
- The `parentNicMac` is obtained by parsing Pod's Multus annotation to identify SpiderMultusConfig
- For VLAN CNI type, the master NIC's MAC address is retrieved via netlink
- Network connectivity exists between Spiderpool Agent/Controller and the IaaS provider

### Dependencies

- Existing WorkloadEndpoint Query API implementation (for MAC storage/retrieval)
- SpiderEndpoint CRD with MAC field (already implemented)
- IPAM allocation/deallocation hooks in Spiderpool Agent
- IP GC mechanism in Spiderpool Controller

---

## 7. Risks and Mitigations

| Risk | Impact | Mitigation |
|------|--------|------------|
| IaaS provider API unavailable | High - blocks IP allocation | Implement circuit breaker, configurable timeout, option to disable integration |
| IaaS provider returns invalid data | Medium - incorrect network config | Validate response format, implement retry with backoff |
| Increased IP allocation latency | Medium - slower Pod startup | Configure appropriate timeouts, async processing if possible |
| Security concerns with external API | Medium - potential data exposure | Support mTLS, API authentication tokens via Kubernetes secrets |

---

## 8. Open Questions

- [RESOLVED: 1] Should the IaaS release API be called before or after Spiderpool releases the IP from its pool?
  - **Answer**: Option B - Release Spiderpool IP first, then call IaaS API. This prioritizes speed; IaaS cleanup failures are logged but don't block Spiderpool release.
- [RESOLVED: 2] What authentication mechanism is required for the IaaS provider API?
  - **Answer**: mTLS with client certificates. The certificate and key will be mounted from Kubernetes secrets via Helm configuration.
- [RESOLVED: 3] How should the `parentNicMac` be determined?
  - **Answer**: Parse Pod's Multus annotation to identify SpiderMultusConfig instance. If CNI type is VLAN, get master NIC name from config and retrieve MAC via netlink.

---

## 9. Implementation Notes

### Parent NIC MAC Discovery

The `parentNicMac` parameter in the IaaS allocate request is determined as follows:

1. **Parse Multus Annotation**: Read the Pod's `k8s.v1.cni.cncf.io/networks` annotation to identify network attachments
2. **Lookup SpiderMultusConfig**: Match the network name to a SpiderMultusConfig CRD instance
3. **Check CNI Type**: If the SpiderMultusConfig uses VLAN CNI type:
   - Extract `master` NIC name from the CNI configuration
   - Use netlink to retrieve the MAC address of the master interface
4. **Reuse Existing Code**: Leverage existing Multus annotation parsing code from the IPAM allocation path

This approach ensures consistency with Spiderpool's existing network configuration handling.

---

## 10. Appendix

### API Endpoint Details

**Allocate API:**
- Method: POST
- Path: `/v1/iaas.network.io/ipam/allocate-ips`
- Content-Type: application/json

**Release API:**
- Method: POST
- Path: `/v1/iaas.network.io/ipam/release-ip`
- Content-Type: application/json

### Related Features

- 002-workloadendpoint-query-api: Provides MAC retrieval capability
- SpiderEndpoint CRD with MAC field: Storage for IaaS-returned MAC addresses
