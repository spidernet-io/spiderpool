# Multi-CNI Coexistence with a Cluster

**English** | [**简体中文**](./multi_cni_coexist-zh_CN.md)

## Background

CNIs are important components of a Kubernetes cluster. Typically, one CNI (e.g. Calico) is deployed and is responsible for the connectivity of the cluster network. In some cases, customers may use multiple types of CNIs in the cluster based on performance, security, etc., such as Macvlan CNIs of the Underlay type, and then there may be multiple Pods of different CNI types in a cluster, and different types of Pods are suitable for different scenarios:

* Pod with a single Calico NIC: System components such as CoreDNS do not have the need for a fixed IP, nor do they need to communicate with north-south traffic, but only need to communicate with east-west traffic in the cluster.
* Pods with a single Macvlan card: For applications with special requirements for performance and security, or for traditional up-cloud applications that require direct north-south traffic with the Pod IP.
* Multi-Card Pod with Calico and Macvlan cards: A combination of both. Both need to access cluster north-south traffic with a fixed Pod IP and cluster east-west traffic (e.g., with a Calico Pod or Service).

In addition, when multiple CNI pods exist in a cluster, there are actually two different data forwarding schemes in the cluster: Underlay and Overlay. this can lead to a number of other issues:

* Pods using the Underlay network cannot access the cluster's north-south traffic.
* Pods using Underlay networks cannot communicate directly with Pods using Overlay networks in the cluster: Due to inconsistent forwarding paths, Overlay networks often need to go through nodes for secondary forwarding, whereas Underlay networks are generally forwarded directly through the underlying gateway. Therefore, when they access each other, packet loss may occur because the underlying switch does not synchronize the routes of the cluster subnet.
* Using two network modes for a cluster may increase the complexity of use and operation, such as IP address management.

Spiderpool is a complete Underlay network solution that solves the interoperability problem when there are multiple CNIs in a cluster and reduces the IP address operation and maintenance burden. The following section describes the data forwarding process between them.

## Quick start

* Calico + Macvlan multi-network card quickstart can be found in [get-stared-calico](./install/overlay/get-started-calico.md)
* For a single Macvlan card see [get-started-macvlan](./install/underlay/get-started-macvlan.md).
* Underlay CNI Access Service See [underlay_cni_service](./underlay_cni_service.md)

## Data forwarding process

![dataplane](../images/underlay_overlay_cni.png)

Several typical communication scenarios are described below.

### Calico Pod Accessing Calico and Macvlan Multi-NIC Pods

