# Contract: Helm Values

## Values

Proposed values under the existing provider integration section:

```yaml
iaasNetworkProvider:
  serverUrl: ""
  eniDevPlugin:
    enabled: false
    resourceName: spidernet.io/sub-eni
    maxSlotsPerNode: 0
    kubeletRootDir: /var/lib/kubelet
    injectPodENIResources: true
```

## Rendering Rules

- When `eniDevPlugin.enabled=false`, spiderpool-agent does not mount kubelet plugin directories and does not register `spidernet.io/sub-eni`.
- When `eniDevPlugin.enabled=true`, the chart renders configmap values consumed by spiderpool-agent.
- When `serverUrl` is set and `eniDevPlugin.enabled=true`, the spiderpool-agent DaemonSet mounts `{kubeletRootDir}/device-plugins` and `{kubeletRootDir}/plugins_registry` as writable hostPath directories.
- The chart must keep the existing provider `serverUrl` behavior. If `serverUrl` is empty, provider integration is disabled and the ENI slot device plugin must stay inactive.
- `injectPodENIResources` only controls automatic ENI slot resource injection. It must not affect existing RDMA or network resource injection behavior.

## Validation Rules

- `resourceName` must be a valid extended resource name.
- `maxSlotsPerNode` must be an integer greater than or equal to zero.
- `kubeletRootDir` must be an absolute path and defaults to `/var/lib/kubelet`.

## Backward Compatibility

Defaults preserve current behavior: provider mode and the ENI slot device plugin remain disabled unless the operator explicitly configures them.
