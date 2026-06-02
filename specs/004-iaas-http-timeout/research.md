# Phase 0 Research: IaaS Provider HTTP Timeout

This document resolves the unknowns flagged in the spec's Clarifications,
Assumptions, and Edge Cases sections by inspecting the current Spiderpool code.

## Decision 1: Where the provider HTTP timeout is applied

- **Decision**: Add a configurable timeout to the IaaS HTTP client and set it as
  `http.Client.Timeout` in `NewClient`, replacing the hardcoded `30 * time.Second`.
- **Rationale**: The IaaS client is the single chokepoint for all provider
  interactions. Both `AllocateIPs` and `ReleaseIP` use the same
  `c.httpClient.Do(httpReq)` path, so setting `http.Client.Timeout` once bounds
  every provider call (FR-003). `http.Client.Timeout` covers connection, redirects,
  and reading the response body.
- **Evidence**:
  - `pkg/iaas/client/client.go:85-90` — `httpClient := &http.Client{ ... Timeout: 30 * time.Second }`.
  - `pkg/iaas/client/client.go:127` (`AllocateIPs`) and `:219` (`releaseSingleIP`)
    both call `c.httpClient.Do(httpReq)`.
- **Alternatives considered**: Per-call `context.WithTimeout` inside
  `AllocateIPs`/`ReleaseIP`. Rejected as the primary mechanism because it
  duplicates logic across call sites; however it is used additively (see Decision 3)
  to also honor the parent context deadline.

## Decision 2: What the configured timeout is compared against (parent budget)

- **Decision**: Validate at startup that the configured provider timeout is
  strictly smaller than the current CNI plugin-to-agent timeout of **100 seconds**,
  which is shared by both CNI ADD and CNI DEL. Also enforce the static safety
  limit of **2 minutes** and `> 0`.
- **Rationale**: The clarification session established that kubelet's original CNI
  timeout is not reliably visible to the agent. The concrete, code-defined budget
  that bounds a CNI-triggered provider call is the plugin-to-agent timeout the CNI
  binary sets before calling the agent. Today that value is a hardcoded 100s for
  both operations.
- **Evidence**:
  - `cmd/spiderpool/cmd/command_add.go:112` — `ctx, cancel := context.WithTimeout(context.Background(), 100*time.Second)`.
  - `cmd/spiderpool/cmd/command_delete.go:92` — identical `100*time.Second`.
- **Alternatives considered**: Hardcoding a 2-minute kubelet assumption. Rejected
  per the spec because kubelet defaults are configurable and not the binding budget
  in the current flow.

## Decision 3: Whether the parent context deadline can be obtained

- **Decision**: At call time, derive the effective per-call deadline as
  `min(configuredTimeout, remaining parent context budget)` by checking
  `ctx.Deadline()` before issuing the request. `http.Client.Timeout` provides the
  static upper bound; the context deadline (when present) provides the dynamic one.
- **Rationale**: Go's `http.Client.Do` already aborts when the request context is
  cancelled or its deadline passes, because requests are created with
  `http.NewRequestWithContext(ctx, ...)`. So if the parent context carries a
  deadline, the call already cannot exceed it. The explicit `ctx.Deadline()` check
  is used only to produce a clear, operator-facing error (User Story 3 / FR-012)
  when the remaining budget is already smaller than or exhausted relative to the
  configured timeout, rather than failing opaquely at the network layer.
- **Evidence**:
  - `pkg/iaas/client/client.go:119` and `:212` — `http.NewRequestWithContext(ctx, ...)`,
    so the parent context already governs cancellation.
  - The IPAM ADD/DEL path passes the inbound request context down to
    `callIaaSAllocate`/`callIaaSRelease` (`pkg/ipam/iaas.go:25,113`), which forward
    it into the client calls.
- **Caveat**: Over the unix-socket OpenAPI transport the CNI plugin's 100s deadline
  is not automatically propagated as a context deadline into the agent process; the
  agent's request context may carry a server-side deadline or none. The static
  100s comparison (Decision 2) therefore remains the authoritative startup gate,
  and the runtime `ctx.Deadline()` check is best-effort defense in depth.

## Decision 4: Default value and configuration surface

- **Decision**: Add an optional `httpRequestTimeout` field (Go `HTTPRequestTimeout`)
  to `IaaSProviderConfig` (`yaml:"httpRequestTimeout,omitempty"`), surfaced as Helm
  value `iaasNetworkProvider.httpRequestTimeout` and rendered into the
  agent/controller ConfigMap. Accepted format is a Go
  duration string (e.g. `30s`, `1m30s`). When IaaS integration is enabled and the
  timeout is unset/empty, default to **30s** (the current hardcoded value).
- **Rationale**: Matches the existing `serverUrl` configuration pattern, preserves
  backward compatibility (Principle III, FR-004/FR-013), and keeps explicit
  configuration above inferred defaults.
- **Evidence**:
  - The name is namespaced under `iaasNetworkProvider`, so it omits a redundant
    "provider" prefix while still stating it bounds a single HTTP request.
  - `pkg/types/k8s.go:135-137` — `IaaSProviderConfig{ ServerURL string }`.
  - `charts/spiderpool/values.yaml:1072-1074` — `iaasNetworkProvider.serverUrl`.
  - `charts/spiderpool/templates/configmap.yaml:39-40` — renders `serverUrl`.

## Decision 5: Removing the duplicated 100s literal

- **Decision**: Introduce a shared exported constant for the CNI plugin-to-agent
  timeout (e.g. `constant.DefaultCNIClientTimeout = 100 * time.Second`) and use it
  in `command_add.go`, `command_delete.go`, and the startup validation.
- **Rationale**: FR-008/FR-009/FR-010 require ADD and DEL to share one value today
  and to support future configurability without re-touching the literal. A single
  constant removes drift and gives validation one source of truth.
- **Alternatives considered**: Keep two literals and a third in validation.
  Rejected: violates DRY and risks the three values diverging.

## Validation rules summary (resolves Edge Cases)

| Condition | Behavior |
|-----------|----------|
| httpRequestTimeout unset/empty, IaaS enabled | default to `30s` |
| httpRequestTimeout `<= 0` | startup validation error |
| httpRequestTimeout unparsable | startup validation error |
| httpRequestTimeout `>= 2m` (static limit) | startup validation error |
| httpRequestTimeout `>= 100s` (CNI client timeout) | startup validation error |
| valid httpRequestTimeout, parent ctx has deadline | use `min(httpRequestTimeout, remaining budget)` |
| parent ctx deadline already exhausted | do not start call; clear error |
| parent ctx has no deadline | use configured httpRequestTimeout + static validation |

## Open items

- None blocking. mTLS for the IaaS client remains a separate TODO
  (`pkg/iaas/client/client.go:80`) and is out of scope for this feature.
