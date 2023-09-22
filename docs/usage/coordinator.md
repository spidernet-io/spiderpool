# Coordinator

**English** | [**简体中文**](./coordinator-zh_CN.md)

Spiderpool incorporates a CNI meta-plugin called `coordinator` that works after the Main CNI is invoked. It mainly offers the following features:

- Resolve the problem of underlay Pods unable to access ClusterIP
- Coordinate the routing for Pods with multiple NICs, ensuring consistent packet paths
- Detect IP conflicts within Pods
- Check the reachability of Pod gateways
- Support fixed Mac address prefixes for Pods

Let's delve into how coordinator implements these features.

> You can configure `coordinator` by specifying all the relevant fields in `SpinderMultusConfig` if a NetworkAttachmentDefinition CR is created via `SpinderMultusConfig CR`. For more information, please refer to [SpinderMultusConfig](../reference/crd-spidermultusconfig.md).
>
> `Spidercoordinators CR` serves as the global default configuration (all fields) for `coordinator`. However, this configuration has a lower priority compared to the settings in the NetworkAttachmentDefinition CR. In cases where no configuration is provided in the NetworkAttachmentDefinition CR, the values from `Spidercoordinators CR` serve as the defaults. For detailed information, please refer to [Spidercoordinator](../reference/crd-spidercoordinator.md).

## Resolve the problem of underlay Pods unable to access ClusterIP

When using underlay CNIs like Macvlan, IPvlan, SR-IOV, and others, a common challenge arises where underlay pods are unable to access ClusterIP. This occurs because accessing ClusterIP from underlay pods requires routing through the gateway on the switch. However, in many instances, the gateway is not configured with the proper routes to reach the ClusterIP, leading to restricted access.

### Configure `coordinator` to run in underlay networks

> By default, the value of mode is set to auto (specified as spec.mode in the `Spidercoordinator CR`). In this configuration, `coordinator` will check if the current CNI network interface is `eth0`. If it is, `coordinator` identifies itself as operating in the underlay mode.
> If the current network interface is not `eth0`, `coordinator` further checks if the Pod has a `veth0` network interface. If `veth0` exists,  `coordinator` identifies itself as operating in the underlay mode.

When your businesses are deployed in a "traditional network" or IAAS environment, the IP addresses of the Pods can be directly assigned from the host machine's IP subnet. This enables the Pods to use their own IP addresses for both east-west and north-south communication.

This mode offers several advantages:

- Avoid interference from NAT mappings, allowing the Pods to maintain their original IP addresses
- Leverage underlying network devices for access control over Pods
- Enhance the performance of Pod network communication by eliminating the need for tunneling technologies

When Pods access east-west traffic (ClusterIP) within the cluster, the traffic is initially routed to the local host. The host then uses its own IP address to reach the target Pod. This process may involve three-layer hops through external routers. Therefore, it is crucial to ensure proper layer 3 network connectivity.

To resolve this issue, you can configure `coordinator` to operate in underlay networks. In this mode, `coordinator` creates a Veth pair, with one end placed on the host and the other within the Pod's network namespace. Furthermore, routing rules are set within the Pod, enabling traffic forwarding via the veth device when accessing ClusterIP. And Traffic is forwarded via eth0 or net1 interfaces for external destinations outside the cluster.

![underlay](../images/spiderpool-underlay.jpg)

Here are some examples:

```yaml
apiVersion: k8s.cni.cncf.io/v1
kind: NetworkAttachmentDefinition
metadata:
  name: macvlan-underlay
  namespace: kube-system
spec:
  config: |-
    {
        "cniVersion": "0.3.1",
        "name": "macvlan-underlay",
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
                "mode": "underlay"
            }
        ]
    }
```

mode: the operating mode of `coordinator`. It defaults to auto mode. With the annotation `v1.multus-cni.io/default-network: kube-system/macvlan-underlay` written into the Pod, `coordinator` will automatically determine the mode as underlay.

Let's view the routing information and other details of the Pod created through the macvlan-underlay configuration:

```shell
root@controller:~# kubectl exec -it macvlan-underlay-5496bb9c9b-c7rnp sh
kubectl exec [POD] [COMMAND] is DEPRECATED and will be removed in a future version. Use kubectl exec [POD] -- [COMMAND] instead.
#
# ip a show veth0
5: veth0@if428513: <BROADCAST,MULTICAST,UP,LOWER_UP> mtu 1500 qdisc noqueue state UP group default
    link/ether 4a:fe:19:22:65:05 brd ff:ff:ff:ff:ff:ff link-netnsid 0
    inet6 fe80::48fe:19ff:fe22:6505/64 scope link
       valid_lft forever preferred_lft forever
# ip r
default via 10.6.0.1 dev eth0
10.6.0.0/16 dev eth0 proto kernel scope link src 10.6.212.241
10.6.212.101 dev veth0 scope link
10.233.64.0/18 via 10.6.212.101 dev veth0
```

