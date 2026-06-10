# Configuration

> Instructions for global configuration and environment arguments of Spiderpool.

## Configmap Configuration

Configmap "spiderpool-conf" is the global configuration of Spiderpool.

```yaml
apiVersion: v1
kind: ConfigMap
metadata:
  name: spiderpool-conf
  namespace: kube-system
data:
  conf.yml: |
    ipamUnixSocketPath: /var/run/spidernet/spiderpool.sock
    enableIPv4: true
    enableIPv6: true
    enableStatefulSet: true
    enableKubevirtStaticIP: true
    enableSpiderSubnet: true
    enableIPConflictDetection: true
    enableGatewayDetection: true
    clusterSubnetDefaultFlexibleIPNumber: 1
    tuneSysctlConfig: {{ .Values.spiderpoolAgent.tuneSysctlConfig }}
    podResourceInject:
      enabled: false
      namespacesExclude: 
      - kube-system
      - spiderpool
      namespacesInclude: []
    iaasNetworkProvider:
      serverUrl: ""
      eniDevPlugin:
        enabled: false
        resourceName: spidernet.io/eni-slot
        maxSlotsPerNode: 0
        kubeletRootDir: /var/lib/kubelet
        injectPodENIResources: true
```

- `ipamUnixSocketPath` (string): Spiderpool agent listens to this UNIX socket file and handles IPAM requests from IPAM plugin.
- `enableIPv4` (bool):
  - `true`: Enable IPv4 IP allocation capability of Spiderpool.
  - `false`: Disable IPv4 IP allocation capability of Spiderpool.
- `enableIPv6` (bool):
  - `true`: Enable IPv6 IP allocation capability of Spiderpool.
  - `false`: Disable IPv6 IP allocation capability of Spiderpool.
- `enableStatefulSet` (bool):
  - `true`: Enable StatefulSet static IP capability of Spiderpool.
  - `false`: Disable StatefulSet static IP capability of Spiderpool.
- `enableKubevirtStaticIP` (bool):
  - `true`: Enable kubevirt VM static IP capability of Spiderpool.
  - `false`: Disable kubevirt VM static IP capability of Spiderpool.
- `enableSpiderSubnet` (bool):
  - `true`: Enable SpiderSubnet capability of Spiderpool.
  - `false`: Disable SpiderSubnet capability of Spiderpool.
- `enableIPConflictDetection` (bool):
  - `true`: Enable IP conflict detection capability of Spiderpool.
  - `false`: Disable IP conflict detection capability of Spiderpool.
- `enableGatewayDetection` (bool):
  - `true`: Enable gateway detection capability of Spiderpool.
  - `false`: Disable gateway detection capability of Spiderpool.
- `clusterSubnetDefaultFlexibleIPNumber` (int): Global SpiderSubnet default flexible IP number. It takes effect across the cluster.
- `podResourceInject` (object): Pod resource inject capability of Spiderpool.
  - `enabled` (bool):
    - `true`: Enable pod resource inject capability of Spiderpool.
    - `false`: Disable pod resource inject capability of Spiderpool.
  - `namespacesExclude` (array): Exclude the namespaces of the pod resource inject.
  - `namespacesInclude` (array): Include the namespaces of the pod resource inject.
- `iaasNetworkProvider` (object): IaaS Network Provider integration configuration.
  - `serverUrl` (string): Base URL for the provider HTTP API. If empty, provider mode is disabled.
  - `eniDevPlugin` (object): Auxiliary ENI slot device plugin configuration.
    - `enabled` (bool): Enable or disable the spiderpool-agent device plugin for ENI slot scheduling.
    - `resourceName` (string): Extended resource name advertised to kubelet. The default is `spidernet.io/eni-slot`.
    - `maxSlotsPerNode` (int): Total auxiliary ENI slot capacity advertised on each node. The default `0` advertises no schedulable slots, so Pods requesting `spidernet.io/eni-slot` will remain unschedulable until a positive capacity is configured.
    - `kubeletRootDir` (string): Kubelet root directory used to derive the agent's `device-plugins` and `plugins_registry` hostPath mounts. The default is `/var/lib/kubelet`. Kubernetes v1.13 changed the external plugin registration directory from `{kubeletRootDir}/plugins/` to `{kubeletRootDir}/plugins_registry/`; the device plugin v1beta1 API still exposes the historical kubelet registration socket under `{kubeletRootDir}/device-plugins/kubelet.sock`, so Spiderpool mounts both derived directories for compatibility.
    - `injectPodENIResources` (bool): Enable webhook injection of ENI slot resource requests for eligible provider-mode VLAN Pods. The default is `true`; when `false`, users must declare `spidernet.io/eni-slot` manually on Pods that need scheduling protection.

When `injectPodENIResources` is true, the Pod webhook checks existing Multus annotations for VLAN `SpiderMultusConfig` references whose `vlanID` is unset. The injected resource quantity equals the number of eligible references.
