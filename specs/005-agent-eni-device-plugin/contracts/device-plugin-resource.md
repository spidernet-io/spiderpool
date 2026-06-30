# Contract: Device Plugin Resource

## Resource Name

Default scheduler-facing resource name:

```text
spidernet.io/sub-eni
```

The name must satisfy Kubernetes extended resource naming rules: `<domain>/<resource>`.

Master NIC scheduler-facing resource names:

```text
spidernet.io/<master>-nic
```

`<master>` is the selected physical master NIC interface name. Each selected master NIC resource is advertised with the matching rule's `defaultMaxCount`, defaulting to `10000`.

## Node Status Semantics

`Node.status.capacity[spidernet.io/sub-eni]` and `Node.status.allocatable[spidernet.io/sub-eni]` represent device plugin advertised capacity according to Kubernetes device plugin behavior.

`Node.status.allocatable[spidernet.io/sub-eni]` is the current healthy schedulable total capacity. It is not a remaining/free count and must not be decremented by Spiderpool after each Pod allocation.

`Node.status.allocatable[spidernet.io/<master>-nic]` represents selected physical master NIC presence with configurable virtual quantity. It is not intended to model bandwidth or queue count.

## Device Plugin RPC Behavior

- The agent starts a gRPC device plugin endpoint under the selected kubelet plugin path.
- Path selection must prefer `{kubeletRootDir}/plugins_registry` when present and fall back to `{kubeletRootDir}/device-plugins` when the preferred directory is absent.
- The agent registers the configured resource name with kubelet after the device plugin service is listening.
- The agent registers or updates master NIC resources for selected physical NICs on enabled nodes.
- `ListAndWatch` reports one healthy device per configured ENI slot.
- `ListAndWatch` reports the desired master NIC resource device list with the configured virtual quantity for selected physical NIC resources.
- If a slot becomes unavailable, `ListAndWatch` reports it unhealthy or removes it according to Kubernetes device plugin expectations.
- `Allocate` succeeds only for slot IDs known to the device plugin and returns the container runtime configuration required by Spiderpool's implementation. If no runtime change is required, it returns an empty successful response.
- When local Node labels change, the agent recomputes desired resources from its local Node cache and notifies kubelet only if the computed device list changes.

## Restart Contract

- On kubelet restart, the plugin detects socket removal or registration failure and re-registers.
- Before re-registration, kubelet may temporarily advertise no `spidernet.io/sub-eni` capacity.
- Previously assigned Pod-device mappings are recovered by kubelet from the device manager checkpoint.
- New Pods requesting `spidernet.io/sub-eni` are not schedulable until kubelet advertises the resource again.

## Failure Contract

- If provider mode is disabled, the resource is not registered.
- If configured maximum is zero, zero slots are advertised.
- If the local Node does not match `devicePluginAffinity.nodeSelector`, no Spiderpool network resources are advertised.
- If `resourceAdvertisement.subENI.rules[].defaultMaxCount` is invalid, configuration validation must reject it before advertising an unsafe capacity.
- If the plugin cannot register, it must log an operator-visible error and keep retrying.
- If the preferred plugin registration path is absent, it must log the fallback to the derived device-plugin path.
- If `Allocate` receives an unknown slot ID, it must fail the request and emit diagnostics.
