# IaaS Network Provider

**English** | [**简体中文**](./iaas-network-provider-zh_CN.md)

## Overview

Spiderpool can integrate with a generic IaaS Network Provider. When Spiderpool allocates or releases Pod IP addresses, it calls the configured provider to bind or unbind the corresponding IaaS-side IP resources on a cloud platform.

This feature is useful for public cloud or private cloud environments where an IP address assigned by Spiderpool must also be registered, bound, or programmed in an external cloud network system before the Pod can use it correctly.

Typical use cases include:

- Allocating auxiliary IP resources from a cloud platform.
- Binding an IP to a node, ENI, auxiliary network interface, VLAN sub-interface, or other cloud networking resource.
- Returning cloud-specific attributes such as Pod interface MAC address and VLAN ID to Spiderpool.
- Releasing the IaaS-side IP binding when Spiderpool releases the Pod IP.

## How it works

When the feature is enabled, Spiderpool performs the following calls:

1. During Pod IP allocation, Spiderpool allocates IPs from Spiderpool IP pools first, then calls the IaaS Network Provider allocation API.
2. The IaaS Network Provider binds the IP on the cloud platform and returns the cloud-side network attributes.
3. Spiderpool writes the returned MAC address and VLAN ID into the allocation result, and the VLAN CNI pipeline uses them to configure the Pod interface.
4. During Pod IP release, Spiderpool calls the IaaS Network Provider release API for each IPv4 address that should be released.
5. After the IaaS release call returns successfully, Spiderpool releases the IP from the internal IP pool. "Success" here means the IaaS Network Provider has accepted the release request and started the cloud-side cleanup. It does **not** guarantee that the IaaS-side IP resource is fully released, because the cloud platform may still be processing due to rate limits or asynchronous cleanup.

The IaaS Network Provider is an HTTP service. Spiderpool only defines the API contract and does not depend on a specific cloud vendor implementation.

## Usage

Configure the provider URL and HTTP timeout through Helm values:

```yaml
ipam:
  enableGatewayDetection: false
  enableIPConflictDetection: false
plugins:
  installVlanCNI: true
iaasNetworkProvider:
  serverUrl: "http://iaas-network-provider.iaas-network-provider-system.svc:80"
  httpRequestTimeout: "50s"
```

- If `iaasNetworkProvider.serverUrl` is empty, Spiderpool does not call the IaaS Network Provider.
- `plugins.installVlanCNI` must also be enabled.
- `ipam.enableGatewayDetection` and `ipam.enableIPConflictDetection` must be disabled. This mode is different from the traditional approach of calling CNI first and then calling IPAM. In this mode, IPAM must be called first to obtain the IaaS IP information before calling CNI to complete the Pod network configuration. Therefore, gateway detection and IP conflict detection cannot work in this mode.

### Configure the HTTP request timeout

`iaasNetworkProvider.httpRequestTimeout` controls how long Spiderpool waits for a single provider HTTP call (allocate or release) before treating it as failed.

#### Provider timing model

A single provider request goes through two stages:

| Stage | Max duration | Description |
| --- | --- | --- |
| Rate-limit wait | 30 s | The provider checks its token bucket. If no slot is available it waits up to 30 s before accepting the request. |
| Cloud API call | 16 s | The provider forwards the request to the underlying cloud platform. Network latency and cloud-side processing can take up to 16 s. |
| **Worst-case total** | **~48 s** | Sum of the two stages plus a small network round-trip margin. |

Setting `httpRequestTimeout` shorter than ~48 s risks cancelling a request that the provider has already accepted and started executing on the cloud platform. This creates a state inconsistency: Spiderpool treats the call as a failure while the cloud operation may have succeeded or be in progress.

#### Recommended values

| Scenario | Recommended `httpRequestTimeout` |
| --- | --- |
| Default / general use | `50s` (default) |
| Low-latency private cloud with no rate limiting | `20s` |
| High-contention environment with long rate-limit queues | `55s`–`59s` (must remain `< 100s`) |

#### Validation rules

- Must be a valid Go duration string (e.g. `50s`, `1m`).
- Must be greater than `0`.
- Must be less than `2m` (static safety limit).
- Must be less than `100s` (the CNI plugin-to-agent timeout for ADD and DEL).
- Empty or unset defaults to `50s`.
- Validation failure is **fatal**: the agent and controller will not start with an invalid value.

#### Time budget hierarchy

Understanding the full budget chain helps explain why `httpRequestTimeout` has the constraints it does:

