# Implementation Plan: IaaS Network Provider Integration

**Feature**: IaaS Network Provider Integration  
**Branch**: `003-iaas-provider-integration`  
**Created**: 2025-04-27  
**Spec**: [spec.md](./spec.md)

---

## 1. Technical Context

### Project Structure

```
spiderpool/
├── charts/spiderpool/          # Helm charts
│   ├── values.yaml            # Helm values (needs: iaasNetworkProvider config)
│   └── templates/             # Kubernetes manifests
│       ├── daemonset.yaml     # Agent daemonset (needs: secret volume mounts)
│       └── deployment.yaml    # Controller deployment (needs: secret volume mounts)
│
├── cmd/
│   ├── spiderpool-agent/
│   │   └── cmd/
│   │       ├── config.go      # Agent configuration (needs: IaaS provider config)
│   │       └── daemon.go      # Agent initialization (needs: secret validation)
│   │
│   └── spiderpool-controller/
│       └── cmd/
│           ├── config.go      # Controller configuration (needs: IaaS provider config)
│           └── daemon.go      # Controller initialization (needs: secret validation)
│
└── pkg/
    └── config/               # Configuration types
        └── config.go          # Global config struct (needs: IaaSProviderConfig)
```

### Key Technologies

- **Helm**: Kubernetes package manager for templating manifests
- **Kubernetes Secrets**: For storing TLS certificates
- **Volume Mounts**: Mounting secrets into pods
- **Environment Variables**: Passing configuration to applications
- **Go**: Application language for Agent and Controller

### Existing Similar Implementations

Looking at Spiderpool's existing Helm chart structure:
- Certificate mounting for webhook: `spiderpool-controller.tls`
- Secret configuration pattern in `values.yaml`
- Volume and volumeMount patterns in daemonset/deployment templates

---

## 2. Constitution Check

### Constitution Alignment

| Principle | Alignment | Notes |
|-----------|-----------|-------|
| Minimal Configuration | ✅ | Only 4 fields required: url, secret name, secret namespace |
| Secure by Default | ✅ | mTLS enforced when configured |
| Backward Compatible | ✅ | Empty URL disables integration completely |
| Separation of Concerns | ✅ | Phase 1: Config only, Phase 2: API calls |

### Gate Evaluation

**Architecture Gate**:
- No structural changes to core IPAM logic (Phase 1)
- Configuration follows existing Helm patterns
- ✅ PASS

**Security Gate**:
- mTLS with certificate mounting
- Secrets in dedicated namespace
- No hardcoded credentials
- ✅ PASS

**Testing Gate**:
- Helm template tests for secret mounting
- Configuration validation tests
- ⏳ PENDING (TBD in tasks)

---

## 3. Design & Contracts

### Data Model

#### Configuration Schema

```yaml
iaasNetworkProvider:
  url: string                    # IaaS provider endpoint (host:port)
  tlsSecret:
    name: string                 # Kubernetes secret name
    namespace: string            # Secret namespace
```

#### Environment Variables

| Variable | Source | Description |
|----------|--------|-------------|
| `SPIDERPOOL_IAAS_PROVIDER_URL` | `values.yaml` → ConfigMap | IaaS provider URL |
| `SPIDERPOOL_IAAS_TLS_SECRET_NAME` | `values.yaml` → ConfigMap | TLS secret name |
| `SPIDERPOOL_IAAS_TLS_SECRET_NAMESPACE` | `values.yaml` → ConfigMap | TLS secret namespace |
| `SPIDERPOOL_IAAS_TLS_CERT_PATH` | Hardcoded | Mounted cert path |
| `SPIDERPOOL_IAAS_TLS_KEY_PATH` | Hardcoded | Mounted key path |

### Helm Values Contract

New `values.yaml` section:

```yaml
iaasNetworkProvider:
  # URL of the IaaS provider service (host:port)
  # If empty, IaaS integration is disabled
  url: ""
  
  # TLS certificate configuration for mTLS authentication
  # Secret must exist and contain tls.crt and tls.key
  tlsSecret:
    name: ""
    namespace: ""
```

### Secret Mount Contract

**Source Secret** (user-provided):
```yaml
apiVersion: v1
kind: Secret
metadata:
  name: iaas-provider-client-cert
  namespace: spiderpool
type: kubernetes.io/tls
data:
  tls.crt: <base64-encoded-cert>
  tls.key: <base64-encoded-key>
```

**Mount Target**:
- Path: `/etc/spiderpool/iaas-tls/`
- Files: `tls.crt`, `tls.key`
- Read-only: true

---

## 4. Implementation Phases

### Phase 0: Research (Complete)

No research needed. This is a configuration-only task using existing Helm/Kubernetes patterns.

### Phase 1: Configuration Infrastructure

#### T001: Update Helm values schema
**File**: `charts/spiderpool/values.yaml`
**Task**: Add `iaasNetworkProvider` configuration section
**Effort**: Small
**Dependencies**: None

#### T002: Add Agent secret volume mount
**File**: `charts/spiderpool/templates/daemonset.yaml`
**Task**: Add volume and volumeMount for TLS secret in Agent daemonset
**Effort**: Small
**Dependencies**: T001

