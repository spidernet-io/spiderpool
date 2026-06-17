# Contract: Helm Values

## Values

Proposed values under the existing spiderpoolAgent section:

```yaml
spiderpoolAgent:
  networkResourcePlugin:
    enabled: false
    kubeletRootDir: /var/lib/kubelet
    devicePluginAffinity:
      nodeSelector: {}
    resourceAdvertisement:
      subENI:
        rules: []
      masterNIC:
        rules: []
```

Example with dynamic node controls:

```yaml
spiderpoolAgent:
  networkResourcePlugin:
    enabled: true
    kubeletRootDir: /var/lib/kubelet
    devicePluginAffinity:
      nodeSelector:
        matchExpressions:
        - key: spidernet.io/network-resource
          operator: NotIn
          values:
          - "disabled"
    resourceAdvertisement:
      subENI:
        rules:
        - resourceName: spidernet.io/sub-eni
          defaultMaxCount: 256
          nodeSelector:
            matchLabels:
              key: value
      masterNIC:
        rules:
        - nodeSelector:
            matchLabels:
              spidernet.io/nic-profile: eth
          defaultMaxCount: 10000
          includeInterfaces:
          - "eth*"
        - defaultMaxCount: 5000
          includeInterfaces:
          - "ens[0-9]"
          excludeInterfaces:
          - "ens4"
```

## Rendering Rules

- When `spiderpoolAgent.networkResourcePlugin.enabled=false`, spiderpool-agent does not mount kubelet plugin directories and does not register `spidernet.io/sub-eni` or `spidernet.io/<master>-nic`.
- When `spiderpoolAgent.networkResourcePlugin.enabled=true`, the chart renders configmap values consumed by spiderpool-agent and mounts `{kubeletRootDir}/device-plugins` and `{kubeletRootDir}/plugins_registry` as writable hostPath directories.
- Pod resource injection for network resources is controlled by the existing `spiderpoolController.podResourceInject.enabled` setting; no separate network resource plugin webhook switch is exposed.
- The chart must keep existing provider behavior. If provider integration is disabled, `resourceAdvertisement.subENI` remains inactive, but `resourceAdvertisement.masterNIC` may still advertise and inject master NIC resources.
- Nodes matching `devicePluginAffinity.nodeSelector` advertise Spiderpool network resources; nodes that do not match advertise none.
- `resourceAdvertisement.subENI.rules[].defaultMaxCount` defines the advertised sub-ENI capacity for enabled provider-mode nodes matching that entry.
- If a `resourceAdvertisement.subENI.rules[].nodeSelector` is non-empty, only matching nodes advertise that sub-ENI resource.
- Empty `resourceAdvertisement.masterNIC.rules` disables master NIC advertisement.
- If no `resourceAdvertisement.masterNIC.rules` match the local Node, no master NIC resources are advertised.
- If a `resourceAdvertisement.masterNIC.rules` entry omits `nodeSelector`, it matches all enabled nodes.
- `resourceAdvertisement.masterNIC.rules[].defaultMaxCount` defines the advertised virtual capacity for each matching master NIC and defaults to `10000`.
- `includeInterfaces` and `excludeInterfaces` use shell-style glob patterns; excluded interfaces are removed from the final set even when they also match include patterns.

## Validation Rules

- `resourceAdvertisement.subENI.rules[].resourceName` must be a valid extended resource name.
- `resourceAdvertisement.subENI.rules[].defaultMaxCount` must be an integer greater than or equal to zero.
- `resourceAdvertisement.subENI.rules[].nodeSelector` must follow Kubernetes label selector semantics.
- `resourceAdvertisement.masterNIC.rules[].defaultMaxCount` must be an integer greater than or equal to zero.
- `resourceAdvertisement.masterNIC.rules[].nodeSelector` must follow Kubernetes label selector semantics.
- `kubeletRootDir` must be an absolute path and defaults to `/var/lib/kubelet`.
- `devicePluginAffinity.nodeSelector` must follow Kubernetes label selector semantics.
- Interface name patterns must be valid shell-style glob patterns.

## Backward Compatibility

Defaults preserve current behavior: Spiderpool network resource advertisement and injection remain disabled unless the operator explicitly configures them.