| Layer | Default timeout | Description |
| --- | --- | --- |
| kubelet sandbox operation | **2 min** | kubelet's default timeout for the entire sandbox setup (Pod network setup). If the CNI pipeline does not complete within this window, the Pod fails to start. This is the outermost budget. |
| Spiderpool CNI plugin → agent call | **100 s** | The timeout the Spiderpool CNI binary uses when calling the spiderpool-agent over gRPC. This is the budget available to the agent to complete all IPAM and IaaS work before the CNI plugin gives up. |
| IaaS provider HTTP call | **50 s** (default) | The per-call timeout configured by `httpRequestTimeout`. Must fit inside the 100 s agent budget alongside all other IPAM work. |
| Provider worst-case completion | **~48 s** | The maximum time a single provider request can take (30 s rate-limit wait + 16 s cloud API). This is the minimum meaningful value for `httpRequestTimeout`. |

#### Runtime behavior

Before sending each provider HTTP call, Spiderpool checks how much time remains in the parent CNI operation context (the 100 s agent budget):

- If the remaining time is **less than the provider worst-case** (~48 s), Spiderpool **does not start the call** and returns a `parent budget insufficient` error immediately. This prevents the provider from consuming a rate-limit slot for a call that cannot complete, which would leave the cloud-side operation in an unknown state.
- If the remaining time is sufficient, Spiderpool derives a per-call context bounded by `httpRequestTimeout`. The effective HTTP deadline is `min(now + httpRequestTimeout, parent deadline)`.

#### Error messages

| Message | Meaning | Suggested action |
| --- | --- | --- |
| `parent budget insufficient: Xs remaining is less than provider worst-case 48s` | The CNI pipeline consumed most of the budget before reaching the IaaS call. | Check pipeline latency; consider raising the CNI timeout or reducing `httpRequestTimeout`. |
| `provider-interaction timeout: ... exceeded configured timeout 50s` | The provider did not respond within `httpRequestTimeout`. | Check provider health; consider raising `httpRequestTimeout` if provider load is consistently high. |
| `parent budget exhausted: ... cancelled by parent context deadline` | The parent deadline arrived while the provider was responding. | Same as above; the parent budget ran out before the configured timeout. |

