# RDMA with Infiniband

**English** ｜ [**简体中文**](./rdma-ib-zh_CN.md)

## Introduction

This chapter introduces how POD access network with the infiniband interface of the host. 

## Features

Different from RoCE, Infiniband network cards are proprietary devices for the Infiniband network, and the Spiderpool offers two CNI options:

1. [IB-SRIOV CNI](https://github.com/k8snetworkplumbingwg/ib-sriov-cni) provides SR-IOV network card with the RDMA device. It is suitable for workloads requiring RDMA communication.

    It offers two RDMA modes:

    - Shared mode: Pod will have a SR-IOV network interface with RDMA feature, but all RDMA devices cloud be seen by all PODs running in the same node. POD may be confused for which RDMA device it should use. 

    - Exclusive mode: Pod will have a SR-IOV network interface with RDMA feature, and POD just enable to see its own RDMA device. 

        For isolated RDMA network cards, at least one of the following conditions must be met:

        (1) Kernel based on 5.3.0 or newer, RDMA modules loaded in the system. rdma-core package provides means to automatically load relevant modules on system start
   
        (2) Mellanox OFED version 4.7 or newer is required. In this case it is not required to use a Kernel based on 5.3.0 or newer.

2. [IPoIB CNI](https://github.com/mellanox/ipoib-cni) provides an IPoIB network card for POD, without RDMA device. It is suitable for conventional applications that require TCP/IP communication, as it does not require an SRIOV network card, allowing more PODs to run on the host

## RDMA based on IB-SRIOV

The following steps demonstrate how to use [IB-SRIOV](https://github.com/k8snetworkplumbingwg/ib-sriov-cni) on a cluster with 2 nodes. It enables POD to own SR-IOV network card and the RDMA devices of isolated network namespace

1. Ensure that the host machine has an Infiniband card installed and the driver is properly installed.

    In our demo environment, the host machine is equipped with a Mellanox ConnectX-5 VPI NIC.

    Follow [the official NVIDIA guide](https://developer.nvidia.com/networking/ethernet-software) to install the latest OFED driver. 

    For Mellanox's VPI series network cards, you can refer to the official [Switching Infiniband Mode](https://support.mellanox.com/s/article/mlnx2-117-1997kn) to ensure that the network card is working in Infiniband mode.

    To confirm the presence of Inifiniband devices, use the following command:

        ~# lspci -nn | grep Infiniband
        86:00.0 Infiniband controller [0207]: Mellanox Technologies MT27800 Family [ConnectX-5] [15b3:1017]

        ~# rdma link
        link mlx5_0/1 subnet_prefix fe80:0000:0000:0000 lid 2 sm_lid 2 lmc 0 state ACTIVE physical_state LINK_UP

        ~# ibstat mlx5_0 | grep "Link layer"
        Link layer: InfiniBand

    Make sure that the RDMA subsystem of the host is in exclusive mode. If not, switch to shared mode.

        ~# rdma system set netns exclusive
        ~# echo "options ib_core netns_mode=0" >> /etc/modprobe.d/ib_core.conf
        ~# reboot

        ~# rdma system
        netns exclusive copy-on-fork on

    > if it is expected to work under shared mode, `rm /etc/modprobe.d/ib_core.conf && reboot`

    (Optional) In an SR-IOV scenario, applications can enable NVIDIA's GPUDirect RDMA feature. For instructions on installing the kernel module, please refer to [the official documentation](https://network.nvidia.com/products/GPUDirect-RDMA/).

2. Install Spiderpool, and notice the helm options:

        helm upgrade --install spiderpool spiderpool/spiderpool --namespace kube-system  --reuse-values --set sriov.install=true

    - If you are a user from China, you can specify the parameter `--set global.imageRegistryOverride=ghcr.m.daocloud.io` to pull image from China registry.
    
    Once the installation is complete, the following components will be installed:

        ~# kubectl get pod -n kube-system
        spiderpool-agent-9sllh                         1/1     Running     0          1m
        spiderpool-agent-h92bv                         1/1     Running     0          1m
        spiderpool-controller-7df784cdb7-bsfwv         1/1     Running     0          1m
        spiderpool-sriov-operator-65b59cd75d-89wtg     1/1     Running     0          1m
        spiderpool-init                                0/1     Completed   0          1m

3. configure SR-IOV operator.

    To enable the SR-IOV CNI on specific nodes, you need to apply the following command to label those nodes. This will allow the sriov-network-operator to install the components on the designated nodes.

        kubectl label node $NodeName node-role.kubernetes.io/worker=""

    Use the following commands to look up the device information of infiniband card

        ~# lspci -nn | grep Infiniband
        86:00.0 Infiniband controller [0207]: Mellanox Technologies MT27800 Family [ConnectX-5] [15b3:1017]

    The number of VFs determines how many SR-IOV network cards can be provided for PODs on a host. The network card from different manufacturers have different amount limit of VFs. For example, the Mellanox connectx5 used in this example can create up to 127 VFs.

    Apply the following configuration, and the VFs will be created on the host. Notice, this may cause the nodes to reboot, owing to taking effect the new configuration in the network card driver.

        cat <<EOF | kubectl apply -f -
        apiVersion: sriovnetwork.openshift.io/v1
        kind: SriovNetworkNodePolicy
        metadata:
          name: ib-sriov
          namespace: kube-system
        spec:
          nodeSelector:
            kubernetes.io/os: "linux"
          resourceName: mellanoxibsriov
          priority: 99
          numVfs: 12
          nicSelector:
              deviceID: "1017"
              rootDevices:
              - 0000:86:00.0
              vendor: "15b3"
          deviceType: netdevice
          isRdma: true
        EOF

    View the available resources on a node, including the reported RDMA device resources:

        ~# kubectl get no -o json | jq -r '[.items[] | {name:.metadata.name, allocable:.status.allocatable}]'
        [
          {
            "name": "10-20-1-10",
            "allocable": {
              "cpu": "40",
              "pods": "110",
              "spidernet.io/mellanoxibsriov": "12",
              ...
            }
          },
          ...
        ]  

4. Create the CNI configuration of IB-SRIOV, and the ippool resource

        cat <<EOF | kubectl apply -f -
        apiVersion: spiderpool.spidernet.io/v2beta1
        kind: SpiderIPPool
        metadata:
          name: v4-91
        spec:
          gateway: 172.91.0.1
          ips:
            - 172.91.0.100-172.91.0.120
          subnet: 172.91.0.0/16
        ---
        apiVersion: spiderpool.spidernet.io/v2beta1
        kind: SpiderMultusConfig
        metadata:
          name: ib-sriov
          namespace: kube-system
        spec:
          cniType: ib-sriov
          ibsriov:
            resourceName: spidernet.io/mellanoxibsriov
            ippools:
              ipv4: ["v4-91"]
        EOF

5. Following the configurations from the previous step, create a DaemonSet application that spans across nodes for testing

        ANNOTATION_MULTUS="v1.multus-cni.io/default-network: kube-system/ib-sriov"
        RESOURCE="spidernet.io/mellanoxibsriov"
        NAME=ib-sriov
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

6. Verify that RDMA data transmission is working correctly between the Pods across nodes.

    Open a terminal and access one Pod to launch a service:

        ~# rdma link
        link mlx5_4/1 subnet_prefix fe80:0000:0000:0000 lid 8 sm_lid 1 lmc 0 state ACTIVE physical_state LINK_UP
        
        # Start an RDMA service
        ~# ib_read_lat

    Open a terminal and access another Pod to launch a service:

        ~# rdma link
        link mlx5_8/1 subnet_prefix fe80:0000:0000:0000 lid 7 sm_lid 1 lmc 0 state ACTIVE physical_state LINK_UP
        
        # succeed to visit the service on the other POD
        ~# ib_read_lat 172.91.0.115

7.【optional】For environments with a UFM management platform, Subnet management can be implemented in combination with IB-Kubernetz and UFM, refer to the document [UFM and IB-Kubernetes](#using-ib-kubernetes-and-ufm-to-manager-infiniband-network). For environments that do not have a UFM management platform, you can ignore this step.

## IPoIB

The following steps demonstrate how to use [IPoIB](https://github.com/mellanox/ipoib-cni) on a cluster with 2 nodes, it enables Pod to own a regular TCP/IP network cards without RDMA device.

1. Ensure that the host machine has an Infiniband card installed and the driver is properly installed.

    In our demo environment, the host machine is equipped with a Mellanox ConnectX-5 VPI NIC.

    Follow [the official NVIDIA guide](https://developer.nvidia.com/networking/ethernet-software) to install the latest OFED driver.

    For Mellanox's VPI series network cards, you can refer to the official [Switching Infiniband Mode](https://support.mellanox.com/s/article/mlnx2-117-1997kn) to ensure that the network card is working in Infiniband mode.

    To confirm the presence of Inifiniband devices, use the following command:

        ~# lspci -nn | grep Infiniband
        86:00.0 Infiniband controller [0207]: Mellanox Technologies MT27800 Family [ConnectX-5] [15b3:1017]

        ~# rdma link
        link mlx5_0/1 subnet_prefix fe80:0000:0000:0000 lid 2 sm_lid 2 lmc 0 state ACTIVE physical_state LINK_UP

        ~# ibstat mlx5_0 | grep "Link layer"
        Link layer: InfiniBand

    Check the ipoib interface of the Inifiniband device

        ~# ip a show ibs5f0
        9: ibs5f0: <BROADCAST,MULTICAST,UP,LOWER_UP> mtu 2044 qdisc mq state UP group default qlen 256
        link/infiniband 00:00:10:49:fe:80:00:00:00:00:00:00:e8:eb:d3:03:00:93:ae:10 brd 00:ff:ff:ff:ff:12:40:1b:ff:ff:00:00:00:00:00:00:ff:ff:ff:ff
        altname ibp134s0f0
        inet 172.91.0.10/16 brd 172.91.255.255 scope global ibs5f0
        valid_lft forever preferred_lft forever
        inet6 fd00:91::172:91:0:10/64 scope global
        valid_lft forever preferred_lft forever
        inet6 fe80::eaeb:d303:93:ae10/64 scope link
        valid_lft forever preferred_lft forever

2. Install Spiderpool

    If you are a user from China, you can specify the parameter `--set global.imageRegistryOverride=ghcr.m.daocloud.io` to pull image from China registry.

    Once the installation is complete, the following components will be installed:

        ~# kubectl get pod -n kube-system
        spiderpool-agent-9sllh                         1/1     Running     0          1m
        spiderpool-agent-h92bv                         1/1     Running     0          1m
        spiderpool-controller-7df784cdb7-bsfwv         1/1     Running     0          1m
        spiderpool-init                                0/1     Completed   0          1m

3. Create the CNI configuration of ipoib, and the ippool. The `spec.ipoib.master` of SpiderMultusConfig should be set to the infiniband interface of the node.

        cat <<EOF | kubectl apply -f -
        apiVersion: spiderpool.spidernet.io/v2beta1
        kind: SpiderIPPool
        metadata:
          name: v4-91
        spec:
          gateway: 172.91.0.1
          ips:
            - 172.91.0.100-172.91.0.120
          subnet: 172.91.0.0/16
        ---
        apiVersion: spiderpool.spidernet.io/v2beta1
        kind: SpiderMultusConfig
        metadata:
          name: ipoib
          namespace: kube-system
        spec:
          cniType: ipoib
          ipoib:
            master: "ibs5f0"
            ippools:
              ipv4: ["v4-91"]
        EOF

4. Following the configurations from the previous step, create a DaemonSet application that spans across nodes for testing

        ANNOTATION_MULTUS="v1.multus-cni.io/default-network: kube-system/ipoib"
        NAME=ipoib
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
                command:
                - sh
                - -c
                - |
                  ls -l /dev/infiniband /sys/class/net
                  sleep 1000000
        EOF

5. Verify that the network communication is correct between the PODs across nodes.

        ~# kubectl get pod -o wide
        NAME                         READY   STATUS             RESTARTS          AGE    IP             NODE         NOMINATED NODE   READINESS GATES
        ipoib-psf4q                  1/1     Running            0                 34s    172.91.0.112   10-20-1-20   <none>           <none>
        ipoib-t9hm7                  1/1     Running            0                 34s    172.91.0.116   10-20-1-10   <none>           <none>

    Succeed to access each other

        ~# kubectl exec -it ipoib-psf4q bash
        kubectl exec [POD] [COMMAND] is DEPRECATED and will be removed in a future version. Use kubectl exec [POD] -- [COMMAND] instead.
        root@ipoib-psf4q:/# ping 172.91.0.116
        PING 172.91.0.116 (172.91.0.116) 56(84) bytes of data.
        64 bytes from 172.91.0.116: icmp_seq=1 ttl=64 time=1.10 ms
        64 bytes from 172.91.0.116: icmp_seq=2 ttl=64 time=0.235 ms

## Using ib-kubernetes and UFM to manager infiniband network

[ib-kubernetes](https://github.com/Mellanox/ib-kubernetes) is an open-source Infiniband plugin for Kubernetes from Mellanox, which is compatible with [ib-sriov-cni](https://github.com/k8snetworkplumbingwg/ib-sriov-cni). [Multus-cni](https://github.com/k8snetworkplumbingwg/multus-cni) Collaboration: Complete the setting of the Pkey and GUID, and advertise it to the UFM plug-in, and then UFM completes the subnet management under the Infiniband network.

[UFM](https://docs.nvidia.com/networking/display/ufmenterpriseqsgv6160/ufm+installation+steps) is Nvidia's software for managing Infiniland's network, which manages Infiniband's switches as well as hosts. It has the ability to:

- Discover and manage network devices such as hosts, switches, cables, and gateways in the Infiniband network, and perform basic management such as device discovery, topology display, software upgrade, and log collection.
- Implement network configurations including PKEY
- Network device status monitoring, network real-time traffic telemetry, network self-check, and alarm implementation
- OpenSM manages network devices

> Note: The UFM software runs as a server on a host with an Infiniband network card and requires a license to be purchased.

Here's how UFM and ib-kubernetes work together and some use cases.

### Workflow

![ib-kubernetes-ufm](../images/ib-kubernetes.png)

As can be seen from the above figure, ib-kubernetes will read its multus configuration or annotations when the pod is created, so as to read the pkey or guid (if there is a configuration, it will be automatically generated), and pass the pkey and guid information to the UFM plugin by calling the API of the UFM plugin. UFM will then complete subnet management of the pod based on both information.

### How to use

You need to install the UFM plugin in the environment in advance, let's install ib-kubernetes as follows:

1. Prepare the plugin configuration, the following configuration is used to help you log in to the UFM management platform.

        apiVersion: v1
        kind: Secret
        metadata:
          name: ib-kubernetes-ufm-secret
          namespace: kube-system
        stringData:
          UFM_USERNAME: "admin" # UFM username
          UFM_PASSWORD: "123456" # UFM password
          UFM_ADDRESS: "" # UFM admin address 
          UFM_HTTP_SCHEMA: ""    # http/https. Default: https
          UFM_PORT: ""           # UFM REST API port. Defaults: 443(https), 80(http)
        string:
          UFM_CERTIFICATE: ""    # UFM Certificate in base64 format. (if not provided client will not

2. To log in to UFM, you need to use a certificate, and you need to generate a certificate file on the UFM host first:

        $ openssl req -x509 -newkey rsa:4096 -keyout ufm.key -out ufm.crt -days 365 -subj '/CN=<UFM hostname>'

        Copy the certificate file to the UFM certificate location:
        $ cp ufm.key /etc/pki/tls/private/ufmlocalhost.key
        $ cp ufm.crt /etc/pki/tls/certs/ufmlocalhost.crt

        Restart UFM:
        $ docker restart ufm

        If deployed as bare metal:
        $ systemctl restart ufmd

3. Create a certificate key file for UFM:

        $ kubectl create secret generic ib-kubernetes-ufm-secret --namespace="kube-system" --from-literal=UFM_USER="admin" --from-literal=UFM_PASSWORD="12345" --from-literal=UFM_ ADDRESS="127.0.0.1" --from-file=UFM_CERTIFICATE=ufmlocalhost.crt --dry-run -o yaml > ib-kubernetes-ufm-secret.yaml
        $ kubectl create -f ./ib-kubernetes-ufm-secret.yaml 

4. Install ib-kubernetes:

        $ git clone https://github.com/Mellanox/ib-kubernetes.git && cd ib-kubernetes
        $ $ kubectl create -f deployment/ib-kubernetes-configmap.yaml
        $ kubectl create -f deployment/ib-kubernetes-ufm-secret.yaml
        $ kubectl create -f deployment/ib-kubernetes.yaml 

5. we only need to pay attention to specifying the pkey when creating SpiderMultusConfig:

        apiVersion: spiderpool.spidernet.io/v2beta1
        kind: SpiderMultusConfig
        metadata:
          name: ib-sriov
          namespace: kube-system
        spec:
          cniType: ib-sriov
          ibsriov:
            resourceName: spidernet.io/mellanoxibsriov
            pkey: 1000
            ippools:
              ipv4: ["v4-91"]

### Conclusion

ib-kubernetes can be integrated with UFM software to help UFM complete the subnet management of the Infiniband network under Kubernetes.
