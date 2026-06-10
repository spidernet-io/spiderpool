# Quickstart: IaaS Provider HTTP Timeout

## What this feature does

Lets operators configure how long Spiderpool waits for a single IaaS provider
HTTP call (allocate/release). The value is validated at startup so it can never
exceed the time Spiderpool has to complete a CNI operation.

## Provider timing model

Understanding the provider internals is important for choosing a correct value.

A single IaaS provider request goes through two stages:

| Stage | Max duration | Description |
|---|---|---|
| Rate-limit wait | 30 s | Provider checks its token bucket; if no slot is available it waits up to 30 s before accepting the request |
| Cloud API call | 16 s | Provider forwards the request to the underlying cloud platform; network latency and cloud-side processing can take up to 16 s |
| **Worst case total** | **~48 s** | Sum of the two stages plus a small network round-trip margin |

**Consequence**: a timeout value shorter than ~48 s risks cancelling a request
that the provider has already accepted and started executing. This creates a
state inconsistency: Spiderpool treats it as a failure while the cloud
operation may have succeeded or be in progress.

## Time budget hierarchy

The full budget chain explains why `httpRequestTimeout` has the constraints it does:

| Layer | Default timeout | Description |
|---|---|---|
| kubelet sandbox operation | **2 min** | kubelet's default timeout for the entire sandbox setup. If the CNI pipeline does not finish within this window, the Pod fails to start. This is the outermost budget. |
| Spiderpool CNI plugin → agent call | **100 s** | The timeout the Spiderpool CNI binary uses when calling spiderpool-agent. This is the total budget available to the agent to complete all IPAM and IaaS work. |
| IaaS provider HTTP call | **50 s** (default) | Per-call timeout set by `httpRequestTimeout`. Must fit inside the 100 s agent budget alongside all other IPAM processing. |
| Provider worst-case completion | **~48 s** | Maximum time a single provider request can take (30 s rate-limit wait + 16 s cloud API). This is the minimum meaningful value for `httpRequestTimeout`. |

## Recommended value

| Scenario | Recommended `httpRequestTimeout` |
|---|---|
| Default / general use | `50s` (default) |
| Low-latency private cloud with no rate limiting | `20s` |
| High-contention environment with long rate-limit queues | `55s`–`59s` (must stay `< 100s`) |

**Why 50 s**: covers the 48 s worst case with a 2 s margin, while staying well
below the 100 s CNI plugin-to-agent budget.

## Configure via Helm

```yaml
# values.yaml
iaasNetworkProvider:
  serverUrl: "https://iaas-provider.example.com:8443"
  httpRequestTimeout: "50s"   # Go duration; must be > 0, < 2m, and < 100s
```

```bash
helm upgrade --install spiderpool spiderpool/spiderpool \
  --set iaasNetworkProvider.serverUrl="https://iaas-provider.example.com:8443" \
  --set iaasNetworkProvider.httpRequestTimeout="50s"
```

### Validation rules

- Accepted format: Go duration string (`50s`, `45s`, `1m`).
- Must be greater than `0`.
- Must be less than `2m` (static safety limit).
- Must be less than `100s` (current CNI plugin-to-agent timeout for ADD and DEL).
- Empty/unset defaults to `50s`.
- Ignored entirely when `serverUrl` is empty (IaaS integration disabled).
- Validation failure at startup is **fatal**: the agent/controller will not start
  with an invalid timeout.

## Verify

### Helm rendering

```bash
helm template spiderpool charts/spiderpool \
  --set iaasNetworkProvider.serverUrl="https://x:8443" \
  --set iaasNetworkProvider.httpRequestTimeout="50s" \
  | grep -A2 iaasNetworkProvider
# expect:
#   iaasNetworkProvider:
#     serverUrl: "https://x:8443"
#     httpRequestTimeout: "50s"
```

### Startup validation (negative case)

Set an unsafe value and confirm the component reports a clear error:

```bash
helm upgrade ... --set iaasNetworkProvider.httpRequestTimeout="2m"
# agent/controller log (fatal):
#   IaaS provider configuration validation failed: invalid iaasNetworkProvider.httpRequestTimeout "2m": ...
```

### Unit tests

```bash
make unittest-tests
# or target the IaaS client package:
go test ./pkg/iaas/client/...
```

## Behavior at runtime

Each allocate/release call enforces two independent guards before sending the
HTTP request:

### Guard 1 — minimum parent budget check

Before starting a call, Spiderpool checks how much time remains in the parent
operation context (the CNI ADD/DEL budget). If the remaining time is **less than
the provider worst-case** (~48 s), Spiderpool **does not start the call** and
returns immediately with `"parent budget insufficient"`.

**Why**: sending a request with insufficient budget risks the provider consuming
a rate-limit slot and starting the cloud operation, only for Spiderpool to
receive a cancellation error. This guard prevents that state inconsistency.

**What this means for operators**: if CNI operations are consistently failing
with `"parent budget insufficient"`, either the `httpRequestTimeout` is too
large relative to the available CNI budget, or the overall CNI pipeline is
taking too long before reaching the IaaS call.

### Guard 2 — per-request timeout context

After passing Guard 1, Spiderpool derives a child context with a deadline of
`httpRequestTimeout` from the current time. The effective HTTP deadline is:

```
effective deadline = min(now + httpRequestTimeout, parent deadline)
```

Since Guard 1 ensures `remaining parent budget >= 48 s >= httpRequestTimeout`
for correctly configured deployments, the configured `httpRequestTimeout`
normally wins and the provider call has its full configured budget.

### Error message guide

| Error message | Meaning | Action |
|---|---|---|
| `parent budget insufficient: Xs remaining is less than provider worst-case 48s` | CNI operation budget was almost exhausted before the IaaS call was attempted | Check pipeline latency; consider raising CNI timeout or lowering `httpRequestTimeout` |
| `provider-interaction timeout: ... exceeded configured timeout 50s` | Provider did not respond within `httpRequestTimeout` | Check provider health; consider raising `httpRequestTimeout` if provider load is high |
| `parent budget exhausted: ... cancelled by parent context deadline` | Parent deadline arrived while provider was responding | As above, but the parent budget ran out before the configured timeout |

## Rollback

Remove or empty `iaasNetworkProvider.httpRequestTimeout`; behavior reverts to the
default `50s`. Removing `serverUrl` disables IaaS integration and the timeout
entirely.
