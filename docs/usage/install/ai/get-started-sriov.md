# AI Cluster With SR-IOV

**English** | [**简体中文**](./get-started-sriov-zh_CN.md)

## Introduction

This section introduces how to provide RDMA communication capabilities to containers based on SR-IOV technology in the context of building AI clusters. Spiderpool uses [the sriov-network-operator](https://github.com/k8snetworkplumbingwg/sriov-network-operator) to provide SR-IOV network interfaces for containers, which can offer RDMA devices suitable for RDMA communication in RoCE and Infiniband networks.

The Linux RDMA subsystem provides two operating modes:

- Shared mode: Containers can see all RDMA devices on the host, including those allocated to other containers.
- Exclusive mode: Containers can only see and use the RDMA devices allocated to them, and cannot see RDMA devices allocated to other containers.
    For isolating RDMA network cards, at least one of the following conditions must be met:

    1. A Linux kernel version 5.3.0 or later, with the RDMA modules loaded in the system. The rdma-core package provides a method to automatically load the relevant modules at system startup.
  
    2. Mellanox OFED version 4.7 or later. In this case, it is not necessary to use a kernel version 5.3.0 or later.

In the scenario of SR-IOV virtual network cards, it can operate in RDMA exclusive mode. The benefit is that different PODs only see their exclusive RDMA devices, and the RDMA device INDEX always starts from 0, preventing confusion in RDMA device selection for applications.

In both Infiniband and Ethernet network scenarios, the related CNIs support RDMA exclusive mode:

- In an Infiniband network scenario, use [the IB-SRIOV](https://github.com/k8snetworkplumbingwg/ib-sriov-cni) CNI to provide SR-IOV network cards for PODs.

- In an Ethernet network scenario, use [SR-IOV CNI](https://github.com/k8snetworkplumbingwg/sriov-cni) to expose the RDMA network cards on the host to the PODs, thereby exposing RDMA resources. Use RDMA CNI to achieve RDMA device isolation.

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

1. Install the RDMA network card driver.

    For Mellanox network cards, you can download [the NVIDIA OFED official driver](https://network.nvidia.com/products/infiniband-drivers/linux/mlnx_ofed/) and install it on the host using the following installation command:

        $ mount /root/MLNX_OFED_LINUX-24.01-0.3.3.1-ubuntu22.04-x86_64.iso   /mnt
        $ /mnt/mlnxofedinstall --all

    For Mellanox network cards, you can also perform a containerized installation to batch install drivers on all Mellanox network cards in the cluster hosts. Run the following command. Note that this process requires internet access to fetch some installation packages. When all the OFED pods enter the ready state, it indicates that the OFED driver installation on the hosts is complete:

        $ helm repo add spiderchart https://spidernet-io.github.io/charts
        $ helm repo update
        $ helm search repo ofed
          NAME                 	        CHART VERSION	APP VERSION	DESCRIPTION
          spiderchart/ofed-driver            24.04.0      	24.04.0    	ofed driver

        # pelase replace the following values with your actual environment
        # for china user, it could set `--set image.registry=nvcr.m.daocloud.io` to use a domestic registry
        $ helm install ofed-driver spiderchart/ofed-driver -n kube-system \
            --set image.OSName="ubuntu" \
            --set image.OSVer="22.04" \
            --set image.Arch="amd64"

2. Verify that the network card supports Infiniband or Ethernet operating modes.

    In this example environment, the host is equipped with Mellanox ConnectX 5 VPI network cards. Query the RDMA devices to confirm that the network card driver is installed correctly.

        $ rdma link
          link mlx5_0/1 state ACTIVE physical_state LINK_UP netdev ens6f0np0
          link mlx5_1/1 state ACTIVE physical_state LINK_UP netdev ens6f1np1
          ....... 

    Verify the network card's operating mode. The following output indicates that the network card is operating in Ethernet mode and can achieve RoCE communication:

        $ ibstat mlx5_0 | grep "Link layer"
          Link layer: Ethernet

    The following output indicates that the network card is operating in Infiniband mode and can achieve Infiniband communication:

        $ ibstat mlx5_0 | grep "Link layer"
          Link layer: InfiniBand

    If the network card is not operating in the expected mode, enter the following command to verify that the network card supports configuring the LINK_TYPE parameter. If the parameter is not available, please switch to a supported network card model:

        $ mst start

        # check the card's PCIE 
        $ lspci -nn | grep Mellanox
          86:00.0 Infiniband controller [0207]: Mellanox Technologies MT27800 Family [ConnectX-5] [15b3:1017]
          86:00.1 Infiniband controller [0207]: Mellanox Technologies MT27800 Family [ConnectX-5] [15b3:1017]
          ....... 

        # check whether the network card supports parameters LINK_TYPE 
        $ mlxconfig -d 86:00.0  q | grep LINK_TYPE
          LINK_TYPE_P1                                IB(1)

3. Enable [GPUDirect RDMA](https://docs.nvidia.com/cuda/gpudirect-rdma/)

    The installation of the [gpu-operator](https://github.com/NVIDIA/gpu-operator): 

    a.  Enable the Helm installation options: `--set driver.rdma.enabled=true --set driver.rdma.useHostMofed=true`. The gpu-operator will install [the nvidia-peermem](https://network.nvidia.com/products/GPUDirect-RDMA/) kernel module, 
    enabling GPUDirect RDMA functionality to accelerate data transfer performance between the GPU and RDMA network cards. Enter the following command on the host to confirm the successful installation of the kernel module:

        $ lsmod | grep nvidia_peermem
           nvidia_peermem         16384  0

   b. Enable the Helm installation option: `--set gdrcopy.enabled=true`. The gpu-operator will install the [gdrcopy](https://network.nvidia.com/products/GPUDirect-RDMA/) kernel module to accelerate data transfer performance between GPU memory and CPU memory. Enter the following command on the host to confirm the successful installation of the kernel module:

        $ lsmod | grep gdrdrv
           gdrdrv                 24576  0

4. Set the RDMA subsystem on the host to exclusive mode, allowing containers to independently use RDMA devices and avoiding sharing with other containers.

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

## Install Spiderpool

1. Use Helm to install Spiderpool and enable the SR-IOV component:

    ```
    $ helm repo add spiderpool https://spidernet-io.github.io/spiderpool
    $ helm repo update spiderpool
    $ kubectl create namespace spiderpool
    $ helm install spiderpool spiderpool/spiderpool -n spiderpool --set sriov.install=true
    ```

    > If you are a user in China, you can specify the helm option `--set global.imageRegistryOverride=ghcr.m.daocloud.io` to use a domestic image source.

    After completion, the installed components are as follows:

    ```
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

    ```
    $ lspci -nn | grep Mellanox
        86:00.0 Infiniband controller [0207]: Mellanox Technologies MT27800 Family [ConnectX-5] [15b3:1017]
        86:00.1 Infiniband controller [0207]: Mellanox Technologies MT27800 Family [ConnectX-5] [15b3:1017]
        ....
    ```

    The number of SR-IOV VFs (Virtual Functions) determines how many PODs a network card can simultaneously support. Different models of network cards have different maximum VF limits. For example, Mellanox's ConnectX series network cards typically have a maximum VF limit of 127.

    In the following example, we set up the network cards of GPU1 and GPU2 on each node, configuring 12 VFs for each card. Refer to the following configuration to set up the SriovNetworkNodePolicy for each network card associated with a GPU on the host. This setup will provide 8 SR-IOV resources for use.

    ```
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

    ```
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

    ```
    $ kubectl get node
        NAME           STATUS                     ROLES                  AGE     VERSION
        ai-10-1-16-1   Ready                      worker                 2d15h   v1.28.9
        ai-10-1-16-2   Ready,SchedulingDisabled   worker                 2d15h   v1.28.9
        .......
    ```
   
    It may take several minutes for all nodes to complete the VF configuration process. You can monitor the sriovnetworknodestates status to see if it has entered the Succeeded state, indicating that the configuration is complete.

    ```
    $ kubectl get sriovnetworknodestates -A
        NAMESPACE        NAME           SYNC STATUS   DESIRED SYNC STATE   CURRENT SYNC STATE   AGE
        spiderpool       ai-10-1-16-1   Succeeded     Idle                 Idle                 4d6h
        spiderpool       ai-10-1-16-2   Succeeded     Idle                 Idle                 4d6h
        .......
    ```

    For nodes that have successfully configured VFs, you can check the available resources of the node, including the reported SR-IOV device resources.

    ```
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

4. Create CNI Configuration and Corresponding IP Pool Resources

    a. For Infiniband Networks, configure [the IB-SRIOV CNI](https://github.com/k8snetworkplumbingwg/ib-sriov-cni)  for all GPU-affinitized SR-IOV network cards and create the corresponding IP address pool. The following example configures the network card and IP address pool for GPU1

    ```
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
            ippools:
              ipv4: ["gpu1-net91"]
    EOF
    ```

    b. For Ethernet Networks, configure [the SR-IOV CNI](https://github.com/k8snetworkplumbingwg/sriov-cni) for all GPU-affinitized SR-IOV network cards and create the corresponding IP address pool. The following example configures the network card and IP address pool for GPU1

    ```   
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

## Create a Test Application

1. Create a DaemonSet application on a specified node to test the availability of SR-IOV devices on that node. 
    In the following example, the annotation field `v1.multus-cni.io/default-network` specifies the use of the default Calico network card for control plane communication. The annotation field `k8s.v1.cni.cncf.io/networks` connects to the 8 VF network cards affinitized to the GPU for RDMA communication, and configures 8 types of RDMA resources.

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
    0:	from all lookup local
    32762:	from 172.16.11.10 lookup 107
    32763:	from 172.16.12.10 lookup 106
    32764:	from 172.16.13.10 lookup 105
    32765:	from 172.16.14.10 lookup 104
    32765:	from 172.16.15.10 lookup 103
    32765:	from 172.16.16.10 lookup 102
    32765:	from 172.16.17.10 lookup 101
    32765:	from 172.16.18.10 lookup 100
    32766:	from all lookup main
    32767:	from all lookup default
    root@rdma-tools-4v8t8:/# ip route show table 100
        default via 172.16.11.254 dev net1
    ```

    In the main routing table, ensure that Calico network traffic, ClusterIP traffic, and local host communication traffic are all forwarded through the Calico network card.

    ```
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

    ```
    root@rdma-tools-4v8t8:/# rdma link
        link mlx5_27/1 state ACTIVE physical_state LINK_UP netdev net2
        link mlx5_54/1 state ACTIVE physical_state LINK_UP netdev net1
        link mlx5_67/1 state ACTIVE physical_state LINK_UP netdev net4
        link mlx5_98/1 state ACTIVE physical_state LINK_UP netdev net3
        .....
    ```

3. Confirm that RDMA data transmission is functioning properly between Pods across nodes.

    Open a terminal, enter a Pod, and start the service:

    ```
    # see 8 RDMA devices assigned to the Pod
    $ rdma link

    # Start an RDMA service
    $ ib_read_lat
    ```
   
    Open another terminal, enter another Pod, and access the service:

    ```
    # You should be able to see all RDMA network cards on the host
    $ rdma link
        
    # Successfully access the RDMA service of the other Pod
    $ ib_read_lat 172.91.0.115
    ```

## (Optional) Integrate with UFM on Infiniband Networks

For clusters using Infiniband networks, if there is a [UFM management platform](https://www.nvidia.com/en-us/networking/infiniband/ufm/) in the network, you can use [the ib-kubernetes plugin](https://github.com/Mellanox/ib-kubernetes). This plugin runs as a daemonset, monitoring all containers using SRIOV network cards and reporting the Pkey and GUID of VF devices to UFM.

1. Create the necessary certificates for communication on the UFM host:

    ```
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

    ```
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

    ```
    $ git clone https://github.com/Mellanox/ib-kubernetes.git && cd ib-kubernetes
    $ $ kubectl create -f deployment/ib-kubernetes-configmap.yaml
    $ kubectl create -f deployment/ib-kubernetes.yaml 
    ```

4. On Infiniband networks, when creating Spiderpool's SpiderMultusConfig, you can configure the Pkey. Pods created with this configuration will use the Pkey settings and be synchronized with UFM by ib-kubernetes

    ```
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
