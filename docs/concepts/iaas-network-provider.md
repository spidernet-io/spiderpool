# IaaS Network Provider Architecture

**English** | [**简体中文**](./iaas-network-provider-zh_CN.md)

This document describes the design and internal mechanics of the Spiderpool IaaS Network Provider integration: the allocation and release call flow, pool placement modes, pool candidate selection rules, the HTTP timeout model, and the API contract that a provider must implement.

For installation and step-by-step usage, see [IaaS Network Provider](../usage/iaas-network-provider.md).

## How it works

When the feature is enabled, Spiderpool performs the following calls:

1. During Pod IP allocation, Spiderpool allocates IPs from Spiderpool IP pools first, then calls the IaaS Network Provider allocation API.
2. The IaaS Network Provider binds the IP on the cloud platform and returns the cloud-side network attributes.
3. Spiderpool writes the returned MAC address and VLAN ID into the allocation result, and the VLAN CNI pipeline uses them to configure the Pod interface.
4. During Pod IP release, Spiderpool calls the IaaS Network Provider release API for each address that should be released.
5. After the IaaS release call returns successfully, Spiderpool releases the IP from the internal IP pool. "Success" here means the IaaS Network Provider has accepted the release request and started the cloud-side cleanup. It does **not** guarantee that the IaaS-side IP resource is fully released, because the cloud platform may still be processing due to rate limits or asynchronous cleanup.

The IaaS Network Provider is an HTTP service. Spiderpool only defines the API contract and does not depend on a specific cloud vendor implementation.

## Pool placement modes

An IaaS-backed `SpiderIPPool` can operate in one of two placement modes:

- **Node-level pool** (default): the pool is pinned to a single node via `spec.nodeName`, and the provider prewarms IP resources on that node ahead of time. Allocation prefers prewarmed, ready-to-use addresses and skips the synchronous provider call for them.
- **Global pool**: the pool carries the `iaas-provider` marker but sets **no** `spec.nodeName`. One pool serves one Deployment (or similar workload) whose Pods spread across many nodes, so per-node prewarming does not apply. Instead, allocation works in realtime with a sticky sub-ENI cache.

The mode is derived solely from the pool shape and is fixed for the pool's lifetime: the validating webhook rejects adding or removing `spec.nodeName` on an IaaS pool after creation.

### Node-level pool prewarming

A node-level pool is strictly prewarm-only. The provider creates sub-ENIs on the pool's node ahead of time and publishes them into the pool's `status.ipMetaData`. At allocation time, the candidate set is the **intersection** of the pool's `spec.ips`-derived free addresses with the ready entries in `status.ipMetaData`; Spiderpool never allocates a node-level address that the cloud has not prepared. If no ready entry exists (for example, the pool was just created and the provider has not flushed metadata yet), the allocation fails with an IP-used-out error and kubelet retries until prewarmed addresses appear.

To let the provider learn the cloud-side parent port, the spiderpool-agent on the pool's node resolves the MAC address of the NIC named by the pool's `ipam.spidernet.io/parent-nic` annotation and publishes both name and MAC to the pool's `status.parentNic`. For a paired dual-stack pool set, only the primary (IPv4) pool carries `status.parentNic`.

### Global pool mode

In global mode:

1. When a Pod lands on a node where the pool already has an idle IP bound to that node (a cached sub-ENI), Spiderpool reuses it directly with **no** provider call — the Pod starts fast.
2. Otherwise Spiderpool picks a free address (preferring addresses whose sub-ENI does not exist yet over stealing an idle one from another node, to minimize cloud API calls) and calls the provider synchronously to create/attach the sub-ENI on the Pod's node.
3. When the Pod is deleted, the IP is released in Spiderpool but the cloud-side sub-ENI stays bound to the node as a cache for the next Pod there.
4. Reclaiming idle sub-ENIs when the pool usage crosses a watermark is the **provider's** responsibility. Before detaching an idle sub-ENI, the provider marks its metadata entry with `vlan: -1`; Spiderpool never allocates an entry in that state, which prevents a Pod from receiving an IP that is concurrently being unbound.
5. For dual-stack pairs, the IPv6 address chosen at sub-ENI creation stays sticky to that sub-ENI for its whole lifetime.

If the synchronous provider call fails during a global-pool allocation, Spiderpool rolls the just-claimed addresses back so the retry starts clean.

## Pool candidate class exclusivity

When a Pod interface's candidate pools mix different pool classes, Spiderpool keeps only the highest class and ignores the rest (with a warning log and a Pod warning event), so IaaS allocation never silently degrades to a lower class:

