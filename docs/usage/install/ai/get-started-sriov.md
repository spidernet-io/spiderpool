# AI Cluster With SR-IOV

**English** | [**简体中文**](./get-started-sriov-zh_CN.md)

## Introduction

This section explains how to provide RDMA communication capabilities to containers using SR-IOV technology in the context of building an AI cluster. This approach is applicable in both RoCE and Infiniband network scenarios.

Spiderpool uses the [sriov-network-operator](https://github.com/k8snetworkplumbingwg/sriov-network-operator) to provide containers with RDMA devices based on SR-IOV interfaces:

The Linux RDMA subsystem can operate in two modes: shared mode or exclusive mode:

- In shared mode, the container can see the RDMA devices of all VF devices on the PF interface, but only the VF assigned to the container will have a GID Index starting from 0.

- In exclusive mode, the container will only see the RDMA device of the VF assigned to it, without visibility of the PF or other VF RDMA devices.
Different CNIs are used for different network scenarios:

    1. In Infiniband network scenarios, the [IB-SRIOV CNI](https://github.com/k8snetworkplumbingwg/ib-sriov-cni) is used to provide SR-IOV network interfaces to the POD.

    2. In RoCE network scenarios, the [SR-IOV CNI](https://github.com/k8snetworkplumbingwg/sriov-cni) is used to expose the RDMA network interface on the host to the Pod, thereby exposing RDMA resources. Additionally, the [RDMA CNI](https://github.com/k8snetworkplumbingwg/rdma-cni) can be used to achieve RDMA device isolation.

Note:

- Based on SR-IOV technology, the RDMA communication capability of containers is only applicable to bare metal environments, not to virtual machine environments.

## Comparison of Macvlan CNI RDMA Solution

| Comparison Dimension | Macvlan Shared RDMA Solution       | SR-IOV CNI Isolated RDMA Solution  |
| -------------------- | ---------------------------------- | ---------------------------------- |
| Network Isolation    | All containers share RDMA devices, poor isolation | Containers have dedicated RDMA devices, good isolation |
| Performance          | High performance                   | Optimal performance with hardware passthrough |
| Resource Utilization | High resource utilization          | Low, limited by the number of supported VFs |
| Configuration Complexity | Relatively simple configuration | More complex configuration, requires hardware support |
| Compatibility        | Good compatibility, suitable for most environments | Depends on hardware support, less compatible |
| Applicable Scenarios | Suitable for most scenarios, including bare metal and VMs | Only suitable for bare metal, not for VM scenarios |
| Cost                 | Low cost, no additional hardware support needed | High cost, requires hardware supporting SR-IOV |
| Support RDMA Protocol | Support Roce protocol, not support Infiniband protocol | Support Roce and Infiniband protocol |

## Solution

This article will introduce how to set up Spiderpool using the following typical AI cluster topology as an example.

![AI Cluster](../../../images/ai-cluster.png)

Figure 1: AI Cluster Topology

The network planning for the cluster is as follows:

1. The calico CNI runs on the eth0 network card of the nodes to carry Kubernetes traffic. The AI workload will be assigned a default calico network interface for control plane communication.

2. The nodes use Mellanox ConnectX5 network cards with RDMA functionality to carry the RDMA traffic for AI computation. The network cards are connected to a rail-optimized network. The AI workload will be additionally assigned SR-IOV virtualized interfaces for all RDMA network cards to ensure high-speed network communication for the GPUs.

## Installation Requirements

- Refer to [the Spiderpool Installation Requirements](./../system-requirements.md).

- Prepare the Helm binary on the host.

- In Infiniband network scenarios, ensure that the OpenSM subnet manager is functioning properly.

- Install a Kubernetes cluster with kubelet running on the host’s eth0 network card as shown in Figure 1.
    Install Calico as the default CNI for the cluster, using the host’s eth0 network card for Calico’s traffic forwarding.
    If not installed, refer to [the official documentation](https://docs.tigera.io/calico/latest/getting-started/kubernetes/) or use the following commands to install:

    ```shell
    $ kubectl apply -f https://github.com/projectcalico/calico/blob/master/manifests/calico.yaml
    $ kubectl wait --for=condition=ready -l k8s-app=calico-node  pod -n kube-system 
    # set calico to work on host eth0 
    $ kubectl set env daemonset -n kube-system calico-node IP_AUTODETECTION_METHOD=kubernetes-internal-ip
    # set calico to work on host eth0 
    $ kubectl set env daemonset -n kube-system calico-node IP6_AUTODETECTION_METHOD=kubernetes-internal-ip  
    ```

## Host Preparation

1. Install the RDMA network card driver, then restart the host (to see the network card)

    For Mellanox network cards, you can download [the NVIDIA OFED official driver](https://network.nvidia.com/products/infiniband-drivers/linux/mlnx_ofed/) and install it on the host using the following installation command:

    ```shell
    mount /root/MLNX_OFED_LINUX-24.01-0.3.3.1-ubuntu22.04-x86_64.iso   /mnt
    /mnt/mlnxofedinstall --all
    ```

    For Mellanox network cards, you can also perform a containerized installation to batch install drivers on all Mellanox network cards in the cluster hosts. Run the following command. Note that this process requires internet access to fetch some installation packages. When all the OFED pods enter the ready state, it indicates that the OFED driver installation on the hosts is complete:

    ```shell
    $ helm repo add spiderchart https://spidernet-io.github.io/charts
    $ helm repo update
    $ helm search repo ofed

    # pelase replace the following values with your actual environment
    # for china user, it could set `--set image.registry=nvcr.m.daocloud.io` to use a domestic registry
    $ helm install ofed-driver spiderchart/ofed-driver -n kube-system \
            --set image.OSName="ubuntu" \
            --set image.OSVer="22.04" \
            --set image.Arch="amd64"
    ```

    > If you want the RDMA system to operate in exclusive mode, at least one of the following conditions must be met: (1) The system must be based on the Linux kernel version 5.3.0 or later, with the RDMA module loaded. The RDMA core package provides a method to automatically load the relevant modules at system startup. (2) Mellanox OFED version 4.7 or later is required. In this case, it is not necessary to use a kernel based on version 5.3.0 or later.

2. For SR-IOV scenarios, set the RDMA subsystem on the host to exclusive mode, allowing containers to independently use RDMA devices and avoiding sharing with other containers.

    ```shell
    # Check the current operating mode (the Linux RDMA subsystem operates in shared mode by default):
    $ rdma system
       netns shared copy-on-fork on

    # Persist the exclusive mode to remain effective after a reboot
    $ echo "options ib_core netns_mode=0" >> /etc/modprobe.d/ib_core.conf

    # Switch the current operating mode to exclusive mode. If the setting fails, please reboot the host
    $ rdma system set netns exclusive

    # Verify the successful switch to exclusive mode
    $ rdma system
       netns exclusive copy-on-fork on
    ```

3. Set the RDMA operating mode of the network card (Infiniband or Ethernet)

    3.1 Verify the network card's supported operating modes: In this example environment, the host is equipped with Mellanox ConnectX 5 VPI network cards. Query the RDMA devices to confirm that the network card driver is installed correctly.

    ```shell
    $ rdma link
      link mlx5_0/1 state ACTIVE physical_state LINK_UP netdev ens6f0np0
      link mlx5_1/1 state ACTIVE physical_state LINK_UP netdev ens6f1np1
      ....... 
    ```

    Verify the network card's operating mode. The following output indicates that the network card is operating in Ethernet mode and can achieve RoCE communication:

    ```shell
    $ ibstat mlx5_0 | grep "Link layer"
       Link layer: Ethernet
    ```

    The following output indicates that the network card is operating in Infiniband mode and can achieve Infiniband communication:

    ```shell
    $ ibstat mlx5_0 | grep "Link layer"
       Link layer: InfiniBand
    ```

    If the network card is not operating in the expected mode, enter the following command to verify that the network card supports configuring the LINK_TYPE parameter. If the parameter is not available, please switch to a supported network card model:

    ```shell
    $ mst start

    # check the card's PCIE 
    $ lspci -nn | grep Mellanox
          86:00.0 Infiniband controller [0207]: Mellanox Technologies MT27800 Family [ConnectX-5] [15b3:1017]
          86:00.1 Infiniband controller [0207]: Mellanox Technologies MT27800 Family [ConnectX-5] [15b3:1017]
          ....... 

    # check whether the network card supports parameters LINK_TYPE 
    $ mlxconfig -d 86:00.0  q | grep LINK_TYPE
          LINK_TYPE_P1                                IB(1)
    ```

    3.2 Batch setting the operating mode of network cards: Get the [batch setting script](https://github.com/spidernet-io/spiderpool/blob/main/tools/scripts/setNicRdmaMode.sh), after setting, please restart the host

    ```shell
    $ chmod +x ./setNicRdmaMode.sh

    # Batch query all RDMA network cards working in ib or eth mode
    $ ./setNicRdmaMode.sh q

    # Switch all RDMA network cards to eth mode
    $ RDMA_MODE="roce" ./setNicRdmaMode.sh

    # Switch all RDMA network cards to ib mode
    $ RDMA_MODE="infiniband" ./setNicRdmaMode.sh
    ```

4. Set IP address, MTU, and policy routing for all RDMA network cards

    - In RDMA scenarios, switches and host network cards typically operate with larger MTU parameters to improve performance
    - Since Linux hosts have only one default route by default, in multi-network card scenarios, it's necessary to set policy default routes for different network cards to ensure that tasks in hostnetwork mode can run normal All-to-All and other communications

    Get the [Ubuntu network card configuration script](https://github.com/spidernet-io/spiderpool/blob/main/tools/scripts/setNicAddr.sh) and execute the following reference commands:
    
    ```shell
    $ chmod +x ./setNicAddr.sh

    # Configure the network card
    $ INTERFACE="eno3np2" IPV4_IP="172.16.0.10/24"  IPV4_GATEWAY="172.16.0.1" \
          MTU="4200" ENABLE_POLICY_ROUTE="true" ./setNicAddr.sh

    # View the network card's IP and MTU
    $ ip a s eno3np2
      4: eno3np2: <BROADCAST,MULTICAST,UP,LOWER_UP> mtu 4200 qdisc mq state UP group default qlen 1000
        link/ether 38:68:dd:59:44:4a brd ff:ff:ff:ff:ff:ff
        altname enp8s0f2np2
        inet 172.16.0.10/24 brd 172.16.0.255 scope global eno3np2
          valid_lft forever preferred_lft forever
        inet6 fe80::3a68:ddff:fe59:444a/64 scope link proto kernel_ll
          valid_lft forever preferred_lft forever 

    # View policy routing
    $ ip rule
      0: from all lookup local
      32763: from 172.16.0.10 lookup 152 proto static
      32766: from all lookup main
      32767: from all lookup default

    $ ip rou show table 152
      default via 172.16.0.1 dev eno3np2 proto static
    ```

5. Configure Host RDMA Lossless Network 

    In high-performance network scenarios, RDMA networks are very sensitive to packet loss. Once packet retransmission occurs, performance will drop sharply. Therefore, to ensure that RDMA network performance is not affected, the packet loss rate must be kept below 1e-05 (one in 100,000), ideally zero packet loss. For RoCE networks, the PFC + ECN mechanism can be used to ensure no packet loss during network transmission. Refer to [RoCE Lossless Network Configuration](../../roce-qos.md)
  
    > Configuring a lossless network requires an RDMA RoCE network environment and cannot be Infiniband. 
    >
    > Configuring a lossless network requires the switch to support the PFC + ECN mechanism and be aligned with the host side configuration; otherwise, it will not work.

6. Enable GPUDirect RDMA

    The installation of the [gpu-operator](https://github.com/NVIDIA/gpu-operator):

    a.  Enable the Helm installation options: `--set driver.rdma.enabled=true --set driver.rdma.useHostMofed=true`. The gpu-operator will install [the nvidia-peermem](https://network.nvidia.com/products/GPUDirect-RDMA/) kernel module, enabling GPUDirect RDMA functionality to accelerate data transfer performance between the GPU and RDMA network cards. Enter the following command on the host to confirm the successful installation of the kernel module:

    ```shell
    $ lsmod | grep nvidia_peermem
    nvidia_peermem         16384  0
    ```

    b. Enable the Helm installation option: `--set gdrcopy.enabled=true`. The gpu-operator will install the [gdrcopy](https://developer.nvidia.com/gdrcopy) kernel module to accelerate data transfer performance between GPU memory and CPU memory. Enter the following command on the host to confirm the successful installation of the kernel module:

    ```shell
    $ lsmod | grep gdrdrv
    gdrdrv                 24576  0
    ```

## Install Spiderpool

1. Use Helm to install Spiderpool and enable the SR-IOV component:

    ```shell
    helm repo add spiderpool https://spidernet-io.github.io/spiderpool
    helm repo update spiderpool
    kubectl create namespace spiderpool
    helm install spiderpool spiderpool/spiderpool -n spiderpool --set sriov.install=true
    ```

    > - If you are a user in China, you can specify the helm option `--set global.imageRegistryOverride=ghcr.m.daocloud.io` to use a domestic image source.
    > - Setting the command line parameters `--set spiderpoolAgent.prometheus.enabled --set spiderpoolAgent.prometheus.enabledRdmaMetric=true` and `--set grafanaDashboard.install=true` can enable RDMA metrics exporter and Grafana dashboard. For more information, please refer to [RDMA metrics](../../rdma-metrics.md).

    After completion, the installed components are as follows:

    ```shell
    $ kubectl get pod -n spiderpool
        operator-webhook-sgkxp                         1/1     Running     0          1m
        spiderpool-agent-9sllh                         1/1     Running     0          1m
        spiderpool-agent-h92bv                         1/1     Running     0          1m
        spiderpool-controller-7df784cdb7-bsfwv         1/1     Running     0          1m
        spiderpool-sriov-operator-65b59cd75d-89wtg     1/1     Running     0          1m
        spiderpool-init                                0/1     Completed   0          1m
        sriov-network-config-daemon-8h576              1/1     Running     0          1m
        sriov-network-config-daemon-n629x              1/1     Running     0          1m
    ```

2. Configure the SR-IOV Operator to Create VF Devices on Each Host

    Use the following command to query the PCIe information of the network card devices on the host. Confirm that the device ID [15b3:1017] appears
    in [the supported network card models list of the sriov-network-operator](https://github.com/k8snetworkplumbingwg/sriov-network-operator/blob/master/deployment/sriov-network-operator-chart/templates/configmap.yaml).

    ```shell
    $ lspci -nn | grep Mellanox
        86:00.0 Infiniband controller [0207]: Mellanox Technologies MT27800 Family [ConnectX-5] [15b3:1017]
        86:00.1 Infiniband controller [0207]: Mellanox Technologies MT27800 Family [ConnectX-5] [15b3:1017]
        ....
    ```

    The number of SR-IOV VFs (Virtual Functions) determines how many PODs a network card can simultaneously support. Different models of network cards have different maximum VF limits. For example, Mellanox's ConnectX series network cards typically have a maximum VF limit of 127.

    In the following example, we set up the network cards of GPU1 and GPU2 on each node, configuring 12 VFs for each card. Refer to the following configuration to set up the SriovNetworkNodePolicy for each network card associated with a GPU on the host. This setup will provide 8 SR-IOV resources for use.

    ```shell
    # For Ethernet networks, set LINK_TYPE=eth. For Infiniband networks, set LINK_TYPE=ib
    $ LINK_TYPE=eth
    $ cat <<EOF | kubectl apply -f -
    apiVersion: sriovnetwork.openshift.io/v1
    kind: SriovNetworkNodePolicy
    metadata:
      name: gpu1-nic-policy
      namespace: spiderpool
    spec:
      nodeSelector:
        kubernetes.io/os: "linux"
      resourceName: gpu1sriov
      priority: 99
      numVfs: 12
      nicSelector:
        deviceID: "1017"
        vendor: "15b3"
        rootDevices:
        - 0000:86:00.0
      linkType: ${LINK_TYPE}
      deviceType: netdevice
      isRdma: true
    ---
    apiVersion: sriovnetwork.openshift.io/v1
    kind: SriovNetworkNodePolicy
    metadata:
      name: gpu2-nic-policy
      namespace: spiderpool
    spec:
      nodeSelector:
        kubernetes.io/os: "linux"
      resourceName: gpu2sriov
      priority: 99
      numVfs: 12
      nicSelector:
        deviceID: "1017"
        vendor: "15b3"
        rootDevices:
        - 0000:86:00.0
      linkType: ${LINK_TYPE}
      deviceType: netdevice
      isRdma: true
    EOF
    ```

    After creating the SriovNetworkNodePolicy configuration, the sriov-device-plugin will be started on each node, responsible for reporting VF device resources.

    ```shell
    $ kubectl get pod -n spiderpool
        operator-webhook-sgkxp                         1/1     Running     0          2m
        spiderpool-agent-9sllh                         1/1     Running     0          2m
        spiderpool-agent-h92bv                         1/1     Running     0          2m
        spiderpool-controller-7df784cdb7-bsfwv         1/1     Running     0          2m
        spiderpool-sriov-operator-65b59cd75d-89wtg     1/1     Running     0          2m
        spiderpool-init                                0/1     Completed   0          2m
        sriov-device-plugin-x2g6b                      1/1     Running     0          1m
        sriov-device-plugin-z4gjt                      1/1     Running     0          1m
        sriov-network-config-daemon-8h576              1/1     Running     0          1m
        sriov-network-config-daemon-n629x              1/1     Running     0          1m
        .......
    ```

    Once the SriovNetworkNodePolicy configuration is created, the SR-IOV operator will sequentially evict PODs on each node, configure the
    VF settings in the network card driver, and then reboot the host. Consequently, you will observe the nodes in the cluster sequentially entering the SchedulingDisabled state and being rebooted.

    ```shell
    $ kubectl get node
        NAME           STATUS                     ROLES                  AGE     VERSION
        ai-10-1-16-1   Ready                      worker                 2d15h   v1.28.9
        ai-10-1-16-2   Ready,SchedulingDisabled   worker                 2d15h   v1.28.9
        .......
    ```

    It may take several minutes for all nodes to complete the VF configuration process. You can monitor the sriovnetworknodestates status to see if it has entered the Succeeded state, indicating that the configuration is complete.

    ```shell
    $ kubectl get sriovnetworknodestates -A
        NAMESPACE        NAME           SYNC STATUS   DESIRED SYNC STATE   CURRENT SYNC STATE   AGE
        spiderpool       ai-10-1-16-1   Succeeded     Idle                 Idle                 4d6h
        spiderpool       ai-10-1-16-2   Succeeded     Idle                 Idle                 4d6h
        .......
    ```

    For nodes that have successfully configured VFs, you can check the available resources of the node, including the reported SR-IOV device resources.

    ```shell
    $ kubectl get no -o json | jq -r '[.items[] | {name:.metadata.name, allocable:.status.allocatable}]'
        [
          {
            "name": "ai-10-1-16-1",
            "allocable": {
              "cpu": "40",
              "pods": "110",
              "spidernet.io/gpu1sriov": "12",
              "spidernet.io/gpu2sriov": "12",
              ...
            }
          },
          ...
        ]
    ```

    <a id="create-spiderpool-resource"></a>

3. Create CNI Configuration and Corresponding IP Pool Resources

    a. For Infiniband Networks, configure [the IB-SRIOV CNI](https://github.com/k8snetworkplumbingwg/ib-sriov-cni)  for all GPU-affinitized SR-IOV network cards and create the corresponding IP address pool. The following example configures the network card and IP address pool for GPU1

    ```shell
    $ cat <<EOF | kubectl apply -f -
    apiVersion: spiderpool.spidernet.io/v2beta1
    kind: SpiderIPPool
    metadata:
      name: gpu1-net11
    spec:
      gateway: 172.16.11.254
      subnet: 172.16.11.0/16
      ips:
        - 172.16.11.1-172.16.11.200
    ---
    apiVersion: spiderpool.spidernet.io/v2beta1
    kind: SpiderMultusConfig
    metadata:
      name: gpu1-sriov
      namespace: spiderpool
    spec:
      cniType: ib-sriov
      ibsriov:
        resourceName: spidernet.io/gpu1sriov
        rdmaIsolation: true
        ippools:
          ipv4: ["gpu1-net91"]
    EOF
    ```

    If you want to customize the MTU size, please refer to [Customize the MTU of SR-IOV VF](#customize-the-mtu-of-sr-iov-vf).

    b. For Ethernet Networks, configure [the SR-IOV CNI](https://github.com/k8snetworkplumbingwg/sriov-cni) for all GPU-affinitized SR-IOV network cards and create the corresponding IP address pool. The following example configures the network card and IP address pool for GPU1

    ```shell
    $ cat <<EOF | kubectl apply -f -
    apiVersion: spiderpool.spidernet.io/v2beta1
    kind: SpiderIPPool
    metadata:
      name: gpu1-net11
    spec:
      gateway: 172.16.11.254
      subnet: 172.16.11.0/16
      ips:
        - 172.16.11.1-172.16.11.200
    ---
    apiVersion: spiderpool.spidernet.io/v2beta1
    kind: SpiderMultusConfig
    metadata:
      name: gpu1-sriov
      namespace: spiderpool
    spec:
      cniType: sriov
      sriov:
        resourceName: spidernet.io/gpu1sriov
        enableRdma: true
        ippools:
          ipv4: ["gpu1-net11"]
    EOF
    ```

    If you want to customize the MTU size, please refer to [Customize the MTU of SR-IOV VF](#customize-the-mtu-of-sr-iov-vf).

## Create a Test Application

1. Create a DaemonSet application on a specified node to test the availability of SR-IOV devices on that node.
    In the following example, the annotation field `v1.multus-cni.io/default-network` specifies the use of the default Calico network card for control plane communication. The annotation field `k8s.v1.cni.cncf.io/networks` connects to the 8 VF network cards affinitized to the GPU for RDMA communication, and configures 8 types of RDMA resources.

    > NOTICE: It support auto inject RDMA resources for application, see [Auto inject RDMA Resources](#auto-inject-rdma-resources-based-on-webhook)

    ```shell
    $ helm repo add spiderchart https://spidernet-io.github.io/charts
    $ helm repo update
    $ helm search repo rdma-tools
   
    # run daemonset on worker1 and worker2
    $ cat <<EOF > values.yaml
    # for china user , it could add these to use a domestic registry
    #image:
    #  registry: ghcr.m.daocloud.io

    # just run daemonset in nodes 'worker1' and 'worker2'
    affinity:
      nodeAffinity:
        requiredDuringSchedulingIgnoredDuringExecution:
          nodeSelectorTerms:
          - matchExpressions:
            - key: kubernetes.io/hostname
              operator: In
              values:
              - worker1
              - worker2

    # sriov interfaces
    extraAnnotations:
      k8s.v1.cni.cncf.io/networks: |-
        [{"name":"gpu1-sriov","namespace":"spiderpool"},
        {"name":"gpu2-sriov","namespace":"spiderpool"},
        {"name":"gpu3-sriov","namespace":"spiderpool"},
        {"name":"gpu4-sriov","namespace":"spiderpool"},
        {"name":"gpu5-sriov","namespace":"spiderpool"},
        {"name":"gpu6-sriov","namespace":"spiderpool"},
        {"name":"gpu7-sriov","namespace":"spiderpool"},
        {"name":"gpu8-sriov","namespace":"spiderpool"}]

    # sriov resource
    resources:
      limits:
        spidernet.io/gpu1sriov: 1
        spidernet.io/gpu2sriov: 1
        spidernet.io/gpu3sriov: 1
        spidernet.io/gpu4sriov: 1
        spidernet.io/gpu5sriov: 1
        spidernet.io/gpu6sriov: 1
        spidernet.io/gpu7sriov: 1
        spidernet.io/gpu8sriov: 1
        #nvidia.com/gpu: 1
    EOF

    $ helm install rdma-tools spiderchart/rdma-tools -f ./values.yaml
    ```

    During the creation of the network namespace for the container, Spiderpool will perform connectivity tests on the gateway of the SR-IOV interface.
    If all PODs of the above application start successfully, it indicates successful connectivity of the VF devices on each node, allowing normal RDMA communication.

    <a id="checking-pod-network"></a>

2. Check the network namespace status of the container.

    You can enter the network namespace of any POD to confirm that it has 9 network cards.

    ```shell
    $ kubectl exec -it rdma-tools-4v8t8  bash
    kubectl exec [POD] [COMMAND] is DEPRECATED and will be removed in a future version. Use kubectl exec [POD] -- [COMMAND] instead.
    root@rdma-tools-4v8t8:/# ip a
       1: lo: <LOOPBACK,UP,LOWER_UP> mtu 65536 qdisc noqueue state UNKNOWN group default qlen 1000
           link/loopback 00:00:00:00:00:00 brd 00:00:00:00:00:00
           inet 127.0.0.1/8 scope host lo
              valid_lft forever preferred_lft forever
           inet6 ::1/128 scope host
              valid_lft forever preferred_lft forever
       2: tunl0@NONE: <NOARP> mtu 1480 qdisc noop state DOWN group default qlen 1000
           link/ipip 0.0.0.0 brd 0.0.0.0
       3: eth0@if356: <BROADCAST,MULTICAST,UP,LOWER_UP> mtu 1480 qdisc noqueue state UP group default qlen 1000
           link/ether ca:39:52:fc:61:cd brd ff:ff:ff:ff:ff:ff link-netnsid 0
           inet 10.233.119.164/32 scope global eth0
              valid_lft forever preferred_lft forever
           inet6 fe80::c839:52ff:fefc:61cd/64 scope link
              valid_lft forever preferred_lft forever
       269: net1: <BROADCAST,MULTICAST,UP,LOWER_UP> mtu 1500 qdisc mq state UP group default qlen 1000
           link/ether 3a:97:49:35:79:95 brd ff:ff:ff:ff:ff:ff
           inet 172.16.11.10/24 brd 10.1.19.255 scope global net1
              valid_lft forever preferred_lft forever
           inet6 fe80::3897:49ff:fe35:7995/64 scope link
              valid_lft forever preferred_lft forever
       239: net2: <BROADCAST,MULTICAST,UP,LOWER_UP> mtu 1500 qdisc mq state UP group default qlen 1000
           link/ether 1e:b6:13:0e:2a:d5 brd ff:ff:ff:ff:ff:ff
           inet 172.16.12.10/24 brd 10.1.19.255 scope global net1
              valid_lft forever preferred_lft forever
           inet6 fe80::1cb6:13ff:fe0e:2ad5/64 scope link
              valid_lft forever preferred_lft forever
       .....
    ```

    Check the routing configuration. Spiderpool will automatically tune policy routes for each network card, ensuring that external requests received on each card are returned through the same card.

    ```shell
    root@rdma-tools-4v8t8:/# ip rule
    0:  from all lookup local
    32762:  from 172.16.11.10 lookup 107
    32763:  from 172.16.12.10 lookup 106
    32764:  from 172.16.13.10 lookup 105
    32765:  from 172.16.14.10 lookup 104
    32765:  from 172.16.15.10 lookup 103
    32765:  from 172.16.16.10 lookup 102
    32765:  from 172.16.17.10 lookup 101
    32765:  from 172.16.18.10 lookup 100
    32766:  from all lookup main
    32767:  from all lookup default

    root@rdma-tools-4v8t8:/# ip route show table 100
        default via 172.16.11.254 dev net1
    ```

    In the main routing table, ensure that Calico network traffic, ClusterIP traffic, and local host communication traffic are all forwarded through the Calico network card.

    ```shell
    root@rdma-tools-4v8t8:/# ip r show table main
        default via 169.254.1.1 dev eth0
        172.16.11.0/24 dev net1 proto kernel scope link src 172.16.11.10
        172.16.12.0/24 dev net2 proto kernel scope link src 172.16.12.10
        172.16.13.0/24 dev net3 proto kernel scope link src 172.16.13.10
        172.16.14.0/24 dev net4 proto kernel scope link src 172.16.14.10
        172.16.15.0/24 dev net5 proto kernel scope link src 172.16.15.10
        172.16.16.0/24 dev net6 proto kernel scope link src 172.16.16.10
        172.16.17.0/24 dev net7 proto kernel scope link src 172.16.17.10
        172.16.18.0/24 dev net8 proto kernel scope link src 172.16.18.10
        10.233.0.0/18 via 10.1.20.4 dev eth0 src 10.233.119.164
        10.233.64.0/18 via 10.1.20.4 dev eth0 src 10.233.119.164
        10.233.119.128 dev eth0 scope link src 10.233.119.164
        169.254.0.0/16 via 10.1.20.4 dev eth0 src 10.233.119.164
        169.254.1.1 dev eth0 scope link
    ```

    Confirm that there are 8 RDMA devices.

    ```shell
    root@rdma-tools-4v8t8:/# rdma link
        link mlx5_27/1 state ACTIVE physical_state LINK_UP netdev net2
        link mlx5_54/1 state ACTIVE physical_state LINK_UP netdev net1
        link mlx5_67/1 state ACTIVE physical_state LINK_UP netdev net4
        link mlx5_98/1 state ACTIVE physical_state LINK_UP netdev net3
        .....
    ```

3. Confirm that RDMA data transmission is functioning properly between Pods across nodes.

    Open a terminal, enter a Pod, and start the service:

    ```shell
    # see 8 RDMA devices assigned to the Pod
    $ rdma link

    # Start an RDMA service
    $ ib_read_lat
    ```

    Open another terminal, enter another Pod, and access the service:

    ```shell
    # You should be able to see all RDMA network cards on the host
    $ rdma link
        
    # Successfully access the RDMA service of the other Pod
    $ ib_read_lat 172.91.0.115
    ```

## (Optional) Integrate with UFM on Infiniband Networks

For clusters using Infiniband networks, if there is a [UFM management platform](https://www.nvidia.com/en-us/networking/infiniband/ufm/) in the network, you can use [the ib-kubernetes plugin](https://github.com/Mellanox/ib-kubernetes). This plugin runs as a daemonset, monitoring all containers using SRIOV network cards and reporting the Pkey and GUID of VF devices to UFM.

1. Create the necessary certificates for communication on the UFM host:

    ```shell
    # replace to right address
    $ UFM_ADDRESS=172.16.10.10
    $ openssl req -x509 -newkey rsa:4096 -keyout ufm.key -out ufm.crt -days 365 -subj '/CN=${UFM_ADDRESS}'

    # Copy the certificate files to the UFM certificate directory:
    $ cp ufm.key /etc/pki/tls/private/ufmlocalhost.key
    $ cp ufm.crt /etc/pki/tls/certs/ufmlocalhost.crt

    # For containerized UFM deployment, restart the container service
    $ docker restart ufm

    # For host-based UFM deployment, restart the UFM service
    $ systemctl restart ufmd
    ```

2. On the Kubernetes cluster, create the communication certificates required by ib-kubernetes. Transfer the ufm.crt file generated on the UFM host to the Kubernetes nodes, and use the following command to create the certificate:

    ```shell
    # replace to right user
    $ UFM_USERNAME=admin

    # replace to right password
    $ UFM_PASSWORD=12345

    # replace to right address
    $ UFM_ADDRESS="172.16.10.10"
    $ kubectl create secret generic ib-kubernetes-ufm-secret --namespace="kube-system" \
                 --from-literal=UFM_USER="${UFM_USERNAME}" \
                 --from-literal=UFM_PASSWORD="${UFM_PASSWORD}" \
                 --from-literal=UFM_ADDRESS="${UFM_ADDRESS}" \
                 --from-file=UFM_CERTIFICATE=ufm.crt 
    ```

3. Install ib-kubernetes on the Kubernetes cluster

    ```shell
    git clone https://github.com/Mellanox/ib-kubernetes.git && cd ib-kubernetes
    $ kubectl create -f deployment/ib-kubernetes-configmap.yaml
    kubectl create -f deployment/ib-kubernetes.yaml 
    ```

4. On Infiniband networks, when creating Spiderpool's SpiderMultusConfig, you can configure the Pkey. Pods created with this configuration will use the Pkey settings and be synchronized with UFM by ib-kubernetes

    ```shell
    $ cat <<EOF | kubectl apply -f -
    apiVersion: spiderpool.spidernet.io/v2beta1
    kind: SpiderMultusConfig
    metadata:
      name: ib-sriov
      namespace: spiderpool
    spec:
      cniType: ib-sriov
      ibsriov:
        pkey: 1000
        ...
    EOF
    ```

    > Note: Each node in an Infiniband Kubernetes deployment may be associated with up to 128 PKeys due to kernel limitation

## Auto Inject RDMA Resources Based on Webhook

In the steps above, we demonstrated how to use SR-IOV technology to provide RDMA communication capabilities for containers in RoCE and Infiniband network environments. However, the process can become complex when configuring AI applications with multiple network cards. To simplify this process, Spiderpool supports classifying a set of network card configurations through annotations (`cni.spidernet.io/rdma-resource-inject` or `cni.spidernet.io/network-resource-inject`). Users only need to add the same annotation to the application, and Spiderpool will automatically inject all corresponding network cards and network resources with the same annotation into the application through a webhook. `cni.spidernet.io/rdma-resource-inject` annotation is only applicable to AI scenarios, automatically injecting RDMA network cards and RDMA resources. `cni.spidernet.io/network-resource-inject` annotation can be used not only for AI scenarios but also supports underlay scenarios. In the future, we hope to uniformly use `cni.spidernet.io/network-resource-inject` to support both of these scenarios.

> This feature only supports network card configurations with cniType of [macvlan, ipvlan, sriov, ib-sriov, ipoib].

1. Currently, Spiderpool's webhook for automatically injecting RDMA network resources is disabled by default and needs to be enabled manually.

    ```shell
    ~# helm upgrade --install spiderpool spiderpool/spiderpool --namespace spiderpool --create-namespace --reuse-values --set spiderpoolController.podResourceInject.enabled=true
    ```

    > After enabling the webhook automatic injection of network resources, you can update the configuration by updating the podResourceInject field in configMap: spiderpool-config.
    >
    > Specify namespaces that do not require RDMA network resource injection through `podResourceInject.namespacesExclude`.
    >
    > Specify namespaces that require RDMA network resource injection through `podResourceInject.namespacesInclude`. If neither `podResourceInject.namespacesExclude` nor `podResourceInject.namespacesInclude` is specified, RDMA network resource injection is performed for all namespaces by default.
    >
    > Currently, after completing the configuration change, you need to restart the spiderpool-controller for the configuration to take effect.

2. When creating all SpiderMultusConfig instances for AI computing networks, add an annotation with the key "cni.spidernet.io/rdma-resource-inject" (or "cni.spidernet.io/network-resource-inject") and a customizable value.

    ```yaml
    apiVersion: spiderpool.spidernet.io/v2beta1
    kind: SpiderIPPool
    metadata:
      name: gpu1-net11
    spec:
      gateway: 172.16.11.254
      subnet: 172.16.11.0/16
      ips:
      - 172.16.11.1-172.16.11.200
    ---
    apiVersion: spiderpool.spidernet.io/v2beta1
    kind: SpiderMultusConfig
    metadata:
      name: gpu1-sriov
      namespace: spiderpool
      annotations:
        cni.spidernet.io/rdma-resource-inject: rdma-network
    spec:
      cniType: sriov
      sriov:
        resourceName: spidernet.io/gpu1rdma
        enableRdma: true
      ippools:
        ipv4: ["gpu1-net11"]
    ```

3. When creating an AI application, add the same annotation to the application:

    ```yaml
    ...
    spec:
      template:
        metadata:
          annotations:
            cni.spidernet.io/rdma-resource-inject: rdma-network
    ```

    > Note: When using the webhook automatic injection of network resources feature, do not add other network configuration annotations (such as `k8s.v1.cni.cncf.io/networks` and `ipam.spidernet.io/ippools`) to the application, as it will affect the automatic injection of resources.

4. Once the Pod is created, you can observe that the Pod has been automatically injected with network card annotations and RDMA resources.

    ```yaml
    ...
    spec:
      template:
        metadata:
          annotations:
              k8s.v1.cni.cncf.io/networks: |-
                [{"name":"gpu1-sriov","namespace":"spiderpool"},
                {"name":"gpu2-sriov","namespace":"spiderpool"},
                {"name":"gpu3-sriov","namespace":"spiderpool"},
                {"name":"gpu4-sriov","namespace":"spiderpool"},
                {"name":"gpu5-sriov","namespace":"spiderpool"},
                {"name":"gpu6-sriov","namespace":"spiderpool"},
                {"name":"gpu7-sriov","namespace":"spiderpool"},
                {"name":"gpu8-sriov","namespace":"spiderpool"}]
         ....
         resources:
           limits:
             spidernet.io/gpu1rdma: 1
             spidernet.io/gpu2rdma: 1
             spidernet.io/gpu3rdma: 1
             spidernet.io/gpu4rdma: 1
             spidernet.io/gpu5rdma: 1
             spidernet.io/gpu6rdma: 1
             spidernet.io/gpu7rdma: 1
             spidernet.io/gpu8rdma: 1
    ```

## Customize the MTU of SR-IOV VF

  By default, the MTU of an SR-IOV VF does not inherit the value of its PF. Therefore, in some special communication scenarios, users need to customize the MTU size for Pods to meet the communication requirements of different data packets. You can refer to the following method to customize the Pod's MTU configuration (using Ethernet as an example):

  ```yaml
  apiVersion: spiderpool.spidernet.io/v2beta1
  kind: SpiderMultusConfig
  metadata:
    name: gpu1-sriov
    namespace: spiderpool
  spec:
    cniType: sriov
    sriov:
      resourceName: spidernet.io/gpu1sriov
      mtu: 8000
      enableRdma: true
      ippools:
        ipv4: ["gpu1-net11"]
  ```

  Note: The MTU value should not exceed the MTU value of the sriov PF.
