# Tasks: IaaS Network Provider Integration

**Feature**: IaaS Network Provider Integration  
**Branch**: `003-iaas-provider-integration`  
**Generated**: 2025-04-27  
**Spec**: [spec.md](./spec.md)  
**Plan**: [plan.md](./plan.md)

---

## Overview

This task list implements Phase 1 of the IaaS Network Provider Integration: Configuration Infrastructure and Secret Mounting. The actual API client and IPAM hooks will be implemented in Phase 2 (future work).

**Total Tasks**: 10  
**Estimated Effort**: Small to Medium (Helm + Go configuration)  
**Parallel Tasks**: T002-T004, T006-T007

---

## Phase 0: Project Setup

**Goal**: Ensure all prerequisites are ready for implementation  
**Test Criteria**: N/A (setup phase)

- [ ] T000 Verify existing documentation is complete
  - [ ] Review spec.md for clarity
  - [ ] Review plan.md for task completeness
  - [ ] Review data-model.md for accuracy

---

## Phase 1: Helm Chart Configuration

**Goal**: Add IaaS provider configuration support to Spiderpool Helm chart  
**User Story**: FR1, FR3 (Configuration Infrastructure)  
**Independent Test Criteria**: 
- `helm template` renders without errors when `iaasNetworkProvider` is configured
- Generated manifests include secret volume mounts when URL is set
- Generated manifests exclude secret volume mounts when URL is empty

### Implementation Tasks

- [x] T001 Add `iaasNetworkProvider` values to Helm chart schema
  - **File**: `charts/spiderpool/values.yaml`
  - **Description**: Add `iaasNetworkProvider` configuration section with `url`, `tlsSecret.name`, `tlsSecret.namespace`
  - **Acceptance**:
    ```yaml
    iaasNetworkProvider:
      url: ""
      tlsSecret:
        name: ""
        namespace: ""
    ```

- [x] T002 [P] Add secret volume mount to Agent DaemonSet template
  - **File**: `charts/spiderpool/templates/daemonset.yaml`
  - **Description**: Add volume and volumeMount for TLS secret when `iaasNetworkProvider.url` is non-empty
  - **Condition**: `{{- if .Values.iaasNetworkProvider.url }}`
  - **Mount Path**: `/etc/spiderpool/iaas-tls/`
  - **Files**: `tls.crt`, `tls.key`

- [x] T003 [P] Add secret volume mount to Controller Deployment template
  - **File**: `charts/spiderpool/templates/deployment.yaml`
  - **Description**: Add volume and volumeMount for TLS secret (same as T002)
  - **Condition**: `{{- if .Values.iaasNetworkProvider.url }}`

- [x] T004 [P] Add IaaS configuration to ConfigMap template
  - **File**: `charts/spiderpool/templates/configmap.yaml` (or create if not exists)
  - **Description**: Add IaaS provider URL and TLS secret reference as environment variables
  - **Variables**:
    - `SPIDERPOOL_IAAS_PROVIDER_URL`
    - `SPIDERPOOL_IAAS_TLS_SECRET_NAME`
    - `SPIDERPOOL_IAAS_TLS_SECRET_NAMESPACE`
    - `SPIDERPOOL_IAAS_TLS_CERT_PATH` (hardcoded: `/etc/spiderpool/iaas-tls/tls.crt`)
    - `SPIDERPOOL_IAAS_TLS_KEY_PATH` (hardcoded: `/etc/spiderpool/iaas-tls/tls.key`)

- [ ] T005 Add Helm template tests for IaaS configuration
  - **Files**: Test scripts or CI pipeline
  - **Description**: Add tests to verify Helm template rendering
  - **Test Cases**:
    1. Template renders with empty `iaasNetworkProvider.url` (no volumes)
    2. Template renders with valid `iaasNetworkProvider` config (volumes present)
    3. Template fails or warns with URL set but missing TLS secret config

---

## Phase 2: Go Configuration Types

**Goal**: Define Go structs and loading logic for IaaS provider configuration  
**User Story**: FR3 (Configuration Infrastructure)  
**Independent Test Criteria**:
- Configuration structs compile without errors
- Unit tests pass for configuration loading

### Implementation Tasks

- [x] T006 [P] Add `IaaSProviderConfig` Go types to config package
  - **File**: `pkg/config/config.go`
  - **Description**: Define configuration structs
  - **Code**:
    ```go
    type IaaSProviderConfig struct {
        URL       string           `yaml:"url"`
        TLSSecret TLSSecretConfig  `yaml:"tlsSecret"`
    }

    type TLSSecretConfig struct {
        Name      string `yaml:"name"`
        Namespace string `yaml:"namespace"`
    }
    ```

- [x] T007 [P] Add configuration loading to Agent
  - **File**: `cmd/spiderpool-agent/cmd/config.go`
  - **Description**: Load IaaS provider config from environment variables into global config
  - **Variables**:
    - `SPIDERPOOL_IAAS_PROVIDER_URL`
    - `SPIDERPOOL_IAAS_TLS_SECRET_NAME`
    - `SPIDERPOOL_IAAS_TLS_SECRET_NAMESPACE`

- [x] T008 [P] Add configuration loading to Controller
  - **File**: `cmd/spiderpool-controller/cmd/config.go`
  - **Description**: Load IaaS provider config (same as T007)

---

## Phase 3: Validation and Integration

**Goal**: Validate configuration at startup and ensure proper integration  
**User Story**: FR3.8-FR3.9 (Startup Validation)  
**Independent Test Criteria**:
- Agent starts successfully with valid IaaS config
- Agent logs appropriate messages about IaaS configuration
- Controller starts successfully with valid IaaS config

### Implementation Tasks