> **Note**: [VLAN-CNI](https://github.com/spidernet-io/vlan-cni) is a VLAN CNI plugin developed by Spiderpool based on the upstream community cni-plugin project. It can be used to integrate with third-party cloud platform IaaS Network Providers, allocating IaaS-layer VLAN network interfaces for containers.

The URL must include the scheme, host, and port. Spiderpool appends the fixed API paths to this base URL.

### Verify the feature is enabled

After installation, you can verify whether the feature is active by:

1. **Check the ConfigMap**

   ```bash
   kubectl get configmap spiderpool-conf -n <spiderpool-namespace> -o yaml | grep iaasNetworkProvider
   ```

   If the output includes `iaasNetworkProvider.serverUrl` and the value is non-empty, the feature is enabled.

2. **Check agent startup logs**

   ```bash
   kubectl logs spiderpool-agent-xxx -n <spiderpool-namespace>
   ```

   Search for `IaaS client created successfully` in the agent startup logs. If you see this log, the agent has successfully initialized the IaaS client and the feature is active. If you see `IaaS provider configuration validation failed`, there is a configuration issue; verify that the `serverUrl` format is correct.

### Configure VLAN CNI

When integrating with the IaaS Network Provider, you must use VLAN CNI to create VLAN sub-interfaces for Pods, and configure the VLAN ID and MAC address allocated by the cloud platform on those sub-interfaces. This ensures that the VLAN sub-interface configuration is consistent with the cloud platform, enabling normal network communication.

If the VLAN ID is manually configured at this point, it will be inconsistent with the VLAN ID allocated by the cloud platform, leading to network communication anomalies. Therefore, **do not set `vlanID` in the `vlan` configuration of SpiderMultusConfig**; otherwise [vlan-cni](https://github.com/spidernet-io/vlan-cni) will be unable to create a correctly configured VLAN sub-interface for the Pod.

> [vlan-cni](https://github.com/spidernet-io/vlan-cni) queries the local spiderpool-agent via a Unix socket during Pod creation to obtain the VLAN ID and MAC address allocated from the IaaS, and then creates the VLAN sub-interface in the Pod network namespace based on this information.

In addition, platform administrators need to prepare the IaaS side in advance:

- Create a VPC subnet and bind it to the elastic network interface. For example, bind the VPC subnet `172.91.0.0/24` to the network interface `enp0s28` on node `ECS-01`.

Then create the corresponding SpiderMultusConfig and SpiderIPPool resources on the PaaS side:

Example configuration:

```yaml
apiVersion: spiderpool.spidernet.io/v2beta1
kind: SpiderMultusConfig
metadata:
  name: iaas-vlan-config
  namespace: spiderpool
spec:
  cniType: vlan
  vlan:
    master:
      - enp0s28
    ippools:
      ipv4:
        - pool-enp0s28
---
apiVersion: spiderpool.spidernet.io/v2beta1
kind: SpiderIPPool
metadata:
  name: pool-enp0s28
spec:
  gateway: 172.91.0.1
  ips:
    - 172.91.0.100-172.91.0.120
  subnet: 172.91.0.0/24
```

- `master` is a required field. It must match the physical network interface name on the node, and the interface name must be consistent across all nodes.
- `subnet` is a required field. It must match the VPC subnet on the cloud platform.

## API contract

The provider must implement the following HTTP APIs.

### Allocate IPs

#### Request

```text
POST /v1/apis/network.iaas.io/ipam/allocate-ips
Content-Type: application/json
```

Request body:

```json
{
  "podName": "example-pod",
  "podNamespace": "default",
  "podUID": "9f8b7c6d-xxxx-xxxx-xxxx-xxxxxxxxxxxx",
  "nodeName": "worker-1",
  "iaasIPsAllocationRequest": [
    {
      "ipAddress": "10.0.0.10",
      "subnet": "10.0.0.0/24",
      "parentNicMac": "fa:16:3e:11:22:33"
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
| `iaasIPsAllocationRequest` | Yes | IPs that Spiderpool has allocated and expects the provider to bind. |
| `ipAddress` | Yes | IP address without CIDR prefix. |
| `subnet` | Yes | Subnet CIDR of the IP. |
| `parentNicMac` | Yes | MAC address of the parent NIC that carries the Pod network. |

#### Response

Any HTTP `2xx` status code is treated as success.

Response body:

```json
{
  "podName": "example-pod",
  "podNamespace": "default",
  "nodeName": "worker-1",
  "iaasIPsAllocationResponse": [
    {
      "parentNicMac": "fa:16:3e:11:22:33",
      "subnet": "10.0.0.0/24",
      "ipAddress": "10.0.0.10",
      "macAddress": "fa:16:3e:aa:bb:cc",
      "vlanId": 100
    }
  ]
}
```

Fields:

| Field | Required | Description |
| --- | --- | --- |
| `iaasIPsAllocationResponse` | Yes | Allocation results returned by the provider. |
| `parentNicMac` | Yes | Parent NIC MAC used by the provider. |
| `subnet` | Yes | Subnet CIDR of the IP. |
| `ipAddress` | Yes | IP address that was bound by the provider. |
| `macAddress` | No | MAC address assigned by the cloud platform for the Pod interface. |
| `vlanId` | No | VLAN ID assigned by the cloud platform. |

If `macAddress` or `vlanId` is empty, Spiderpool keeps the original allocation result for that field.

### Release IP

#### Request

```text
POST /v1/apis/network.iaas.io/ipam/release-ip
Content-Type: application/json
```

Request body:

```json
{
  "podName": "example-pod",
  "podNamespace": "default",
  "podUID": "9f8b7c6d-xxxx-xxxx-xxxx-xxxxxxxxxxxx",
  "nodeName": "worker-1",
  "parentNicMac": "fa:16:3e:11:22:33",
  "subnet": "10.0.0.0/24",
  "ipAddress": "10.0.0.10"
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

Spiderpool passes `parentNicMac` when it can determine the parent NIC MAC address. In agent-side allocation and release, Spiderpool can usually resolve the value from the runtime network environment or cache.

In controller-side GC, Spiderpool may not run in the host network namespace of every node, so it may not be able to resolve the parent NIC MAC. In such cases, Spiderpool may send an empty `parentNicMac` during release. Provider implementations should tolerate this for the release API.

## Abnormal scenario handling

Spiderpool treats the following cases as failures:

- HTTP request failure.
- Non-`2xx` HTTP response status.
- Invalid allocation response JSON.
- Allocation response containing unknown IPs.

When release fails, Spiderpool may retry through later cleanup flows depending on where the release is triggered. Provider implementations should therefore make release operations safe to retry.