1. **Paired IaaS primary pool** (dual-stack enabled): pair allocation is pair-or-nothing; falling back to an unpaired pool would silently produce a single-stack Pod.
2. **IaaS pool** (node-level prewarm or global): its addresses are cloud sub-ENIs with provider-owned MAC/VLAN, incompatible with a static-pool fallback. Node-level and global pools share this class and may be mixed — node-level pools sort first, and global pools serve as the fallback.
3. **Plain (non-IaaS) pool**.

The class decision is made on the configured pool set before any per-node filtering, so behavior is deterministic on every node. If all pools of the kept class fail to allocate, the allocation fails instead of falling back to an ignored pool.

## HTTP request timeout model

`iaasNetworkProvider.httpRequestTimeout` controls how long Spiderpool waits for a single provider HTTP call (allocate or release) before treating it as failed.

### Provider timing model

A single provider request goes through two stages:

| Stage | Max duration | Description |
| --- | --- | --- |
| Rate-limit wait | 30 s | The provider checks its token bucket. If no slot is available it waits up to 30 s before accepting the request. |
| Cloud API call | 16 s | The provider forwards the request to the underlying cloud platform. Network latency and cloud-side processing can take up to 16 s. |
| **Worst-case total** | **~48 s** | Sum of the two stages plus a small network round-trip margin. |

Setting `httpRequestTimeout` shorter than ~48 s risks cancelling a request that the provider has already accepted and started executing on the cloud platform. This creates a state inconsistency: Spiderpool treats the call as a failure while the cloud operation may have succeeded or be in progress.

### Recommended values

| Scenario | Recommended `httpRequestTimeout` |
| --- | --- |
| Default / general use | `50s` (default) |
| Low-latency private cloud with no rate limiting | `20s` |
| High-contention environment with long rate-limit queues | `55s`–`59s` (must remain `< 100s`) |

### Validation rules

- Must be a valid Go duration string (e.g. `50s`, `1m`).
- Must be greater than `0`.
- Must be less than `2m` (static safety limit).
- Must be less than `100s` (the CNI plugin-to-agent timeout for ADD and DEL).
- Empty or unset defaults to `50s`.
- Validation failure is **fatal**: the agent and controller will not start with an invalid value.

### Time budget hierarchy

Understanding the full budget chain helps explain why `httpRequestTimeout` has the constraints it does:

| Layer | Default timeout | Description |
| --- | --- | --- |
| kubelet sandbox operation | **2 min** | kubelet's default timeout for the entire sandbox setup (Pod network setup). If the CNI pipeline does not complete within this window, the Pod fails to start. This is the outermost budget. |
| Spiderpool CNI plugin → agent call | **100 s** | The timeout the Spiderpool CNI binary uses when calling the spiderpool-agent over gRPC. This is the budget available to the agent to complete all IPAM and IaaS work before the CNI plugin gives up. |
| IaaS provider HTTP call | **50 s** (default) | The per-call timeout configured by `httpRequestTimeout`. Must fit inside the 100 s agent budget alongside all other IPAM work. |
| Provider budget check | provider-configured | The provider validates the caller's budget (sent via `X-Request-Timeout-Ms`) against its own configured rate-limit queue wait and cloud-transaction timeouts, and rejects requests whose budget cannot cover them. |

### Runtime behavior

For each provider HTTP call:

- Spiderpool derives a per-call context bounded by `httpRequestTimeout`. The effective HTTP deadline is `min(now + httpRequestTimeout, parent deadline)`.
- Spiderpool sends the effective remaining request budget in the `X-Request-Timeout-Ms` HTTP header. The value is a positive integer in milliseconds, calculated from the request context immediately before the HTTP request is sent.
- The provider is the single source of truth for its own limits: it compares the received budget against its configured rate-limit queue wait plus cloud-transaction timeout, and **rejects the request before consuming a rate-limit slot** if the budget is insufficient. This prevents mid-flight cancellation from leaving the cloud-side operation in an unknown state, and automatically tracks any provider-side configuration changes without requiring a matching Spiderpool setting.

### Error messages

| Message | Meaning | Suggested action |
| --- | --- | --- |
| provider rejects with a budget/rate-limit timeout error | The CNI pipeline consumed most of the budget before reaching the IaaS call, or the provider queue is saturated. | Check pipeline latency and provider load; consider raising the CNI timeout or the provider's rate-limit settings. |
| `provider-interaction timeout: ... exceeded configured timeout 50s` | The provider did not respond within `httpRequestTimeout`. | Check provider health; consider raising `httpRequestTimeout` if provider load is consistently high. |
| `parent budget exhausted: ... cancelled by parent context deadline` | The parent deadline arrived while the provider was responding. | Same as above; the parent budget ran out before the configured timeout. |