- [x] T009 Add Agent startup validation for IaaS configuration
  - **File**: `cmd/spiderpool-agent/cmd/daemon.go` (or initialization code)
  - **Description**: Validate secret existence when URL is configured
  - **Validation**:
    1. If URL is empty: skip validation, log "IaaS integration disabled"
    2. If URL is set: validate TLS secret name and namespace are not empty
    3. Optional: Check if secret files exist at mount path
  - **Error Handling**: Log warning if validation fails, but don't block startup (fail-open for Phase 1)

- [x] T010 Add Controller startup validation for IaaS configuration
  - **File**: `cmd/spiderpool-controller/cmd/daemon.go`
  - **Description**: Same validation as T009 for Controller

---

## Phase 4: Documentation and Examples

**Goal**: Update documentation with IaaS configuration examples  
**User Story**: Documentation and usability  
**Independent Test Criteria**: Documentation is accurate and complete

- [x] T011 Update Helm values documentation
  - **File**: `charts/spiderpool/README.md` (or relevant doc)
  - **Description**: Add `iaasNetworkProvider` configuration example
  - **Include**: 
    - Configuration syntax
    - Secret creation example
    - Troubleshooting tips

- [x] T012 Update quickstart.md with verification steps
  - **File**: `specs/003-iaas-provider-integration/quickstart.md`
  - **Description**: Add actual verification commands for the implementation
  - **Commands**:
    - Check environment variables
    - Verify secret mount
    - Validate certificate files

---

## Dependency Graph

```
T001 (Helm values)
    ├── T002 (Agent volume mount)
    ├── T003 (Controller volume mount)
    └── T004 (ConfigMap)
    
T001 ──> T005 (Helm tests)
T002 ──> T005
T003 ──> T005
T004 ──> T005

T006 (Go types)
    ├── T007 (Agent config loading)
    └── T008 (Controller config loading)

T007 ──> T009 (Agent validation)
T008 ──> T010 (Controller validation)

T005, T009, T010 ──> T011, T012 (Docs)
```

---

## Parallel Execution

### Wave 1: Helm Chart (T001-T005)
- T001, T002, T003, T004 can be done in parallel after T001
- T005 depends on T002, T003, T004

### Wave 2: Go Implementation (T006-T010)
- T006 can be done in parallel with Wave 1
- T007, T008 can be done in parallel after T006
- T009 depends on T007
- T010 depends on T008

### Wave 3: Documentation (T011-T012)
- T011, T012 can be done in parallel after Wave 1 and Wave 2 complete

---

## Suggested MVP Scope

**MVP = T001-T010** (Full Phase 1)

Phase 1 is already minimal - it only includes configuration infrastructure. All 10 tasks are required for a complete, testable feature.

**Optional for MVP**:
- T011, T012 (Documentation) - Can be done immediately after or in parallel

---

## Test Strategy

### Manual Testing

1. **Helm Template Test**:
   ```bash
   helm template spiderpool charts/spiderpool \
     --set iaasNetworkProvider.url="test:444" \
     --set iaasNetworkProvider.tlsSecret.name="test-secret" \
     --set iaasNetworkProvider.tlsSecret.namespace="spiderpool" \
     | grep -A5 "iaas-tls"
   ```

2. **Secret Mount Test**:
   ```bash
   # Create test secret
   kubectl create secret tls test-secret \
     --cert=/path/to/test.crt \
     --key=/path/to/test.key \
     -n spiderpool
   
   # Install/upgrade with IaaS config
   helm upgrade spiderpool charts/spiderpool \
     --set iaasNetworkProvider.url="test:444" \
     --set iaasNetworkProvider.tlsSecret.name="test-secret"
   
   # Verify mount
   kubectl exec -n spiderpool ds/spiderpool-agent -- ls /etc/spiderpool/iaas-tls/
   ```

### Automated Tests

- T005: Helm template rendering tests
- Go unit tests for configuration loading (T007, T008)
- Integration tests for validation (T009, T010) - optional for Phase 1

---

## Implementation Notes

### Important: Phase 1 Scope

This task list **ONLY** covers configuration infrastructure (Phase 1). The following are **NOT included** and will be Phase 2:

- HTTP client for IaaS API communication
- IPAM hooks to call IaaS allocate/release APIs
- MAC address storage beyond configuration
- IP Garbage Collection integration

### Backward Compatibility

All Phase 1 tasks maintain backward compatibility:
- Empty `iaasNetworkProvider.url` disables all IaaS features
- No changes to existing IPAM logic
- No breaking changes to Helm values

### Security Considerations

- TLS certificates are mounted read-only
- Private keys (`tls.key`) are never logged
- Certificate paths are hardcoded to prevent traversal attacks

---

## Task Checklist

### Pre-Implementation
- [ ] Review spec.md and plan.md
- [ ] Ensure Kubernetes test environment is available
- [ ] Create feature branch: `003-iaas-provider-integration`

### Implementation
- [ ] T001 Helm values
- [ ] T002 Agent DaemonSet
- [ ] T003 Controller Deployment
- [ ] T004 ConfigMap
- [ ] T005 Helm tests
- [ ] T006 Go types
- [ ] T007 Agent config loading
- [ ] T008 Controller config loading
- [ ] T009 Agent validation
- [ ] T010 Controller validation

### Post-Implementation
- [ ] T011 Documentation
- [ ] T012 Quickstart update
- [ ] Manual testing on test cluster
- [ ] PR review and merge

---

## Next Steps

After Phase 1 tasks are complete:

1. Merge Phase 1 changes
2. Test in staging environment
3. Plan Phase 2: IaaS API client and IPAM hooks
4. Create Phase 2 specification and tasks

---

**Ready to implement**: Start with T001 (Helm values) → T002-T004 (parallel) → T005 (tests)
