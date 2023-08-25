# Meta plugin: Coordinator Configuration

Spiderpool provides a CNI meta-plugin called 'coordinator', which works after the main CNI is called, and provides the following main features:

- Fixed an issue where Underlay pods could not access ClusterIP
- Tune the pod's routing to ensure that packets are routed consistently while a pod is in multi-NIC
- Supports detecting if the IP of a pod is in conflict
- Supports detecting if the gateway of a pod is reachable
- Support for fixing MAC address prefixes for pods

## CNI fields description

This is a complete manifest file for the coordinator's multus network-attachment-definition:

> 'Spidercoordinators default CR' is the global default configuration (all fields) for the 'coordinator' plugin, which has a lower priority than the configuration in NetworkAttachmentDefinition CR. If NetworkAttachmentDefinition CR is not configured, 'Spidercoordinators CR' is used as the default. For more details, see: [Spidercoordinator](../reference/crd-spidercoordinator.md)


| Field     | Description                                       | Schema | Validation | Default |
|-----------|---------------------------------------------------|--------|------------|---------|
| type      | The name of this Spidercoordinators resource      | string | required   |coordinator     |
| mode      | the mode in which the coordinator run. "auto": Automatically determine if it's overlay or underlay; "underlay": All NICs for pods are underlay NICs, and in this case the coordinator will create veth-pairs device to solve the problem of underlay pods accessing services; "overlay": The coordinator does not create veth-pair devices, but the first NIC of the pod cannot be an underlay NIC, which is created by overlay CNI (e.g. calico, cilium). Solve the problem of pod access to service through the first NIC; "disable": The coordinator does nothing and exits directly            | string | optional   | auto |
| tunePodRoutes | Tune the pod's routing tables while a pod is in multi-NIC mode | bool | optional | true |
| podDefaultRouteNic | Configure the default routed NIC for the pod while a pod is in multi-NIC mode | string | optional | "" |
| podDefaultCniNic | The name of the pod's first NIC defaults to eth0 in kubernetes | bool | optional | eth0 |
| detectGateway | Enable gateway detection while creating pods, which prevent pod creation if the gateway is unreachable | bool | optional | false |
| detectIPConflict | Enable IP conflicting checking for pods, which prevent pod creation if the pod's ip is conflicting | bool | optional | false |
| podMACPrefix | Enable fixing MAC address prefixes for pods. empty value is mean to disable | string | optional | "" |
| overlayPodCIDR | The default cluster CIDR for the cluster. It doesn't need to be configured, and it collected automatically by SpiderCoordinator | []stirng | optional | []string{} |
| serviceCIDR | The default service CIDR for the cluster. It doesn't need to be configured, and it collected automatically by SpiderCoordinator | []stirng | optional | []string{} |
| hijackCIDR | The CIDR that need to be forwarded via the host network, For example, the address of nodelocaldns(169.254.20.10/32 by default) | []stirng | optional | []string{} |
| hostRuleTable | The routes on the host that communicates with the pod's underlay IPs will belong to this routing table number | int | optional | 500 |
| hostRPFilter | Set the rp_filter sysctl parameter on the host, which is recommended to be set to 0 | int | optional | 0 |
| detectOptions | The advanced configuration of detectGateway and detectIPConflict, including retry numbers(default is 3), interval(default is 1s) and timeout(default is 1s) | obejct | optional | nil |
| logOptions | The configuration of logging, including logLevel(default is debug) and logFile(default is /var/log/spidernet/coordinator.log) |  obejct | optional | nil |

## Configure Examples

- Supports detecting if the IP of a pod is in conflict

```yaml
apiVersion: k8s.cni.cncf.io/v1
kind: NetworkAttachmentDefinition
metadata:
  name: coordinator-demo
  namespace: kube-system
spec:
  config: |-
    {
        "cniVersion": "0.3.1",
        "name": "coordinator",
        "plugins": [
            {
                "type": "macvlan",
                "master": "ens160",
                "mode": "bridge",
                "ipam": {
                    "type": "spiderpool"
                }
            },{
                "type": "coordinator",
                "mode": "auto",
                "detectIPConflict": false
            }
        ]
    }
```

- Supports detecting if the IP of a pod is in conflict

```yaml
apiVersion: k8s.cni.cncf.io/v1
kind: NetworkAttachmentDefinition
metadata:
  name: coordinator-demo
  namespace: kube-system
spec:
  config: |-
    {
        "cniVersion": "0.3.1",
        "name": "coordinator",
        "plugins": [
            {
                "type": "macvlan",
                "master": "ens160",
                "mode": "bridge",
                "ipam": {
                    "type": "spiderpool"
                }
            },{
                "type": "coordinator",
                "mode": "auto",
                "detectIPConflict": true
            }
        ]
    }
```

- Supports detecting if the gateway of a pod is reachable

```yaml
apiVersion: k8s.cni.cncf.io/v1
kind: NetworkAttachmentDefinition
metadata:
  name: coordinator-demo
  namespace: kube-system
spec:
  config: |-
    {
        "cniVersion": "0.3.1",
        "name": "coordinator",
        "plugins": [
            {
                "type": "macvlan",
                "master": "ens160",
                "mode": "bridge",
                "ipam": {
                    "type": "spiderpool"
                }
            },{
                "type": "coordinator",
                "detectGateway": true
            }
        ]
    }
```

- Support for fixing MAC address prefixes for pods

```yaml
apiVersion: k8s.cni.cncf.io/v1
kind: NetworkAttachmentDefinition
metadata:
  name: coordinator-demo
  namespace: kube-system
spec:
  config: |-
    {
        "cniVersion": "0.3.1",
        "name": "coordinator",
        "plugins": [
            {
                "type": "macvlan",
                "master": "ens160",
                "mode": "bridge",
                "ipam": {
                    "type": "spiderpool"
                }
            },{
                "type": "coordinator",
                "podMACPrefix": "0a:1b"
            }
        ]
    }
```

- Setting pod's default route NIC while pod in Multi-NIC mode

```yaml
apiVersion: k8s.cni.cncf.io/v1
kind: NetworkAttachmentDefinition
metadata:
  name: coordinator-demo
  namespace: kube-system
spec:
  config: |-
    {
        "cniVersion": "0.3.1",
        "name": "coordinator",
        "plugins": [
            {
                "type": "macvlan",
                "master": "ens160",
                "mode": "bridge",
                "ipam": {
                    "type": "spiderpool"
                }
            },{
                "type": "coordinator",
                "podDefaultRouteNic": "eth0"
            }
        ]
    }
```

> You can also set it by `ipam.spidernet.io/default-route-nic: eth0` in the pod's annotations.

- Configure the subnets that need to be forwarded via the host network

```yaml
apiVersion: k8s.cni.cncf.io/v1
kind: NetworkAttachmentDefinition
metadata:
  name: coordinator-demo
  namespace: kube-system
spec:
  config: |-
    {
        "cniVersion": "0.3.1",
        "name": "coordinator",
        "plugins": [
            {
                "type": "macvlan",
                "master": "ens160",
                "mode": "bridge",
                "ipam": {
                    "type": "spiderpool"
                }
            },{
                "type": "coordinator",
                "hijackCIDR": ["169.254.20.10/32"]
            }
        ]
    }
```

> 169.254.20.10/32 is default ip address of nodelocaldns.