## API contract

The provider must implement the following HTTP APIs.

### Allocate IPs

The allocate API creates sub-ENIs. Each request item describes one sub-network-interface and carries the address families actually allocated for the workload NIC: IPv4-only, IPv6-only, or an IPv4/IPv6 pair provisioned atomically. Spiderpool passes the allocated families through as-is; any family requirement (for example, an IPv4 identity for the sub-ENI) is enforced by the provider.

#### Request

```text
POST /v1/apis/network.iaas.io/ipam/allocate-ips
Content-Type: application/json
X-Request-Timeout-Ms: 50000
```

Request headers:

| Header | Required | Description |
| --- | --- | --- |
| `X-Request-Timeout-Ms` | Yes | Remaining request budget in milliseconds. The provider should treat this as the maximum time available from when it receives the request, and should return before this budget is exhausted. |

Request body:

```json
{
  "podName": "example-pod",
  "podNamespace": "default",
  "podUID": "9f8b7c6d-xxxx-xxxx-xxxx-xxxxxxxxxxxx",
  "nodeName": "worker-1",
  "subEniRequests": [
    {
      "parentNicMac": "fa:16:3e:11:22:33",
      "subnet": "10.0.0.0/24",
      "ipv4Address": "10.0.0.10",
      "ipv6Address": "fd00::10",
      "ipv4PoolName": "example-pool-v4",
      "ipv6PoolName": "example-pool-v6"
    }
  ]
}
```

Fields:

| Field | Required | Description |
| --- | --- | --- |
| `podName` | No | Pod name. |
| `podNamespace` | No | Pod namespace. |
| `podUID` | No | Pod UID. |
| `nodeName` | Yes | Node where the Pod is scheduled. |
| `subEniRequests` | Yes | Sub-ENIs that Spiderpool expects the provider to create. Each item is one sub-ENI carrying the allocated address families. |
| `parentNicMac` | Yes | MAC address of the parent NIC that carries the Pod network. |
| `subnet` | Yes | Cloud subnet of the sub-ENI, identified by its IPv4 CIDR when IPv4 is allocated (shared by both families for dual-stack), otherwise by its IPv6 CIDR. |
| `ipv4Address` | No | IPv4 address without CIDR prefix. Empty for an IPv6-only allocation. |
| `ipv6Address` | No | IPv6 address without CIDR prefix, paired with `ipv4Address` on the same sub-ENI for dual-stack. Empty for an IPv4-only allocation. |
| `ipv4PoolName` | No | Name of the SpiderIPPool the IPv4 address was allocated from. The provider uses it to attribute the sub-ENI to a pool (for example global-pool ownership tagging and metadata flush) without a reverse `{subnet, ip}` lookup. |
| `ipv6PoolName` | No | Name of the SpiderIPPool the IPv6 address was allocated from. Same purpose as `ipv4PoolName`. |

#### Response

Any HTTP `2xx` status code is treated as success.

Response body:

```json
{
  "podName": "example-pod",
  "podNamespace": "default",
  "nodeName": "worker-1",
  "subEniResponses": [
    {
      "parentNicMac": "fa:16:3e:11:22:33",
      "subnet": "10.0.0.0/24",
      "ipv4Address": "10.0.0.10",
      "ipv6Address": "fd00::10",
      "macAddress": "fa:16:3e:aa:bb:cc",
      "vlanId": 100
    }
  ]
}
```

Fields:

| Field | Required | Description |
| --- | --- | --- |
| `subEniResponses` | Yes | Sub-ENI creation results returned by the provider. |
| `parentNicMac` | Yes | Parent NIC MAC used by the provider. |
| `subnet` | Yes | Subnet CIDR of the sub-ENI. |
| `ipv4Address` | No | IPv4 address bound by the provider. Empty for an IPv6-only sub-ENI. |
| `ipv6Address` | No | IPv6 address bound by the provider. Empty for an IPv4-only sub-ENI. |
| `macAddress` | No | MAC address of the sub-ENI, shared by both address families. |
| `vlanId` | No | VLAN ID assigned by the cloud platform, shared by both address families. |

If `macAddress` or `vlanId` is empty, Spiderpool keeps the original allocation result for that field. Otherwise every allocated family result of the sub-ENI takes the shared `macAddress`/`vlanId`.

