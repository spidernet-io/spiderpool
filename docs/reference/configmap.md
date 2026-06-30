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
    agent:
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
- `agent.networkResourcePlugin` (object): Spiderpool agent network resource plugin configuration rendered from Helm `spiderpoolAgent.networkResourcePlugin`.
  - `enabled` (bool): Enable or disable spiderpool-agent network resource advertisement.
  - `kubeletRootDir` (string): Kubelet root directory used to derive the agent's `device-plugins` and `plugins_registry` hostPath mounts. The default is `/var/lib/kubelet`.
  - `devicePluginAffinity.nodeSelector` (object): Kubernetes label selector for nodes that advertise Spiderpool network resources. Empty selector matches all nodes. Use `matchLabels` and `matchExpressions` operators such as `In`, `NotIn`, `Exists`, and `DoesNotExist` to include or exclude nodes.
  - `resourceAdvertisement.subENI` (object): Auxiliary ENI slot advertisement configuration. `rules` contains resource advertisement rules. Empty rules disable Sub-ENI advertisement. `defaultMaxCount` is the default total schedulable slot capacity, and `nodeSelector` is an optional Kubernetes label selector for nodes that advertise each sub-ENI resource.
  - `resourceAdvertisement.masterNIC` (object): Physical master NIC advertisement configuration. Empty `rules` disable master NIC advertisement. `defaultMaxCount` is the virtual capacity advertised for each selected master NIC and defaults to `10000`; `nodeSelector` is an optional Kubernetes label selector for nodes that advertise each master NIC resource.

Pod resource injection is controlled by `podResourceInject.enabled`. When enabled, the Pod webhook checks existing Multus annotations for VLAN `SpiderMultusConfig` references whose `vlanID` is unset. The injected resource quantity equals the number of eligible references.
