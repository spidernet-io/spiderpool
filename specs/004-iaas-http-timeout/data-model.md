# Phase 1 Data Model: IaaS Provider HTTP Timeout

This feature introduces no Kubernetes CRD changes. The "data model" is the
configuration and runtime types that carry the provider HTTP timeout.

## Entity: IaaS Provider Timeout Setting

Operator-configured duration controlling how long Spiderpool may wait for one
provider HTTP interaction.

| Field | Type | Source | Required | Default | Notes |
|-------|------|--------|----------|---------|-------|
| `httpRequestTimeout` | Go duration string | Helm value `iaasNetworkProvider.httpRequestTimeout` → ConfigMap → `IaaSProviderConfig.HTTPRequestTimeout` | No | `30s` (when IaaS enabled) | e.g. `30s`, `1m30s`. Empty/unset → default. |

### Mapping through the stack

```text
charts/spiderpool/values.yaml
  iaasNetworkProvider.httpRequestTimeout: "30s"
        │
        ▼  templates/configmap.yaml
ConfigMap data: conf.yml
  iaasNetworkProvider:
    serverUrl: "..."
    httpRequestTimeout: "30s"
        │
        ▼  pkg/types/k8s.go
type IaaSProviderConfig struct {
    ServerURL          string `yaml:"serverUrl,omitempty"`
    HTTPRequestTimeout string `yaml:"httpRequestTimeout,omitempty"`   // NEW
}
        │
        ▼  cmd/spiderpool-agent|controller/cmd/daemon.go (startup)
ValidateConfig(cfg) → parse + bounds check
        │
        ▼  pkg/iaas/client/client.go NewClient
http.Client{ Timeout: parsed }
```

### Validation rules

Applied in `ValidateConfig` (startup, both agent and controller). All rules only
apply when `ServerURL != ""` (IaaS integration enabled).

1. If `HTTPRequestTimeout == ""` → use default `30s` (no error).
2. Parse `HTTPRequestTimeout` with `time.ParseDuration`; on error → validation
   error naming the bad value.
3. `parsed > 0`, else → validation error.
4. `parsed < StaticSafetyLimit` (`2 * time.Minute`), else → validation error.
5. `parsed < CNIClientTimeout` (`100 * time.Second`), else → validation error.

State: configuration is validated once at startup. There are no runtime state
transitions for this entity.

## Entity: CNI Plugin-to-Agent Timeout

Spiderpool's timeout for CNI plugin requests sent to the Spiderpool Agent.

| Property | Value | Location |
|----------|-------|----------|
| ADD timeout | `100 * time.Second` | `cmd/spiderpool/cmd/command_add.go` |
| DEL timeout | `100 * time.Second` | `cmd/spiderpool/cmd/command_delete.go` |
| Configurable today? | No | — |
| Representation after change | shared constant `constant.DefaultCNIClientTimeout` | `pkg/constant` (or `pkg/iaas/client`) |

This entity is the comparison budget for validation rule 5. If it becomes
configurable later, validation reads the configured value instead of the constant.

## Entity: Parent Operation Budget (runtime)

Remaining time available from the current operation `context.Context` when a
provider call is about to be made.

| Property | Type | Source |
|----------|------|--------|
| deadline | `time.Time, ok bool` | `ctx.Deadline()` in the IaaS client |
| remaining | `time.Duration` | `time.Until(deadline)` when `ok` |

Effective per-call timeout = `min(configuredTimeout, remaining)` when a deadline
is present; otherwise `configuredTimeout`. If `remaining <= 0`, the call is not
started and a clear "parent budget exhausted" error is returned.

## Entity: Provider Call Outcome

Result of a provider interaction, used for diagnostics (User Story 3).

| Outcome | Trigger |
|---------|---------|
| success | provider returns 2xx within budget |
| provider timeout | request exceeds effective per-call timeout |
| parent-budget rejection | `remaining <= 0` before call starts |
| invalid configuration | startup validation failure |
| provider error | non-2xx response or transport error |
