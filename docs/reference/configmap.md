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
    clusterSubnetDefaultFlexibleIPNumber: 1
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
- `clusterSubnetDefaultFlexibleIPNumber` (int): Global SpiderSubnet default flexible IP number. It takes effect across the cluster.