- **10.6.212.101 dev veth0 scope link**: 10.6.212.101 represents the IP address of the node, ensuring that traffic from the Pod to the node is forwarded through `veth0`.
- **10.233.64.0/18 via 10.6.212.101 dev veth0**: 10.233.64.0/18 represents the CIDR for the cluster's Service, ensuring that traffic from the Pod to ClusterIP is forwarded through `veth0`.

This solution heavily relies on kube-proxy's MASQUERADE. In some specific scenarios, it may be necessary to set `masqueradeAll` to true in the kube-proxy configuration.

> By default, the underlay subnet of Pods is different from the cluster's clusterCIDR, so there is no need to enable `masqueradeAll`. Traffic between them will go through SNAT. However, if the underlay subnet of Pods matches the cluster's clusterCIDR, `masqueradeAll` must be set to true.

### Configure `coordinator` to run in overlay networks

In contrast to the underlay mode, there are scenarios whose aim is just to enable the cluster to run on most underlying networks instead of specifying the network the cluster deploy in. This can be achieved by utilizing CNIs such as [Calico](https://github.com/projectcalico/calico) and [Cilium](https://github.com/cilium/cilium). These CNI plugins often leverage tunneling technologies like VXLAN to establish an overlay network plane and implement NAT for north-south communication.

> By default, the mode is set to auto (specified as auto in the spec.mode field of spidercoordinator CR). `coordinator` automatically determines the mode based on the current interface. If the interface that CNI invokes is not `eth0` and then there is no `veth0` within the Pod, `coordinator` identifies it as running in the overlay mode.

This mode offers several advantages:

- Abundant IP addresses mitigates concerns about IP scarcity
- Strong compatibility with on a wide range of underlying networks

However, there are some challenges to consider:

- Performance may be impacted due to the encapsulation techniques used by Calico.
- Most overlay solutions do not support fixed IP addresses for Pods.
- When attaching multiple interfaces to a Pod using Multus, communication between Pods may encounter issues with inconsistent packet routing paths, thus disrupting normal communication.

> With multiple NICs, Pod often encounter inconsistent packet routing paths. This can be problematic if there are security devices along the data path, as the inconsistent routes may cause the traffic to be perceived as a "half-connection" (lacking TCP SYN packet records but receiving TCP ACK packets). In such cases, security devices may block the connection, resulting in disrupted communication.

We can configure `coordinator` to run in the overlay mode to address this issue. In this mode, `coordinator` does not create veth devices but instead implements policy-based routing. This ensures that when Pods access ClusterIP, the traffic is forwarded through `eth0` (typically created by CNIs like Calico or Cilium), while traffic directed towards external destinations from Pods is routed through `net1` (usually created by CNIs like Macvlan or IPvlan).

> In the overlay mode, Spiderpool automatically synchronizes the Pod subnet of the cluster's default CNI. These subnets are utilized to configure routing within Pods with multiple interfaces, ensuring smooth communication between Pods using the default CNI through `eth0`. The configuration is specified by `spidercoordinator.spec.podCIDRType`, which defaults to `auto` and supports other options including ["calico", "cilium", "cluster", "none"].
>
> These routing configurations are injected during Pod startup and do not apply automatically to running Pods if there are changes in the associated CIDRs, unless restarting the Pods.
>
> Refer to [CRD-Spidercoordinator](../reference/crd-spidercoordinator.md) for more information

![overlay](../images/spiderpool-overlay.jpg)

Here is an example:

```yaml
apiVersion: k8s.cni.cncf.io/v1
kind: NetworkAttachmentDefinition
metadata:
  name: macvlan-overlay
  namespace: kube-system
spec:
  config: |-
    {
        "cniVersion": "0.3.1",
        "name": "macvlan-overlay",
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
                "mode": "overlay"
            }
        ]
    }
```

mode: the operating mode of `coordinator`. It defaults to auto mode. With the annotation `k8s.v1.cni.cncf.io/networks: kube-system/macvlan-overlay` written into the Pod, `coordinator` will automatically determine the mode as overlay.

Let's view the routing information and other details of the Pod created through the macvlan-overlay configuration:

```shell
root@controller:~# kubectl exec -it macvlan-overlay-97bf89fdd-kdgrb sh
kubectl exec [POD] [COMMAND] is DEPRECATED and will be removed in a future version. Use kubectl exec [POD] -- [COMMAND] instead.
#
# ip rule
0: from all lookup local
32759: from 10.233.105.154 lookup 100
32761: from all to 169.254.1.1 lookup 100
32762: from all to 10.233.64.0/18 lookup 100
32763: from all to 10.233.0.0/18 lookup 100
32765: from all to 10.6.212.102 lookup 100
32766: from all lookup main
32767: from all lookup default
# ip r
default via 10.6.0.1 dev net1
10.6.0.0/16 dev net1 proto kernel scope link src 10.6.212.227
# ip r show table 100
default via 169.254.1.1 dev eth0
10.6.212.102 dev eth0 scope link
10.233.0.0/18 via 10.6.212.102 dev eth0
10.233.64.0/18 via 10.6.212.102 dev eth0
169.254.1.1 dev eth0 scope link
```

Policy-based routes:

- **32759: from 10.233.105.154 lookup 100**: packets from `eth0` (Calico network interface) are routed through table 100
- **32762: from all to 10.233.64.0/18 lookup 100**: when The traffic of Pods accessing ClusterIP is routed through table 100 via `eth0`
- By default, all subnet routes for net1 are preserved in the Main table, while subnet routes for `eth0` are maintained in table 100.

In the overlay mode, Pods accessing ClusterIP rely solely on the overlay CNI's NIC (`eth0`) without any additional configurations.

## Detect Pod IP conflicts

During Pod creation, `coordinator` can be utilized to detect IP conflicts, supporting both IPv4 and IPv6 addresses. This is accomplished by sending ARP or NDP probe packets and confirming if the MAC address in the response packet is the same as the Pod's one. If a mismatch is found, it indicates an IP conflict.

To enable this feature, the following configuration can be done:

```yaml
apiVersion: k8s.cni.cncf.io/v1
kind: NetworkAttachmentDefinition
metadata:
  name: macvlan-underlay
  namespace: kube-system
spec:
  config: |-
    {
        "cniVersion": "0.3.1",
        "name": "macvlan-underlay",
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
                "mode": "underlay",
                "detectGateway": false,
                "detectIPConflict": true
            }
        ]
    }
```

If an IP conflict is detected, the creation of the Pod will fail. The event logs for the Pod will indicate the physical address of the host with the conflicting IP.

## Detect Pod gateway reachability

During Pod creation, `coordinator` can verify the reachability of the Pod's gateway, supporting both IPv4 and IPv6 gateway addresses. This is accomplished by sending ICMP packets to probe the accessibility of the gateway address.

Enable this feature through the following configuration:

```yaml
apiVersion: k8s.cni.cncf.io/v1
kind: NetworkAttachmentDefinition
metadata:
  name: macvlan-underlay
  namespace: kube-system
spec:
  config: |-
    {
        "cniVersion": "0.3.1",
        "name": "macvlan-underlay",
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
                "mode": "underlay",
                "detectGateway": true
            }
        ]
    }
```

If the Pod's gateway is unreachable, the creation of the Pod will fail. The event logs for the Pod will indicate related errors.

## Fix MAC address prefix for Pods

`coordinator` allows to set a fixed MAC address prefix for Pods. The MAC address of each Pod will be generated by a combination of the configured MAC address prefix and the Pod's IP.

Enable this feature through the following configuration:

```yaml
apiVersion: k8s.cni.cncf.io/v1
kind: NetworkAttachmentDefinition
metadata:
  name: macvlan-underlay
  namespace: kube-system
spec:
  config: |-
    {
        "cniVersion": "0.3.1",
        "name": "macvlan-underlay",
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
                "mode": "underlay",
                "podMACPrefix": "0a:1b"
            }
        ]
    }
```

You can check if the MAC address prefix of the Pod starts with "0a:1b" after a Pod is created.

## Known issues

- Underlay mode: TCP communication between underlay Pods and overlay Pods (Calico or Cilium) fails

    This issue arises from inconsistent packet routing paths. Request packets are matched with the routing on the source Pod side and forwarded through veth0 to the host side. And then the packets are further forwarded to the target Pod. The target Pod perceives the source IP of the packet as the underlay IP of the source Pod, allowing it to bypass the source Pod's host and directly route through the underlay network. However, on the host, this is considered an invalid packet (as it receives unexpected TCP SYN-ACK packets that are conntrack table invalid), explicitly dropping it using an iptables rule in kube-proxy. Switching the kube-proxy mode to ipvs can address this issue.

- Overlay mode: with Cilium as the default CNI and multiple NICs for the Pod, the underlay interface of the Pod cannot communicate with the node.

    Macvlan interfaces do not allow direct communication between parent and child interfaces in bridge mode in most cases. To facilitate communication between the underlay IP of the Pod and the node, we rely on the default CNI to create Veth devices. However, Cilium restricts the forwarding of IPs from non-Cilium subnets through these Veth devices.
