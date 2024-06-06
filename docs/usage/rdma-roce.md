# RDMA with RoCE

**English** ｜ [**简体中文**](./rdma-roce-zh_CN.md)

## Introduction

This chapter introduces how POD access network with the RoCE interface of the host.

## Features

RDMA devices' network namespaces have two modes: shared and exclusive. Containers can either share or exclusively access RDMA network cards. In Kubernetes, shared cards can be utilized with macvlan or ipvlan CNI, while the exclusive one can be used with SR-IOV CNI.

- Shared mode. Spiderpool leverages macvlan or ipvlan CNI to expose RoCE network cards on the host machine for all Pods. The [RDMA shared device plugin](https://github.com/Mellanox/k8s-rdma-shared-dev-plugin) is employed for exposing RDMA card resources and scheduling Pods.

- Exclusive mode. Spiderpool utilizes [SR-IOV CNI](https://github.com/k8snetworkplumbingwg/sriov-network-operator) to expose RDMA cards on the host machine for Pods, providing access to RDMA resources. [RDMA CNI](https://github.com/k8snetworkplumbingwg/rdma-cni) is used to ensure isolation of RDMA devices.

    For isolated RDMA network cards, at least one of the following conditions must be met:

    (1) Kernel based on 5.3.0 or newer, RDMA modules loaded in the system. rdma-core package provides means to automatically load relevant modules on system start
   
    (2) Mellanox OFED version 4.7 or newer is required. In this case it is not required to use a Kernel based on 5.3.0 or newer.

## Shared RoCE NIC with macvlan or ipvlan

The following steps demonstrate how to enable shared usage of RDMA devices by Pods in a cluster with two nodes via macvlan CNI:

1. Ensure that the host machine has an RDMA card installed and the driver is properly installed, ensuring proper RDMA functioning.

    In our demo environment, the host machine is equipped with a Mellanox ConnectX-5 NIC with RoCE capabilities. Follow [the official NVIDIA guide](https://developer.nvidia.com/networking/ethernet-software) to install the latest OFED driver. To confirm the presence of RDMA devices, use the following command:

    To confirm the presence of RoCE devices, use the following command:

        ~# rdma link
        link mlx5_0/1 state ACTIVE physical_state LINK_UP netdev ens6f0np0
        link mlx5_1/1 state ACTIVE physical_state LINK_UP netdev ens6f1np1

        ~# ibstat mlx5_0 | grep "Link layer"
        Link layer: Ethernet

   Make sure that the RDMA subsystem of the host is in shared mode. If not, switch to shared mode.

        ~# rdma system
        netns shared copy-on-fork on

        # switch to shared mode
        ~# rdma system set netns shared

2. Verify the details of the RDMA card for subsequent device resource discovery by the device plugin.

    Enter the following command with NIC vendors being 15b3 and its deviceIDs being 1017:

        ~# lspci -nn | grep Ethernet
        af:00.0 Ethernet controller [0200]: Mellanox Technologies MT27800 Family [ConnectX-5] [15b3:1017]
        af:00.1 Ethernet controller [0200]: Mellanox Technologies MT27800 Family [ConnectX-5] [15b3:1017]

3. Install Spiderpool and configure sriov-network-operator:

        helm upgrade --install spiderpool spiderpool/spiderpool --namespace kube-system  --reuse-values \
           --set rdma.rdmaSharedDevicePlugin.install=true \
           --set rdma.rdmaSharedDevicePlugin.deviceConfig.resourcePrefix="spidernet.io" \
           --set rdma.rdmaSharedDevicePlugin.deviceConfig.resourceName="hca_shared_devices" \
           --set rdma.rdmaSharedDevicePlugin.deviceConfig.rdmaHcaMax=500 \
           --set rdma.rdmaSharedDevicePlugin.deviceConfig.vendors="15b3" \
           --set rdma.rdmaSharedDevicePlugin.deviceConfig.deviceIDs="1017"
    
    > - If Macvlan is not installed in your cluster, you can specify the Helm parameter `--set plugins.installCNI=true` to install Macvlan in your cluster.
    >
    > - If you are a user from China, you can specify the parameter `--set global.imageRegistryOverride=ghcr.m.daocloud.io` to avoid image pull failures from Spiderpool.
    
    After completing the installation of Spiderpool, you can manually edit the spiderpool-rdma-shared-device-plugin configmap to reconfigure the RDMA shared device plugin.
    
    Once the installation is complete, the following components will be installed:

        ~# kubectl get pod -n kube-system
        spiderpool-agent-9sllh                         1/1     Running     0          1m
        spiderpool-agent-h92bv                         1/1     Running     0          1m
        spiderpool-controller-7df784cdb7-bsfwv         1/1     Running     0          1m
        spiderpool-init                                0/1     Completed   0          1m
        spiderpool-rdma-shared-device-plugin-dr7w8     1/1     Running     0          1m
        spiderpool-rdma-shared-device-plugin-zj65g     1/1     Running     0          1m

4. View the available resources on a node, including the reported RDMA device resources:

        ~# kubectl get no -o json | jq -r '[.items[] | {name:.metadata.name, allocable:.status.allocatable}]'
          [
            {
              "name": "10-20-1-10",
              "allocable": {
                "cpu": "40",
                "memory": "263518036Ki",
                "pods": "110",
                "spidernet.io/hca_shared_devices": "500",
                ...
              }
            },
            ...
          ]

    > If the reported resource count is 0, it may be due to the following reasons:
    >
    > (1) Verify that the vendors and deviceID in the spiderpool-rdma-shared-device-plugin configmap match the actual values.
    >
    > (2) Check the logs of the rdma-shared-device-plugin. If you encounter errors related to RDMA NIC support, try installing apt-get install rdma-core or dnf install rdma-core on the host machine.
    >
    >   `error creating new device: "missing RDMA device spec for device 0000:04:00.0, RDMA device \"issm\" not found"`

5. Create macvlan CNI configuration with specifying `spec.macvlan.master` to be an RDMA of the node ,and set up the corresponding ippool resources:

        cat <<EOF | kubectl apply -f -
        apiVersion: spiderpool.spidernet.io/v2beta1
        kind: SpiderIPPool
        metadata:
          name: v4-81
        spec:
          gateway: 172.81.0.1
          ips:
          - 172.81.0.100-172.81.0.120
          subnet: 172.81.0.0/16
        ---
        apiVersion: spiderpool.spidernet.io/v2beta1
        kind: SpiderMultusConfig
        metadata:
          name: macvlan-ens6f0np0
          namespace: kube-system
        spec:
          cniType: macvlan
          macvlan:
            master:
            - "ens6f0np0"
            ippools:
              ipv4: ["v4-81"]
        EOF

6. Following the configurations from the previous step, create a DaemonSet application that spans across nodes for testing

        ANNOTATION_MULTUS="v1.multus-cni.io/default-network: kube-system/macvlan-ens6f0np0"
        RESOURCE="spidernet.io/hca_shared_devices"
        NAME=rdma-macvlan
        cat <<EOF | kubectl apply -f -
        apiVersion: apps/v1
        kind: DaemonSet
        metadata:
          name: ${NAME}
          labels:
            app: $NAME
        spec:
          selector:
            matchLabels:
              app: $NAME
          template:
            metadata:
              name: $NAME
              labels:
                app: $NAME
              annotations:
                ${ANNOTATION_MULTUS}
            spec:
              containers:
              - image: docker.io/mellanox/rping-test
                imagePullPolicy: IfNotPresent
                name: mofed-test
                securityContext:
                  capabilities:
                    add: [ "IPC_LOCK" ]
                resources:
                  limits:
                    ${RESOURCE}: 1
                command:
                - sh
                - -c
                - |
                  ls -l /dev/infiniband /sys/class/net
                  sleep 1000000
        EOF

7. Verify that RDMA communication is correct between the Pods across nodes.

    Open a terminal and access one Pod to launch a service:

        # You are able to see all the RDMA cards on the host machine
        ~# rdma link
        0/1: mlx5_0/1: state ACTIVE physical_state LINK_UP
        1/1: mlx5_1/1: state ACTIVE physical_state LINK_UP
        
        # Start an RDMA service
        ~# ib_read_lat

    Open a terminal and access another Pod to launch a service:

        # You are able to see all the RDMA cards on the host machine
        ~# rdma link
        0/1: mlx5_0/1: state ACTIVE physical_state LINK_UP
        1/1: mlx5_1/1: state ACTIVE physical_state LINK_UP
        
        # Access the service running in the other Pod
        ~# ib_read_lat 172.81.0.120
        ---------------------------------------------------------------------------------------
                            RDMA_Read Latency Test
         Dual-port       : OFF    Device         : mlx5_0
         Number of qps   : 1    Transport type : IB
         Connection type : RC   Using SRQ      : OFF
         TX depth        : 1
         Mtu             : 1024[B]
         Link type       : Ethernet
         GID index       : 12
         Outstand reads  : 16
         rdma_cm QPs   : OFF
         Data ex. method : Ethernet
        ---------------------------------------------------------------------------------------
         local address: LID 0000 QPN 0x0107 PSN 0x79dd10 OUT 0x10 RKey 0x1fddbc VAddr 0x000000023bd000
         GID: 00:00:00:00:00:00:00:00:00:00:255:255:172:81:00:119
         remote address: LID 0000 QPN 0x0107 PSN 0x40001a OUT 0x10 RKey 0x1fddbc VAddr 0x00000000bf9000
         GID: 00:00:00:00:00:00:00:00:00:00:255:255:172:81:00:120
        ---------------------------------------------------------------------------------------
         #bytes #iterations    t_min[usec]    t_max[usec]  t_typical[usec]    t_avg[usec]    t_stdev[usec]   99% percentile[usec]   99.9% percentile[usec]
        Conflicting CPU frequency values detected: 2200.000000 != 1040.353000. CPU Frequency is not max.
        Conflicting CPU frequency values detected: 2200.000000 != 1849.351000. CPU Frequency is not max.
         2       1000          6.88           16.81        7.04              7.06         0.31      7.38        16.81
        ---------------------------------------------------------------------------------------

## Isolated RoCE NIC with SR-IOV

The following steps demonstrate how to enable isolated usage of RDMA devices by Pods in a cluster with two nodes via SR-IOV CNI:

1. Ensure that the host machine has an RDMA and SR-IOV enabled card and the driver is properly installed.

    In our demo environment, the host machine is equipped with a Mellanox ConnectX-5 NIC with RoCE capabilities. Follow [the official NVIDIA guide](https://developer.nvidia.com/networking/ethernet-software) to install the latest OFED driver.

    To confirm the presence of RoCE devices, use the following command:

        ~# rdma link
        link mlx5_0/1 state ACTIVE physical_state LINK_UP netdev ens6f0np0
        link mlx5_1/1 state ACTIVE physical_state LINK_UP netdev ens6f1np1

        ~# ibstat mlx5_0 | grep "Link layer"
        Link layer: Ethernet

    Make sure that the RDMA subsystem on the host is operating in exclusive mode. If not, switch to exclusive mode.

        # switch to exclusive mode and fail to restart the host 
        ~# rdma system set netns exclusive
        # apply persistent settings: 
        ~# echo "options ib_core netns_mode=0" >> /etc/modprobe.d/ib_core.conf

        ~# rdma system
        netns exclusive copy-on-fork on

    (Optional) in an SR-IOV scenario, applications can enable NVIDIA's GPUDirect RDMA feature. For instructions on installing the kernel module, please refer to [the official documentation](https://network.nvidia.com/products/GPUDirect-RDMA/).

2. Install Spiderpool

    - set the values `--set sriov.install=true`

    - If you are a user from China, you can specify the parameter `--set global.imageRegistryOverride=ghcr.m.daocloud.io` to pull image from china registry.
    
    After completing the installation of Spiderpool, you can manually edit the spiderpool-rdma-shared-device-plugin configmap to reconfigure the RDMA shared device plugin.
    
    Once the installation is complete, the following components will be installed:
        ~# kubectl get pod -n kube-system
        spiderpool-agent-9sllh                         1/1     Running     0          1m
        spiderpool-agent-h92bv                         1/1     Running     0          1m
        spiderpool-controller-7df784cdb7-bsfwv         1/1     Running     0          1m
        spiderpool-sriov-operator-65b59cd75d-89wtg     1/1     Running     0          1m
        spiderpool-init                                0/1     Completed   0          1m

3. Configure SR-IOV operator

    Look up the device information of the RoCE interface. Enter the following command to get NIC vendors 15b3 and deviceIDs 1017

        ~# lspci -nn | grep Ethernet
        af:00.0 Ethernet controller [0200]: Mellanox Technologies MT27800 Family [ConnectX-5] [15b3:1017]
        af:00.1 Ethernet controller [0200]: Mellanox Technologies MT27800 Family [ConnectX-5] [15b3:1017]

    By the way, the number of VFs determines how many SR-IOV network cards can be provided for PODs on a host. The network card from different manufacturers have different amount limit of VFs. For example, the Mellanox connectx5 used in this example can create up to 127 VFs.

    Apply the following configuration, and the VFs will be created on the host. Notice, this may cause the nodes to reboot, owing to taking effect the new configuration in the network card driver.

        cat <<EOF | kubectl apply -f -
        apiVersion: sriovnetwork.openshift.io/v1
        kind: SriovNetworkNodePolicy
        metadata:
          name: roce-sriov
          namespace: kube-system
        spec:
          nodeSelector:
            kubernetes.io/os: "linux"
          resourceName: mellanoxroce
          priority: 99
          numVfs: 12
          nicSelector:
              deviceID: "1017"
              rootDevices:
              - 0000:af:00.0
              vendor: "15b3"
          deviceType: netdevice
          isRdma: true
        EOF

    Verify the available resources on the node, including the reported SR-IOV device resources:

        ~# kubectl get no -o json | jq -r '[.items[] | {name:.metadata.name, allocable:.status.allocatable}]'
        [
          {
            "name": "10-20-1-10",
            "allocable": {
              "cpu": "40",
              "pods": "110",
              "spidernet.io/mellanoxroce": "12",
              ...
            }
          },
          ...
        ]

4. Create macvlan CNI configuration and corresponding ippool resources.

        cat <<EOF | kubectl apply -f -
        apiVersion: spiderpool.spidernet.io/v2beta1
        kind: SpiderIPPool
        metadata:
          name: v4-81
        spec:
          gateway: 172.81.0.1
          ips:
          - 172.81.0.100-172.81.0.120
          subnet: 172.81.0.0/16
        ---
        apiVersion: spiderpool.spidernet.io/v2beta1
        kind: SpiderMultusConfig
        metadata:
          name: roce-sriov
          namespace: kube-system
        spec:
          cniType: sriov
          sriov:
            resourceName: spidernet.io/mellanoxroce
            enableRdma: true
            ippools:
              ipv4: ["v4-81"]
        EOF

5. Following the configurations from the previous step, create a DaemonSet application that spans across nodes for testing

        ANNOTATION_MULTUS="v1.multus-cni.io/default-network: kube-system/roce-sriov"
        RESOURCE="spidernet.io/mellanoxroce"
        NAME=rdma-sriov
        cat <<EOF | kubectl apply -f -
        apiVersion: apps/v1
        kind: DaemonSet
        metadata:
          name: ${NAME}
          labels:
            app: $NAME
        spec:
          selector:
            matchLabels:
              app: $NAME
          template:
            metadata:
              name: $NAME
              labels:
                app: $NAME
              annotations:
                ${ANNOTATION_MULTUS}
            spec:
              containers:
              - image: docker.io/mellanox/rping-test
                imagePullPolicy: IfNotPresent
                name: mofed-test
                securityContext:
                  capabilities:
                    add: [ "IPC_LOCK" ]
                resources:
                  limits:
                    ${RESOURCE}: 1
                command:
                - sh
                - -c
                - |
                  ls -l /dev/infiniband /sys/class/net
                  sleep 1000000
        EOF

6. Verify that RDMA communication is correct between the Pods across nodes.

    Open a terminal and access one Pod to launch a service:

        # Only one RDMA device allocated to the Pod can be found
        ~# rdma link
        7/1: mlx5_3/1: state ACTIVE physical_state LINK_UP netdev eth0
        
        # launch an RDMA service
        ~# ib_read_lat

    Open a terminal and access another Pod to launch a service:

        # You are able to see all the RDMA cards on the host machine
        ~# rdma link
        10/1: mlx5_5/1: state ACTIVE physical_state LINK_UP netdev eth0
        
        # Access the service running in the other Pod
        ~# ib_read_lat 172.81.0.118
        libibverbs: Warning: couldn't stat '/sys/class/infiniband/mlx5_4'.
        libibverbs: Warning: couldn't stat '/sys/class/infiniband/mlx5_2'.
        libibverbs: Warning: couldn't stat '/sys/class/infiniband/mlx5_0'.
        libibverbs: Warning: couldn't stat '/sys/class/infiniband/mlx5_3'.
        libibverbs: Warning: couldn't stat '/sys/class/infiniband/mlx5_1'.
        ---------------------------------------------------------------------------------------
                            RDMA_Read Latency Test
         Dual-port       : OFF    Device         : mlx5_5
         Number of qps   : 1    Transport type : IB
         Connection type : RC   Using SRQ      : OFF
         TX depth        : 1
         Mtu             : 1024[B]
         Link type       : Ethernet
         GID index       : 2
         Outstand reads  : 16
         rdma_cm QPs   : OFF
         Data ex. method : Ethernet
        ---------------------------------------------------------------------------------------
         local address: LID 0000 QPN 0x0b69 PSN 0xd476c2 OUT 0x10 RKey 0x006f00 VAddr 0x00000001f91000
         GID: 00:00:00:00:00:00:00:00:00:00:255:255:172:81:00:105
         remote address: LID 0000 QPN 0x0d69 PSN 0xbe5c89 OUT 0x10 RKey 0x004f00 VAddr 0x0000000160d000
         GID: 00:00:00:00:00:00:00:00:00:00:255:255:172:81:00:118
        ---------------------------------------------------------------------------------------
         #bytes #iterations    t_min[usec]    t_max[usec]  t_typical[usec]    t_avg[usec]    t_stdev[usec]   99% percentile[usec]   99.9% percentile[usec]
        Conflicting CPU frequency values detected: 2200.000000 != 1338.151000. CPU Frequency is not max.
        Conflicting CPU frequency values detected: 2200.000000 != 2881.668000. CPU Frequency is not max.
         2       1000          6.66           20.37        6.74              6.82         0.78      7.15        20.37
        ---------------------------------------------------------------------------------------
