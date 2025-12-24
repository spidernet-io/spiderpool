# Usage Index

**English** ｜ [**简体中文**](./readme-zh_CN.md)

## Install Spiderpool

### Install Spiderpool on bare metal environment

Pods can have one or multiple underlay CNI networks, and enable connect to the underlay network. Please refer to [underlay network case](../concepts/arch.md#use-case-pod-with-multiple-underlay-cni-interfaces) for more details.

Please refer to the following examples for installation:

- [Create a cluster using macvlan](./install/underlay/get-started-macvlan.md)

- [Create a cluster using SR-IOV CNI](./install/underlay/get-started-sriov.md)

- [Create a cluster using ovs](./install/underlay/get-started-ovs.md)

- [Create a cluster: calico CNI for fixed IP addresses](./install/underlay/get-started-calico.md)

- [Create a cluster: weave CNI for fixed IP addresses](./install/underlay/get-started-weave.md)

### Install Spiderpool on VMs and Public Cloud Environments

On VMs and Public Cloud environments, it could use the Spiderpool to directly access underlay network of the VPC :

- [Create a cluster on Alibaba Cloud with ipvlan-based networking](./install/cloud/get-started-alibaba.md)

- [Create a cluster on VMware vSphere](./install/cloud/get-started-vmware.md)

- [Create a cluster on AWS with ipvlan-based networking](./install/cloud/get-started-aws.md)

- For VMware vSphere platform, you can run ipvlan CNI without enabling ["promiscuous" mode](https://docs.vmware.com/cn/VMware-vSphere/8.0/vsphere-security/GUID-3507432E-AFEA-4B6B-B404-17A020575358.html) for vSwitch to ensure forwarding performance. Refer to the [installation guide](./install/cloud/get-started-vmware.md) for details.

### Install Spiderpool with Overlay CNI for dual CNI case

Pods can own one overlay CNI interfaces and multiple underlay CNI interfaces of the Spiderpool, and enable connect to both overlay and underlay networks. Please refer to [dual CNIs case](../concepts/arch.md#use-case-pod-with-one-overlay-interface-and-multiple-underlay-interfaces) for more details.

Please refer to the following examples for installation:

- [Create a dual-network cluster using kind](./install/get-started-kind.md)

- [Create a dual-network cluster using calico and macvlan CNI](./install/overlay/get-started-calico.md)

- [Create a dual-network cluster using Cilium and macvlan CNI](./install/overlay/get-started-cilium.md)

### Install Spiderpool for AI cluster

AI clusters typically use multi-path RDMA networks to provide communication for GPUs. Spiderpool can enable RDMA communication capabilities for containers.

- [Create a cluster: provide RDMA(Infiniband or RoCE) network with SR-IOV or Macvlan](./install/ai/index.md)

### TLS Certificate

During the Spiderpool installation, you can choose a method for TLS certificate generation. For more information, refer to the [article](./install/certificate.md).

## Uninstall Spiderpool

For instructions on how to uninstall Spiderpool, please refer to the [uninstall guide](./install/uninstall.md).

## Upgrade Spiderpool

For instructions on how to upgrade Spiderpool, please refer to the [upgrade guide](./install/upgrade.md).

## Use Spiderpool

### IPAM

- Applications can share an IP pool. See the [example](./spider-affinity.md) for reference.

- Stateless applications can have a dedicated IP address pool with fixed IP usage range for all Pods. Refer to the [example](./spider-subnet.md) for more details.

- For stateful applications, each Pod can be allocated a persistent fixed IP address. It also provides control over the IP range used by all Pods during scaling operations. Refer to the [example](./statefulset.md) for details.

- Underlay networking support is available for kubevirt, allowing fixed IP addresses for virtual machines. Refer to the [example](./kubevirt.md) for details.

- Applications deployed across subnets can be assigned different subnet IP addresses for each replica. Refer to the [example](./network-topology.md) for details.

- The Subnet feature separates responsibilities between infrastructure administrators and application ones.
  It automates IP pool management for applications with fixed IP requirements, enabling automatic creation, scaling, and deletion of fixed IP pools.
  This greatly reduces operational burden. See the [example](./spider-subnet.md) for practical use cases.

  In addition to supporting native Kubernetes application controllers, Spiderpool's Subnet feature complements third-party application controllers implemented using operators. Refer to the [example](./operator.md) for more details.

- Default IP pools can be set at either the cluster-level or tenant-level. IP pools can be shared throughout the entire cluster or restricted to specific tenants. Check out the [example](./spider-affinity.md) for details.

- IP pool based on node topology caters to fine-grained subnet planning requirements for each node. Refer to the [example](./network-topology.md) for details.

- Custom routing can be achieved through IP pools, Pod annotations, and other methods. Refer to the [example](./route.md) for details.

- Multiple IP pools can be configured by applications to provide redundancy for IP resources. Refer to the [example](./spider-ippool.md) for details..

- Global reserved IP addresses can be specified to prevent IPAM from allocating those addresses, thereby avoiding conflicts with externally used IPs. Refer to the [example](./reserved-ip.md) for details.

- Efficient performance in IP address allocation and release is ensured. Refer to the [report](../concepts/ipam-performance.md) for details..

- Well-designed IP reclamation mechanisms promptly allocate IP addresses during cluster or application recovery processes. Refer to the [example](../concepts/ipam-des.md) for details.

### Multiple Network Interfaces Features

- Spiderpool offers the ability to assign IP addresses from different subnets to multiple network interfaces of a Pod. This feature ensures coordinated policy routing among all interfaces, guaranteeing consistent data paths for outgoing and incoming requests and mitigating packet loss. Moreover, it allows for customization of the default route using a specific network interface's gateway.

    For Pods with multiple underlay CNI network interfaces, you can refer to the [example](./multi-interfaces-annotation.md).

    For Pods with one overlay network interface and multiple underlay interfaces, you can refer to the [example](./install/overlay/get-started-calico.md).

### Connectivity

- Support for shared and exclusive modes of RDMA network cards enables applications to utilize RDMA communication devices via maclan, ipvlan, and SR-IOV CNI. For more details, see the [SR-IOV example](./install/ai/index.md) and  [Macvlan example](./install/ai/get-started-macvlan.md).

- coordinator plugin facilitates MAC address reconfiguration based on the IP address of the network interface, ensuring a one-to-one correspondence between them. This approach prevents the need to update ARP forwarding rules in network switches and routers, thus eliminating packet loss. Read the [article](../concepts/coordinator.md#fix-mac-address-prefix-for-pods) for further information.

- Spiderpool enables access to ClusterIP through kube-proxy and eBPF kube-proxy replacement for plugins such as [Macvlan CNI](https://github.com/containernetworking/plugins/tree/main/plugins/main/macvlan),
[vlan CNI](https://github.com/containernetworking/plugins/tree/main/plugins/main/vlan),
[ipvlan CNI](https://github.com/containernetworking/plugins/tree/main/plugins/main/ipvlan),
[SR-IOV CNI](https://github.com/k8snetworkplumbingwg/sriov-cni),
[ovs CNI](https://github.com/k8snetworkplumbingwg/ovs-cni). This allows seamless communication between Pods and the host machine, thereby resolving Pod health check issues. Refer to the [example](./underlay_cni_service.md) for details.

- Spiderpool assists in IP address conflict detection and gateway reachability checks, ensuring uninterrupted Pod communication. Refer to the [example](../concepts/coordinator.md) for details.

- The network of Multi-cluster could be connected by a same underlay network, or [Submariner](./submariner.md) .

### Operations and Management

- Spiderpool dynamically creates BOND interfaces and VLAN sub-interfaces on the host machine during Pod startup. This feature assists in setting up master interfaces for  [Macvlan CNI](https://github.com/containernetworking/plugins/tree/main/plugins/main/macvlan) and [ipvlan CNI](https://github.com/containernetworking/plugins/tree/main/plugins/main/ipvlan). Check out the [example](../reference/plugin-ifacer.md) for implementation details.

- Convenient generation of [Multus](https://github.com/k8snetworkplumbingwg/multus-cni) NetworkAttachmentDefinition instances with optimized CNI configurations. Spiderpool ensures correct JSON formatting to enhance user experience. See the [example](./spider-multus-config.md) for details.

### Other Features

- [Metrics](../reference/metrics.md)

- Support for AMD64 and ARM64 architectures

- All features are compatible with ipv4-only, ipv6-only, and dual-stack scenarios. Refer to the [example](./spider-ippool.md) for use cases.