### Release IP

Releasing either address of a dual-stack sub-ENI deletes the whole sub-ENI on the cloud side, so Spiderpool sends one release request per sub-ENI using its IPv4 address (or the IPv6 address for an IPv6-only sub-ENI); any paired address is released together.

#### Request

```text
POST /v1/apis/network.iaas.io/ipam/release-ip
Content-Type: application/json
X-Request-Timeout-Ms: 50000
```

Request headers:

| Header | Required | Description |
| --- | --- | --- |
| `X-Request-Timeout-Ms` | Yes | Remaining request budget in milliseconds. The provider should treat this as the maximum time available from when it receives the request, and should return before this budget is exhausted. |

Request body:

```json
{
  "podName": "example-pod",
  "podNamespace": "default",
  "podUID": "9f8b7c6d-xxxx-xxxx-xxxx-xxxxxxxxxxxx",
  "nodeName": "worker-1",
  "parentNicMac": "fa:16:3e:11:22:33",
  "subnet": "10.0.0.0/24",
  "ipAddress": "10.0.0.10",
  "poolName": "example-pool-v4"
}
```

Fields:

| Field | Required | Description |
| --- | --- | --- |
| `podName` | No | Pod name. |
| `podNamespace` | No | Pod namespace. |
| `podUID` | No | Pod UID. |
| `nodeName` | Yes | Node where the Pod was running. |
| `parentNicMac` | No | Parent NIC MAC. It may be empty in controller-side GC scenarios. |
| `subnet` | Yes | Subnet CIDR of the IP. |
| `ipAddress` | Yes | IP address to release. |
| `poolName` | No | Name of the SpiderIPPool the released IP belongs to. Same attribution purpose as `ipv4PoolName` in the allocation API. |

#### Response

The response body is ignored. Any HTTP `2xx` status code is treated as success.

## Special scenario handling

### Allocation must be synchronously successful

Currently, Spiderpool only continues to update the IP status in SpiderIPPool and create or update the SpiderEndpoint object after the Provider has completed the IaaS-side IP binding and returned the network configuration normally.

In some abnormal scenarios:

- If the Provider or cloud platform throttles the API and the processing takes a long time, causing Spiderpool to time out while waiting for the HTTP response, Spiderpool will treat this allocation as failed.
- If the Provider side fails to respond, Spiderpool will wait for the timeout period and then treat this allocation as failed.

If the spiderpool-agent does not receive a successful response from the Provider within the configured `httpRequestTimeout` (default `50s`), this allocation will be treated as a failure, and the Pod will be retried according to Kubernetes retry mechanisms.

### Release should be idempotent

The release API should be idempotent. If the IP has already been released or does not exist on the cloud platform, the provider should return a `2xx` status code when it is safe to consider the IP released.

This avoids repeated CNI DEL or GC retries causing unnecessary failures.

### Release may be eventually completed

Some cloud platforms release IaaS IP resources slowly due to cloud-side rate limits or asynchronous cleanup mechanisms. Therefore, IP release may not be fully completed immediately after the provider receives the release request.

Spiderpool requires the provider to accept the release request and start the cloud-side cleanup. The provider should return success when the release request is accepted or when the IP is already released.

Spiderpool calls the IaaS release API before releasing the IP from Spiderpool's internal IP pool. This order avoids re-allocating an IP in Spiderpool before the cloud platform has accepted the release request. If the cloud platform completes the cleanup asynchronously after that, it does not block Spiderpool's IP release flow.

### Parent NIC MAC lookup

Spiderpool passes `parentNicMac` when it can determine the parent NIC MAC address. In agent-side allocation and release, Spiderpool resolves the value through a chain: subnet cache → the node-level pool's agent-published `status.parentNic.mac` → the pool's `parent-nic` annotation name resolved via local netlink → the SpiderMultusConfig `master` interface as the final fallback.

In controller-side GC, Spiderpool may not run in the host network namespace of every node, so it may not be able to resolve the parent NIC MAC. In such cases, Spiderpool may send an empty `parentNicMac` during release. Provider implementations should tolerate this for the release API.

## Abnormal scenario handling

Spiderpool treats the following cases as failures:

- HTTP request failure.
- Non-`2xx` HTTP response status.
- Invalid allocation response JSON.
- Allocation response containing unknown IPs.

When release fails, Spiderpool may retry through later cleanup flows depending on where the release is triggered. Provider implementations should therefore make release operations safe to retry.