The lines labeled `1` and `2` in [Data Forwarding Process Figure](#data-forwarding-process) show the following.

1. The request packet is forwarded from Pod1 (single calico NIC pod) via its calixxx virtual NIC to the network stack of node1 on the line `1`, and routed between node1 and node2 to the target host node 2.

2. Regardless of whether the access is to the Calico NIC (10.233.100.2) or the Macvlan NIC IP (10.7.200.1) of the target Pod2 (Calico and Macvlan Multi-NIC Pod), it will be forwarded through the calixxx virtual NIC corresponding to Pod2 (Calico and Macvlan Multi-NIC Pod) to the Pod. Pod.

    Due to the limitation of Macvlan bridge mode, the master parent-child interface cannot communicate directly with each other, so the node cannot access the pod's macvlan IP directly. spiderpool will inject a route through calixxx for the Pod's macvlan NIC for forwarding the communication between the macvlan parent-child interface.

3. When Pod2 (Calico and Macvlan multicard pod) initiates a reply message, it follows the line `2`: since the IP of the target Pod1 is: 10.233.100.1, it hits the Calico subnet route set up in Pod2 (as follows), so that all the accesses to the calico subnet targets are forwarded from eth0 to the network stack of the node node2.

        ～# kubectl  exec -it calico-macvlan-556dddfdb-4ctxv -- ip r
        10.233.64.0/18 via 10.7.168.71 dev eth0 src 10.233.100.2

4. Since the destination IP of the reply message is Pod1 (single calico NIC IP): 10.233.100.1, it will match the tunnel route of the calico subnet and then forward to the target node node1. Finally, it will be forwarded to pod1 through the calixxx virtual NIC corresponding to Pod1, and the whole access will be finished.

### Calico+Macvlan Multi-Card Pod Access to Calico Pod's Service

The lines labeled `1` and `2` in [Data Forwarding Process Figure](#data-forwarding-process) show the following.

1. pod1 (a single calico NIC pod) and pod2 (a pod with both calico and macvlan NICs) as shown in the figure both use the calico ip returned to the kubelet as the PodIP, so when they communicate directly in the normal way, they both use each other's calico ip as the destination to initiate access.

2. When pod2 accesses pod1's clusterip on its own initiative, due to the routing set by spiderpool in the pod: packets accessing the service use the IP of the calico card as the source address, and are forwarded from eth0 to node2. The following 10.233.0.0/18 is the subnet of service:

        ~# kubectl exec -it calico-macvlan-556dddfdb-4ctxv -- ip r
        10.233.0.0/18 via 10.7.168.71 dev eth0 src 10.233.100.2

3. After the kube-proxy on node2's network stack resolves its destination clusterip address to pod1 (a pod with a single calico NIC): 10.233.100.1, it is forwarded to the destination host node1 through the node tunneling route set by calico, and then forwarded to Pod1 through the calixxx virtual NIC corresponding to pod1. Finally, it is forwarded to Pod1 through the calixxx virtual NIC corresponding to Pod1.

4. The reply packet initiated by Pod1 is forwarded to node1 through the calixxx virtual NIC on line `1`, and then forwarded to node2 through the host-to-host tunneling route, and then the kube-proxy of node2 changes the source address to the address of the clusterip, and then sends it to Pod2 through the calixxx virtual NIC. and sent to Pod2 via the calixxx virtual NIC. The whole access is finished.

### Macvlan Pod access to Calico Pod

The lines labeled `3`, `4`, and `6` in [Data Forwarding Process Figure](#data-forwarding-process) show the following.

1. Spiderpool injects a routing table entry inside the pod to the calico subnet via veth0 forwarding. The following 10.233.64.0/18 is the calico subnet, and this route ensures that when Pod3 (a single Macvlan NIC pod) accesses Pod1 (a single calico NIC pod), it will be forwarded to node node3 on line `3` via veth0.

        ~# kubectl exec -it macvlan-76c49c7bfb-82fnt -- ip r
        10.233.64.0/18 via 10.7.168.71 dev veth0 src 10.7.200.2

2. After forwarding to node3, since the IP of the target pod1 is 10.233.100.1, the packet is forwarded to node1 through calico's tunnel route, and then forwarded to pod1 through the calixxx virtual NIC corresponding to pod1. 3.
3. However, pod1 (single calico NIC pod) sends the reply packet to node1 according to line `4`, and since the IP of the destination pod3 (single macvlan pod) is 10.7.200.2, it forwards the packet directly to pod3 according to line `6` without going through the node to forward it, which results in inconsistent paths of the packet forwarding, which may be recognized by the kernel. This results in an inconsistent packet forwarding path back and forth, and the kernel may consider the state of the packet's conntrack to be invalid, which will be discarded by one of kube-proxy's iptables rules: `6`.

        ~# iptables-save -t filter | grep '--ctstate INVALID -j DROP'
        iptables -A FORWARD -m conntrack --ctstate INVALID -j DROP

        This rule was originally written to address an issue raised by [#Issue 74839](https://github.com/kubernetes/kubernetes/issues/74839), where some tcp messages exceeded the window size limit and were marked by the kernel as having an invalid conntrack state. The k8s community has resolved this issue by issuing this rule, but it may affect packet round-trip inconsistencies in this scenario. As reported in related community issues: [#Issue 117924](https://github.com/kubernetes/kubernetes/issues/117924), [#Issue 94861](https://github.com/kubernetes/) kubernetes/issues/94861), [#Issue 177](https://github.com/spidernet-io/cni-plugins/issues/177) and others.

        We pushed through the community to fix this issue, which was finally resolved in [only drop invalid cstate packets if non liberal](https://github.com/kubernetes/kubernetes/pull/120412) with kubernetes We need to make sure to set the sysctl parameter: `sysctl -w net.netfilter.nf_conntrack_tcp_be_liberal=1` on each node and restart Kube-proxy so that kube-proxy doesn't drop this drop rule and it doesn't affect the single Macvlan pod with a non-liberal packet. affect communication between a single Macvlan pod and a single Calico pod.

        After execution, check if the drop rule still exists on the node, if there is no output, it is OK. If there is no output, then it is working. Otherwise, check that sysctl is set correctly and that kube-proxy is restarted.

        ~# iptables-save -t filter | grep '--ctstate INVALID -j DROP'

        Note: You must make sure that the k8s version is greater than v1.29. If your k8s version is less than v1.29, then this rule will affect access between Macvlan Pod and Calico Pod.

4. When this drop rule does not exist, the reply packet sent by pod1 follows the line `6` to be able to forward the packet directly to pod3, and the entire access ends.

### Macvlan Pod access to Calico Pod's Service

As shown in Figure 1 for lines `3`, `4` and `5`.

1. Spiderpool injects a service route into Pod3 (a single macvlan pod), expecting that when Pod3 accesses Service on line `3`, packets are forwarded to node3 through the veth0 card. 10.233.0.0/18 is the subnet for Service.

        ～# 10.233.0.0/18 is the subnet of Service.
        10.233.0.0/18 via 10.7.168.71 dev eth0 src 10.233.100.2

2. After the packet is forwarded to node3, the kube-proxy of the host network stack converts the clusterip to the IP of Pod1 (a pod with a single calico NIC), which is then forwarded to node1 through the tunnel route set by Calico. note that when the request packet is sent from node3, its source address is SNATed to the IP of node3, which ensures that when the target host node1 receives the packet, it can return the packet in the same way without the inconsistency of the round-trip path as in the previous scenario. Note that when the request packet is sent from node3, its source address will be SNATed to the IP of node3, which ensures that when the target host node1 receives the packet, it will be able to return the packet in the original way without the inconsistency in the round-trip path as in the previous scenario. Thus the request packet is forwarded to host node1, and then forwarded to pod1 via the calixxx virtual NIC.

3. Pod1 (single calico NIC pod) forwards the reply packet to node1 via the calixxx virtual NIC on line `4`. The destination address of the reply packet is the IP of node3, so it is forwarded to node3 via node routing, and the source address is changed back to the IP of Pod3 (single macvlan NIC pod) via Kube-proxy. Then, Kube-proxy changes the source address back to the IP of Pod3 (a single macvlan pod), and then matches the macvlan pod direct route set by spiderpool on the host, and forwards the packet to the target Pod3 through the vethxxx device according to the line `5`, and the whole access is completed.

## Conclusion

We have summarized some communication scenarios when these three types of Pods exist in a cluster as follows.

| Source\Target | Calico Pod | Macvlan Pod | Calico + Macvlan Multi-NIC Pod | Service for Calico Pod | Service for Macvlan Pod | Service for Calico + Macvlan Multi-NIC Pod |
|- |- |- |- |- |- |-|
| Calico Pod | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ |
| Macvlan Pod |  requires kube-proxy version greater than v1.29. | ✅ | ✅  | ✅ | ✅ | ✅ |
| Calico + Macvlan Multi NIC Pod | ✅ |  ✅ | ✅  | ✅ | ✅ | ✅ |
