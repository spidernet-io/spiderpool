# Tasks: IaaS Network Provider Integration - Phase 2

**Feature**: IaaS Network Provider Integration - Phase 2 (API Implementation)  
**Branch**: `003-iaas-provider-integration`  
**Created**: 2025-04-27  
**Depends On**: Phase 1 (Configuration and Secret Mounting)

---

## Overview

This task list implements Phase 2 of the IaaS Network Provider Integration: API Client and IPAM Hooks. This phase integrates the actual IaaS API calls into Spiderpool's IP allocation and release flows.

**Total Tasks**: 12  
**Estimated Effort**: Medium (API client + IPAM integration)  
**Parallel Tasks**: T101-T102, T104-T105, T107-T108

---

## Phase 1: IaaS API Client

**Goal**: Create HTTP client with mTLS for IaaS provider communication  
**User Story**: FR2 (IaaS API Client)  
**Independent Test Criteria**: 
- HTTP client can make mTLS requests with test certificates
- Unit tests pass for request/response serialization

### Implementation Tasks

- [x] **T101** Create IaaS API client package structure
  - **File**: `pkg/iaas/client/client.go` (new package)
  - **Description**: Define IaaSClient interface and basic structure
  - **Code**:
    ```go
    type Client interface {
        AllocateIPs(ctx context.Context, req *AllocateIPRequest) (*AllocateIPResponse, error)
        ReleaseIPs(ctx context.Context, req *ReleaseIPRequest) error
    }
    
    type IaaSClient struct {
        baseURL    string
        httpClient *http.Client
        certPath   string
        keyPath    string
    }
    ```

- [x] **T102** [P] Implement mTLS HTTP client initialization
  - **File**: `pkg/iaas/client/client.go`
  - **Description**: Load certificates and create http.Client with TLS config
  - **Code**:
    ```go
    func NewClient(cfg *types.IaaSProviderConfig) (*IaaSClient, error) {
        // Load client certificate
        // Create TLS config
        // Initialize http.Client
    }
    ```

- [x] **T103** Implement request/response types
  - **File**: `pkg/iaas/client/types.go`
  - **Description**: Define Go structs matching API spec
  - **Structs**:
    ```go
    type AllocateIPRequest struct {
        PodName                 string                   `json:"podName,omitempty"`
        PodNamespace            string                   `json:"podNamespace,omitempty"`
        PodUID                  string                   `json:"podUID,omitempty"`
        NodeName                string                   `json:"nodeName"`
        IaaSIPsAllocationRequest []IaaSIPAllocationItem    `json:"iaasIPsAllocationRequest"`
    }
    
    type IaaSIPAllocationItem struct {
        IPAddress      string `json:"ipAddress"`
        Subnet         string `json:"subnet"`
        ParentNicMac   string `json:"parentNicMac"`
    }
    
    type AllocateIPResponse struct {
        PodName                  string                    `json:"podName"`
        PodNamespace             string                    `json:"podNamespace"`
        NodeName                 string                    `json:"nodeName"`
        IaaSIPsAllocationResponse []IaaSIPAllocationResult  `json:"iaasIPsAllocationResponse"`
    }
    
    type IaaSIPAllocationResult struct {
        ParentNicMac string `json:"parentNicMac"`
        Subnet       string `json:"subnet"`
        IPAddress    string `json:"ipAddress"`
        MacAddress   string `json:"macAddress"`
        VlanID       int64  `json:"vlanId"`
    }
    
    type ReleaseIPRequest struct {
        PodName      string `json:"podName,omitempty"`
        PodNamespace string `json:"podNamespace,omitempty"`
        PodUID       string `json:"podUID,omitempty"`
        NodeName     string `json:"nodeName"`
        IPAddresses  []string `json:"ipAddresses"`
    }
    ```

- [x] **T104** [P] Implement AllocateIPs API call
  - **File**: `pkg/iaas/client/allocate.go`
  - **Description**: POST /v1/iaas.network.io/ipam/allocate-ips
  - **Details**:
    - POST to `/v1/iaas.network.io/ipam/allocate-ips`
    - Marshal request body
    - Handle response
    - Return error on non-200 status
    - Timeout: 30 seconds

- [x] **T105** [P] Implement ReleaseIPs API call
  - **File**: `pkg/iaas/client/release.go`
  - **Description**: POST /v1/iaas.network.io/ipam/release-ip
  - **Details**:
    - POST to `/v1/iaas.network.io/ipam/release-ip`
    - Marshal request body
    - Return error on non-200 status
    - Timeout: 30 seconds

- [x] **T106** Add logging and observability
  - **Files**: `pkg/iaas/client/*.go`
  - **Description**: Add structured logging for all API calls
  - **Requirements**:
    - Log request with context (pod info)
    - Log response or error
    - Track latency metrics (optional for Phase 2)

---

## Phase 2: IPAM Integration

**Goal**: Integrate IaaS API calls into IPAM Allocate flow  
**User Story**: FR4 (IP Allocation Flow)  
**Independent Test Criteria**:
- IP allocation with IaaS configured calls the API
- MAC/VLAN returned from IaaS are stored in SpiderEndpoint

### Implementation Tasks

- [x] **T107** [P] Get parent NIC MAC from Multus annotation
  - **File**: `pkg/iaas/utils/multus.go`
  - **Description**: Parse Pod annotation to get parent NIC MAC
  - **Function**:
    ```go
    func GetParentNicMac(ctx context.Context, pod *corev1.Pod, ifName string) (string, error)
    ```
  - **Algorithm**:
    1. Read `k8s.v1.cni.cncf.io/networks` annotation
    2. Parse to get SpiderMultusConfig name
    3. Check if CNI type is vlan
    4. Get master interface name
    5. Use netlink to get MAC address

