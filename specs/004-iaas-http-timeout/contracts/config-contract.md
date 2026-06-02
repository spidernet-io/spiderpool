# Contract: IaaS Provider Timeout Configuration & Validation

This feature exposes no new network API. Its external contracts are (1) the Helm
value / ConfigMap configuration surface and (2) the startup validation behavior.

## 1. Helm value contract

```yaml
# charts/spiderpool/values.yaml
iaasNetworkProvider:
  ## @param iaasNetworkProvider.serverUrl the URL of the IaaS provider service.
  serverUrl: ""
  ## @param iaasNetworkProvider.httpRequestTimeout HTTP timeout for a single
  ## provider request (allocate/release). Go duration string (e.g. "30s",
  ## "1m30s"). Must be > 0, < 2m, and < 100s (the current CNI plugin-to-agent
  ## timeout). Empty defaults to "30s".
  httpRequestTimeout: "30s"
```

Rendered into the ConfigMap (`templates/configmap.yaml`):

```yaml
iaasNetworkProvider:
  serverUrl: {{ (.Values.iaasNetworkProvider).serverUrl | default "" | quote }}
  httpRequestTimeout: {{ (.Values.iaasNetworkProvider).httpRequestTimeout | default "30s" | quote }}
```

### Backward compatibility

- Omitting `iaasNetworkProvider.httpRequestTimeout` MUST render `"30s"` and
  preserve current behavior.
- When `serverUrl` is empty (IaaS disabled), `httpRequestTimeout` is ignored and
  never causes a validation failure.

## 2. Go type contract

```go
// pkg/types/k8s.go
type IaaSProviderConfig struct {
    ServerURL          string `yaml:"serverUrl,omitempty"`
    HTTPRequestTimeout string `yaml:"httpRequestTimeout,omitempty"` // Go duration string; "" => default 30s
}
```

## 3. Shared constant contract

```go
// pkg/constant (or pkg/iaas/client)
const (
    // DefaultCNIClientTimeout is the timeout the CNI plugin uses when calling the
    // Spiderpool agent. Shared by CNI ADD and CNI DEL.
    DefaultCNIClientTimeout = 100 * time.Second

    // IaaSTimeoutStaticLimit is the absolute upper bound for a provider timeout.
    IaaSTimeoutStaticLimit = 2 * time.Minute

    // DefaultIaaSProviderTimeout is used when serverUrl is set but timeout is empty.
    DefaultIaaSProviderTimeout = 30 * time.Second
)
```

`cmd/spiderpool/cmd/command_add.go` and `command_delete.go` MUST use
`DefaultCNIClientTimeout` instead of the literal `100 * time.Second`.

## 4. Validation contract

`ValidateConfig(cfg *IaaSProviderConfig) error` behavior when `ServerURL != ""`:

| Input `httpRequestTimeout` | Result |
|-----------------|--------|
| `""` | OK, effective value `30s` |
| `"30s"` | OK |
| `"1m30s"` | OK |
| `"0s"` / negative | error: `must be greater than 0` |
| `"abc"` | error: `invalid duration ...` |
| `"2m"` / `">=2m"` | error: `must be less than the 2m safety limit` |
| `"100s"` / `">=100s"` | error: `must be less than the 100s CNI plugin-to-agent timeout` |

Error messages MUST include the configured value and the limit it violated
(FR-012, SC-005). When `ServerURL == ""`, `ValidateConfig` returns `nil`
regardless of `httpRequestTimeout`.

## 5. Client construction contract

`NewClient(cfg, logger)` MUST:

1. Call `ValidateConfig(cfg)` and return its error.
2. Parse the (validated) timeout, defaulting empty to `DefaultIaaSProviderTimeout`.
3. Set `http.Client.Timeout` to the parsed value.

## 6. Runtime call contract

Before issuing each provider request, the client SHOULD inspect `ctx.Deadline()`:

- deadline present and `time.Until(deadline) <= 0` → return a
  "parent budget exhausted" error without calling the provider.
- otherwise issue the request via `http.NewRequestWithContext(ctx, ...)`; the
  effective bound is `min(http.Client.Timeout, remaining context budget)`.