#### T003: Add Controller secret volume mount
**File**: `charts/spiderpool/templates/deployment.yaml`
**Task**: Add volume and volumeMount for TLS secret in Controller deployment
**Effort**: Small
**Dependencies**: T001

#### T004: Create ConfigMap template for IaaS config
**File**: `charts/spiderpool/templates/configmap.yaml` (new or existing)
**Task**: Add IaaS provider configuration to ConfigMap
**Effort**: Small
**Dependencies**: T001

#### T005: Add Go configuration types
**File**: `pkg/config/config.go`
**Task**: Add `IaaSProviderConfig` struct
**Effort**: Small
**Dependencies**: None

```go
type IaaSProviderConfig struct {
    URL            string            `yaml:"url"`
    TLSSecret      TLSSecretConfig   `yaml:"tlsSecret"`
}

type TLSSecretConfig struct {
    Name      string `yaml:"name"`
    Namespace string `yaml:"namespace"`
}
```

#### T006: Agent configuration loading
**File**: `cmd/spiderpool-agent/cmd/config.go`
**Task**: Load IaaS provider config from environment
**Effort**: Small
**Dependencies**: T005

#### T007: Controller configuration loading
**File**: `cmd/spiderpool-controller/cmd/config.go`
**Task**: Load IaaS provider config from environment
**Effort**: Small
**Dependencies**: T005

#### T008: Agent startup validation
**File**: `cmd/spiderpool-agent/cmd/daemon.go`
**Task**: Validate IaaS secret existence at startup (when configured)
**Effort**: Medium
**Dependencies**: T006

#### T009: Controller startup validation
**File**: `cmd/spiderpool-controller/cmd/daemon.go`
**Task**: Validate IaaS secret existence at startup (when configured)
**Effort**: Medium
**Dependencies**: T007

#### T010: Helm template tests
**Files**: Test files or CI scripts
**Task**: Add tests for Helm template rendering with IaaS config
**Effort**: Medium
**Dependencies**: T002, T003, T004

### Phase 2: Future (Not in Current Plan)

See spec.md for Phase 2 requirements (API client, hooks, MAC storage).

---

## 5. Task Dependencies

```
T001 (Helm values)
  ├── T002 (Agent volume mount)
  ├── T003 (Controller volume mount)
  └── T004 (ConfigMap)

T005 (Go types)
  ├── T006 (Agent config loading)
  │   └── T008 (Agent validation)
  └── T007 (Controller config loading)
      └── T009 (Controller validation)

T002 + T003 + T004 → T010 (Helm tests)
```

---

## 6. Quickstart

### Pre-requisites

1. Kubernetes cluster with Spiderpool installed
2. IaaS provider service endpoint accessible
3. TLS certificate and key for mTLS authentication

### Setup Steps

1. **Create TLS Secret**:
```bash
kubectl create secret tls iaas-provider-client-cert \
  --cert=client.crt \
  --key=client.key \
  -n spiderpool
```

2. **Configure Spiderpool**:
```bash
helm upgrade spiderpool spiderpool/spiderpool \
  --set iaasNetworkProvider.url="iaas-provider:444" \
  --set iaasNetworkProvider.tlsSecret.name="iaas-provider-client-cert" \
  --set iaasNetworkProvider.tlsSecret.namespace="spiderpool"
```

3. **Verify Mount**:
```bash
kubectl exec -n spiderpool ds/spiderpool-agent -- ls /etc/spiderpool/iaas-tls/
# Should show: tls.crt tls.key
```

---

## 7. Deliverables

| Artifact | Path | Status |
|----------|------|--------|
| Spec | `specs/003-iaas-provider-integration/spec.md` | ✅ Complete |
| Plan | `specs/003-iaas-provider-integration/plan.md` | ✅ Complete |
| Checklist | `specs/003-iaas-provider-integration/checklists/requirements.md` | ✅ Complete |
| Helm Values | `charts/spiderpool/values.yaml` | ⏳ T001 |
| Agent Daemonset | `charts/spiderpool/templates/daemonset.yaml` | ⏳ T002 |
| Controller Deployment | `charts/spiderpool/templates/deployment.yaml` | ⏳ T003 |
| ConfigMap | `charts/spiderpool/templates/configmap.yaml` | ⏳ T004 |
| Go Types | `pkg/config/config.go` | ⏳ T005 |
| Agent Config | `cmd/spiderpool-agent/cmd/config.go` | ⏳ T006 |
| Controller Config | `cmd/spiderpool-controller/cmd/config.go` | ⏳ T007 |
| Agent Validation | `cmd/spiderpool-agent/cmd/daemon.go` | ⏳ T008 |
| Controller Validation | `cmd/spiderpool-controller/cmd/daemon.go` | ⏳ T009 |
| Helm Tests | Test files | ⏳ T010 |

---

## 8. Next Steps

1. Review plan with stakeholders
2. Create feature branch: `003-iaas-provider-integration`
3. Execute Phase 1 tasks (T001-T010)
4. Test Helm chart rendering
5. Validate secret mounting in test cluster
6. Proceed to `/speckit.tasks` for task generation