- [x] **T108** [P] Integrate IaaS allocate into IPAM
  - **File**: `pkg/ipam/allocate.go` (existing or new file)
  - **Description**: After Spiderpool IP allocation, call IaaS API
  - **Integration Point**: In `Allocate()` method, after IP allocation, before returning
  - **Code Flow**:
    ```go
    // After Spiderpool allocates IPs
    if iaasClient != nil {
        // Build IaaS request
        req := buildIaaSAllocateRequest(pod, results, parentNicMac)
        
        // Call IaaS API
        resp, err := iaasClient.AllocateIPs(ctx, req)
        if err != nil {
            return nil, fmt.Errorf("iaas allocation failed: %w", err)
        }
        
        // Merge IaaS response into allocation results
        mergeIaaSResponse(results, resp)
    }
    ```

- [x] **T109** Store MAC/VLAN in SpiderEndpoint (already in Phase 1)
  - **File**: `pkg/utils/convert/convert.go` (existing) or IPAM
  - **Description**: Store returned MAC and VLAN in IPAllocationDetail
  - **Already Done**: MAC field already in IPAllocationDetail from previous PR
  - **Task**: Ensure IaaS response MAC/VLAN are passed to conversion function

---

## Phase 3: IP Release Integration

**Goal**: Integrate IaaS API calls into IPAM Release flow  
**User Story**: FR5 (IP Release Flow)  
**Independent Test Criteria**:
- IP release calls IaaS API
- Release proceeds even if IaaS call fails

### Implementation Tasks

- [x] **T110** Integrate IaaS release into IPAM Release
  - **File**: `pkg/ipam/release.go` (existing)
  - **Description**: Before or during Spiderpool release, call IaaS API
  - **Requirements**:
    - Call IaaS release API with IP addresses being released
    - Log IaaS API result
    - Continue release even if IaaS call fails (fail-open)

---

## Phase 4: Controller GC Integration

**Goal**: Integrate IaaS API calls into IP Garbage Collection  
**User Story**: FR5.4 (GC Integration)  
**Independent Test Criteria**:
- GC triggers IaaS release calls
- GC proceeds even if IaaS call fails

### Implementation Tasks

- [ ] **T111** Integrate IaaS release into Controller GC
  - **File**: Controller GC code (find location)
  - **Description**: When GC releases IPs, call IaaS API
  - **Requirements**:
    - Similar to IPAM Release integration
    - Call IaaS before releasing IPs
    - Continue GC even if IaaS call fails

---

## Phase 5: Agent Initialization

**Goal**: Wire IaaS client into Agent  
**User Story**: Infrastructure

### Implementation Tasks

- [x] **T112** Initialize IaaS client in Agent
  - **File**: `cmd/spiderpool-agent/cmd/daemon.go`
  - **Description**: Create IaaS client if configured
  - **Code**:
    ```go
    // After config validation
    if agentContext.Cfg.IaaSProviderConfig.URL != "" {
        iaasClient, err := iaasclient.NewClient(&agentContext.Cfg.IaaSProviderConfig)
        if err != nil {
            logger.Sugar().Warnf("Failed to create IaaS client: %v", err)
        } else {
            agentContext.IaaSClient = iaasClient
        }
    }
    ```

---

## Dependency Graph

```
T101 (Package structure)
    ├── T102 (mTLS client)
    └── T103 (Types)
        ├── T104 (Allocate API)
        └── T105 (Release API)
        
T104 + T105 → T106 (Logging)

T103 + T106 → T107 (Multus MAC) + T108 (IPAM Allocate)
T108 → T109 (MAC/VLAN storage)

T105 + T106 → T110 (IPAM Release)
T110 → T111 (GC Integration)

T101-T106 + T112 → Agent initialization
```

---

## Parallel Execution

### Wave 1: API Client (T101-T106)
- T101 → T102, T103 (并行) → T104, T105 (并行) → T106

### Wave 2: IPAM Integration (T107-T109)
- T103 → T107, T108 (并行) → T109

### Wave 3: Release & GC (T110-T111)
- T105 → T110 → T111

### Wave 4: Agent Wiring (T112)
- T106 + T108 + T110 → T112

---

## Testing Strategy

### Unit Tests
- T102: mTLS client creation with test certificates
- T104: Allocate API request/response handling
- T105: Release API request handling

### Integration Tests (Manual)
1. Deploy mock IaaS server
2. Configure Spiderpool to use mock server
3. Create Pod and verify:
   - IaaS allocate is called
   - MAC/VLAN are stored in SpiderEndpoint
4. Delete Pod and verify:
   - IaaS release is called

---

## Task Checklist

### Phase 2 API Client
- [x] T101 Package structure
- [x] T102 mTLS client
- [x] T103 Request/response types
- [x] T104 Allocate API
- [x] T105 Release API
- [x] T106 Logging

### Phase 2 IPAM Integration
- [x] T107 Multus MAC parsing
- [x] T108 IPAM Allocate integration
- [x] T109 MAC/VLAN storage

### Phase 2 Release & GC
- [x] T110 IPAM Release integration
- [ ] T111 GC integration

### Phase 2 Agent Wiring
- [x] T112 Agent initialization

---

## Next Steps

After Phase 2 tasks are complete:

1. End-to-end testing
2. Documentation update
3. PR review and merge
4. Consider Phase 3 (metrics, advanced features)

---

**Ready to implement**: Start with T101 (Package structure) → T102-T103 (并行)
