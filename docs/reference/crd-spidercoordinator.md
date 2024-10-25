# Spidercoordinator

A Spidercoordinator resource represents the global default configuration of the cni meta-plugin: coordinator.

> There is only one instance of this resource, which is automatically generated while you install Spiderpool and does not need to be created manually.

## Sample YAML

```yaml
apiVersion: spiderpool.spidernet.io/v2beta1
kind: SpiderCoordinator
metadata:
  name: default
spec:
  detectGateway: false
  detectIPConflict: false
  enableVethLinkLocalAddress: false
  hostRPFilter: 0
  podRPFilter: 0
  hostRuleTable: 500
  mode: underlay
  podCIDRType: auto
  podDefaultRouteNIC: eth0
  podMACPrefix: ""
  tunePodRoutes: true
status:
  overlayPodCIDR:
  - 10.233.64.0/18
  - fd85:ee78:d8a6:8607::1:0000/112
  phase: Synced
  serviceCIDR:
  - 10.233.0.0/18
  - fd85:ee78:d8a6:8607::1000/116
```

## Spidercoordinators definition

### Metadata

| Field     | Description                                       | Schema | Validation |
|-----------|---------------------------------------------------|--------|------------|
| name      | The name of this Spidercoordinators resource      | string | required   |

### Spec

This is the Spidercoordinators spec for users to configure.

| Field              | Description                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                              | Schema  | Validation | Values                                        | Default                      |
|--------------------|------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------|---------|------------|-----------------------------------------------|------------------------------|
| mode               | The mode in which the coordinator. auto: automatically determine if it's overlay or underlay. underlay: coordinator creates veth devices to solve the problem that CNIs such as macvlan cannot communicate with clusterIP. overlay: fix the problem that CNIs such as Macvlan cannot access ClusterIP through the Calico network card attached to the pod,coordinate policy route between interfaces to ensure consistence data path of request and reply packets                                                                                                                                                                                                                                                                        | string  | require    | auto,underlay,overlay                         | auto                         |
| podCIDRType        | The ways to fetch the CIDR of the cluster. auto(default), This means that it will automatically switch podCIDRType to cluster or calico or cilium. based on cluster CNI. calico: auto fetch the subnet of the pod from the ip pools of calico, This only works if the cluster CNI is calico; cilium: Auto fetch the pod's subnet from cilium's configMap or ip pools. Supported IPAM modes: ["cluster-pool","kubernetes","multi-pool"]; cluster: auto fetch the subnet of the pod from the kubeadm-config configmap, This is useful if there is only a globally unique default pod's subnet; none: don't get the subnet of the pod, which is useful for some special cases. In this case,you can manually configure the hijackCIDR field | string  | require    | auto,cluster,calico,cilium,none               | auto                         |
| tunePodRoutes      | tune pod's route while the pod is attached to multiple NICs                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                              | bool    | optional   | true,false                                    | true                         |
| podDefaultRouteNIC | The NIC where the pod's default route resides                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                            | string  | optional   | "",eth0,net1...                               | underlay: eth0,overlay: net1 |
| detectGateway      | enable detect gateway while launching pod, If the gateway is unreachable, pod will be failed to created; Note: We use ARP probes to detect if the gateway is reachable, and some gateway routers may warn about this                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                     | boolean | optional   | true,false                                    | false                        |
| detectIPConflict   | enable the pod's ip if is conflicting while launching pod. If an IP conflict of the pod is detected, pod will be failed to created                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                       | boolean | optional   | true,false                                    | false                        |
|  vethLinkAddress  |               configure an link-local address for veth0 device, fix the istio case                                                                                        | boolean | optional   | true,false                                    | false                        |
| podMACPrefix       | fix the pod's mac address with this prefix + 4 bytes IP                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                  | string  | optional   | a invalid mac address prefix                  | ""                           |
| podRPFilter       | set rp_filter sysctl for the pod                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                               | int     | required   | 0,1,2;suggest to be 0                         | 0                            |
| hostRPFilter       | (deprecated) set rp_filter sysctl for the node                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                               | int     | required   | 0,1,2;suggest to be 0                         | 0                            |
| hostRuleTable      | The directly routing table of the host accessing the pod's underlay IP will be placed in this policy routing table                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                       | int     | required   | int                                           | 500                          |

### Status (subresource)

The Spidercoordinators status is a subresource that processed automatically by the system to summarize the current state.

| Field               | Description                                        | Schema                                                 | Validation |
|---------------------|----------------------------------------------------|--------------------------------------------------------|------------|
| overlayPodCIDR      | the cluster pod cidr                               |    []string                                            | required   |
| serviceCIDR         | the cluster service cidr                           |    []string                                            | required   |
| phase               | Represents the status of synchronization           |    string                                              | required   |
| reason              | the reason why the status is NotReady              |    string                                              | optional   |
