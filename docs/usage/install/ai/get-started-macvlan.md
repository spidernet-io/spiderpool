# AI Cluster With Macvlan

**English** | [**简体中文**](./get-started-macvlan-zh_CN.md)

## Introduction

This section explains how to provide RDMA communication capabilities to containers using Macvlan technology in the context of building an AI cluster, applicable in RoCE network scenarios.

By using [RDMA shared device plugin](https://github.com/Mellanox/k8s-rdma-shared-dev-plugin), a Macvlan interface can be attached to a container, allowing the RDMA device on the master interface to be shared with the container. Therefore:

- The RDMA system needs to operate in shared mode, where all containers share the RDMA device of the host's master network interface. A key characteristic of this setup is that in each newly launched container, the available GID index of the RDMA device continuously increments and is not a fixed value.

- Macvlan interfaces cannot be created on an Infiniband IPOIB network card, so this solution is only applicable in RoCE network scenarios and cannot be used in Infiniband network scenarios.

## Solution

This article will introduce how to set up Spiderpool using the following typical AI cluster topology as an example.

![AI Cluster](../../../images/ai-cluster.png)

Figure 1: AI Cluster Topology

The network planning for the cluster is as follows:

1. The calico CNI runs on the eth0 network card of the nodes to carry Kubernetes traffic. The AI workload will be assigned a default calico network interface for control plane communication.

2. The nodes use Mellanox ConnectX5 network cards with RDMA functionality to carry the RDMA traffic for AI computation. The network cards are connected to a rail-optimized network. The AI workload will be additionally assigned Macvlan virtualized interfaces for all RDMA network cards to ensure high-speed network communication for the GPUs.

## Installation Requirements

- Refer to [the Spiderpool Installation Requirements](./../system-requirements.md).

- Prepare the Helm binary on the host.

- Install a Kubernetes cluster with kubelet running on the host’s eth0 network card as shown in [Figure 1](#solution).

- Install Calico as the default CNI for the cluster, using the host’s eth0 network card for Calico’s traffic forwarding.
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

1. Install the RDMA network card driver.

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

2. Verify that the network card supports Ethernet operating modes.

    In this example environment, the host is equipped with Mellanox ConnectX 5 VPI network cards. Query the RDMA devices to confirm that the network card driver is installed correctly.

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

3. Enable [GPUDirect RDMA](https://docs.nvidia.com/cuda/gpudirect-rdma/)

    The installation of the [gpu-operator](https://github.com/NVIDIA/gpu-operator):

    1. Enable the Helm installation options: `--set driver.rdma.enabled=true --set driver.rdma.useHostMofed=true`. The gpu-operator will install [the nvidia-peermem](https://network.nvidia.com/products/GPUDirect-RDMA/) kernel module,
       enabling GPUDirect RDMA functionality to accelerate data transfer performance between the GPU and RDMA network cards. Enter the following command on the host to confirm the successful installation of the kernel module:

        ```shell
        $ lsmod | grep nvidia_peermem
          nvidia_peermem         16384  0
        ```

    2. Enable the Helm installation option: `--set gdrcopy.enabled=true`. The gpu-operator will install the [gdrcopy](https://network.nvidia.com/products/GPUDirect-RDMA/) kernel module to accelerate data transfer performance between GPU memory and CPU memory. Enter the following command on the host to confirm the successful installation of the kernel module:

        ```shell
        $ lsmod | grep gdrdrv
          gdrdrv                 24576  0
        ```

4. Set the RDMA subsystem on the host to shared mode, allowing containers to independently use shared RDMA device.

    ```shell
    # Check the current operating mode (the Linux RDMA subsystem operates in shared mode by default):
    $ rdma system
       netns shared copy-on-fork on
    ```

## Install Spiderpool

1. Use Helm to install Spiderpool and enable the rdmaSharedDevicePlugin:

    ```shell
    helm repo add spiderpool https://spidernet-io.github.io/spiderpool
    helm repo update spiderpool
    kubectl create namespace spiderpool
    helm install spiderpool spiderpool/spiderpool -n spiderpool --set rdma.rdmaSharedDevicePlugin.install=true
    ```

    > If you are a user in China, you can specify the helm option `--set global.imageRegistryOverride=ghcr.m.daocloud.io` to use a domestic image source.
    > Set `--set spiderpoolAgent.prometheus.enabled --set spiderpoolAgent.prometheus.enabledRdmaMetric=true` and `--set grafanaDashboard.install=true` flag to enable the RDMA metrics exporter and Grafana dashboard. Refer to [RDMA metrics](../../rdma-metrics.md).

    After completion, the installed components are as follows:

    ```shell
    $ kubectl get pod -n spiderpool
        spiderpool-agent-9sllh                         1/1     Running     0          1m
        spiderpool-agent-h92bv                         1/1     Running     0          1m
        spiderpool-controller-7df784cdb7-bsfwv         1/1     Running     0          1m
        spiderpool-init                                0/1     Completed   0          1m
        spiderpool-rdma-shared-device-plugin-9xsm9     1/1     Running     0          1m
        spiderpool-rdma-shared-device-plugin-nxvlx     1/1     Running     0          1m
    ```

2. Configure k8s-rdma-shared-dev-plugin

    Modify the following ConfigMap to create eight types of RDMA shared devices, each associated with a specific GPU device. For detailed configuration of the ConfigMap, refer to [the official documentation](https://github.com/Mellanox/k8s-rdma-shared-dev-plugin?tab=readme-ov-file#rdma-shared-device-plugin-configurations).

    ```shell
    $ kubectl edit configmap -n spiderpool spiderpool-rdma-shared-device-plugi
      ....
      config.json: |
        {
         "periodicUpdateInterval": 300,
         "configList": [
            {
             "resourcePrefix": "spidernet.io",
             "resourceName": "shared_cx5_gpu1",
             "rdmaHcaMax": 100,
             "selectors": { "ifNames": ["enp11s0f0np0"] }
           },
           ....
           {
             "resourcePrefix": "spidernet.io",
             "resourceName": "shared_cx5_gpu8",
             "rdmaHcaMax": 100,
             "selectors": { "ifNames": ["enp18s0f0np0"] }
           }
         ]
    ```

    After completing the above configuration, you can check the available resources on the node to confirm that each node has correctly recognized and reported the eight types of RDMA device resources.

    ```shell
    $ kubectl get no -o json | jq -r '[.items[] | {name:.metadata.name, allocable:.status.allocatable}]'
        [
          {
            "name": "ai-10-1-16-1",
            "allocable": {
              "cpu": "40",
              "pods": "110",
              "spidernet.io/shared_cx5_gpu1": "100",
              "spidernet.io/shared_cx5_gpu2": "100",
              ...
              "spidernet.io/shared_cx5_gpu8": "100",
              ...
            }
          },
          ...
        ]
    ```

    <a id="create-spiderpool-resource"></a>

3. Create CNI configuration and proper IP pool resources

    For Ethernet networks, please configure the Macvlan network interfaces associated with all GPUs and create corresponding IP address pools. The example below shows the configuration for the network interface and IP address pool associated with GPU1.

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
      name: gpu1-macvlan
      namespace: spiderpool
    spec:
      cniType: macvlan
      macvlan:
        master: ["enp11s0f0np0"]
        ippools:
          ipv4: ["gpu1-net11"]
    EOF
    ```

## Create a Test Application

1. Create a DaemonSet application on specified nodes.

    In the following example, the annotation field `v1.multus-cni.io/default-network` specifies the use of the default Calico network card for control plane communication. The annotation field `k8s.v1.cni.cncf.io/networks` connects to the 8 network cards affinitized to the GPU for RDMA communication, and configures 8 types of RDMA resources.

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

    # interfaces
    extraAnnotations:
    k8s.v1.cni.cncf.io/networks: |-
      [{"name":"gpu1-macvlan","namespace":"spiderpool"},
      {"name":"gpu2-macvlan","namespace":"spiderpool"},
      {"name":"gpu3-macvlan","namespace":"spiderpool"},
      {"name":"gpu4-macvlan","namespace":"spiderpool"},
      {"name":"gpu5-macvlan","namespace":"spiderpool"},
      {"name":"gpu6-macvlan","namespace":"spiderpool"},
      {"name":"gpu7-macvlan","namespace":"spiderpool"},
      {"name":"gpu8-macvlan","namespace":"spiderpool"}]
    # macvlan resource
    resources:
      requests:
        spidernet.io/shared_cx5_gpu1: 1
        spidernet.io/shared_cx5_gpu2: 1
        spidernet.io/shared_cx5_gpu3: 1
        spidernet.io/shared_cx5_gpu4: 1
        spidernet.io/shared_cx5_gpu5: 1
        spidernet.io/shared_cx5_gpu6: 1
        spidernet.io/shared_cx5_gpu7: 1
        spidernet.io/shared_cx5_gpu8: 1
        #nvidia.com/gpu: 1
    ```

    During the creation of the network namespace for the container, Spiderpool will perform connectivity tests on the gateway of the macvlan interface.
    If all PODs of the above application start successfully, it indicates successful connectivity of the network cards on each node, allowing normal RDMA communication.

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
      cniType: macvlan
      macvlan:
        master: ["enp11s0f0np0"]
        enableRdma: true
        rdmaResourceName: spidernet.io/gpu1rdma
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
