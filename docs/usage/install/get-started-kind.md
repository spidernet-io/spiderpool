# Quick Start

**English** | [**简体中文**](./get-started-kind-zh_CN.md)

Kind is a tool for running local Kubernetes clusters using Docker container "nodes". Spiderpool provides a script to install the Kind cluster, you can use it to deploy a cluster that meets your needs, and test and experience Spiderpool.

## Prerequisites

* Get the Spiderpool stable version code to the local host and enter the root directory of the Spiderpool project.

    ```bash
    ~# LATEST_RELEASE_VERISON=$(curl -s https://api.github.com/repos/spidernet-io/spiderpool/releases | grep '"tag_name":' | grep -v rc | grep -Eo "([0-9]+\.[0-9]+\.[0-9])" | sort -r | head -n 1)
    ~# curl -Lo /tmp/$LATEST_RELEASE_VERISON.tar.gz https://github.com/spidernet-io/spiderpool/archive/refs/tags/v$LATEST_RELEASE_VERISON.tar.gz
    ~# tar -xvf /tmp/$LATEST_RELEASE_VERISON.tar.gz -C /tmp/
    ~# cd /tmp/spiderpool-$LATEST_RELEASE_VERISON
    ```

* Execute `make dev-doctor` to check whether the development tools on the local host meet the conditions for deploying Kind cluster and Spiderpool.

    Building a Spiderpool environment requires Kubectl, Kind, Docker, Helm, and yq tools. If they are missing on your machine, run `test/scripts/install-tools.sh` to install them.

## Quick Start

If you are mainland user who is not available to access ghcr.io, Additional parameter `-e E2E_CHINA_IMAGE_REGISTRY=true` can be specified during installation to help you pull images faster.

=== "Spiderpool with Macvlan"

    In this scenario, POD cloud be assigned multiple macvlan interfaces, and communicate through Pod IP, clusterIP, nodePort, etc. Please refer to [underlay network case](../../concepts/arch.md#use-case-pod-with-multiple-underlay-cni-interfaces) for more details.

    The following command will create a single-CNI cluster with Macvlan，and it implements the ClusterIP by kube-proxy. 

    ```bash
    ~# make setup_singleCni_macvlan
    ```

=== "Dual CNIs with Spiderpool and Calico"

    In this scenario, you can experience the effect of Pod having dual CNI NICs. Please refer to [dual CNIs case](../../concepts/arch.md#use-case-pod-with-one-overlay-interface-and-multiple-underlay-interfaces) for more details.

    The following command will create a multi-CNI cluster with Calico and Spiderpool. In this environment, Calico serves as the default CNI and Pod could get a secondary underlay interface from Spiderpool. Calico works based on iptables datapath and implements service resolution based on kube-proxy.

    ```bash
    ~# make setup_dualCni_calico
    ```

=== "Dual CNIs with Spiderpool and Cilium"

    In this scenario, you can experience the effect of Pod having dual CNI NICs. Please refer to [dual CNIs case](../../concepts/arch.md#use-case-pod-with-one-overlay-interface-and-multiple-underlay-interfaces) for more details.

    The following command will create a multi-CNI cluster with Cilium and Spiderpool, In this environment, Cilium serves as the default CNI and Pod could get a secondary underlay interface from Spiderpool. Cilium's eBPF acceleration is enabled, kube-proxy is disabled, and service resolution is implemented based on eBPF.

    > Confirm whether the operating system Kernel version number is >= 4.9.17. If the kernel is too low, the installation will fail. Kernel 5.10+ is recommended.

    ```bash
    ~# make setup_dualCni_cilium
    ```

## Check the environment

Execute the following command in the root directory of the Spiderpool project to configure KUBECONFIG for the Kind cluster for kubectl.

```bash
~# export KUBECONFIG=$(pwd)/test/.cluster/spider/.kube/config
```

It should be possible to observe the following:

```bash
~# kubectl get nodes
NAME                   STATUS   ROLES           AGE     VERSION
spider-control-plane   Ready    control-plane   2m29s   v1.26.2
spider-worker          Ready    <none>          2m58s   v1.26.2

~# kubectl get po -n kube-system | grep spiderpool
NAME                                           READY   STATUS      RESTARTS   AGE                                
spiderpool-agent-4dr97                         1/1     Running     0          3m
spiderpool-agent-4fkm4                         1/1     Running     0          3m
spiderpool-controller-7864477fc7-c5dk4         1/1     Running     0          3m
spiderpool-controller-7864477fc7-wpgjn         1/1     Running     0          3m
spiderpool-init                                0/1     Completed   0          3m
```

The Quick Install Kind Cluster script provided by Spiderpool will automatically create an application for you to verify that your Kind cluster is working properly and the following is the running state of the application:

```bash
~# kubectl get po -l app=test-pod -o wide
NAME                       READY   STATUS    RESTARTS   AGE     IP             NODE            NOMINATED NODE   READINESS GATES
test-pod-856f9689d-876nm   1/1     Running   0          5m34s   172.18.40.63   spider-worker   <none>           <none>
```

## Deploy Applicatiion

Through the above checks, everything is normal in the Kind cluster. In this chapter, we will introduce how to use Spiderpool in different environments.

> Spiderpool introduces the [Spidermultusconfig](../spider-multus-config.md) CR to automate the management of Multus NetworkAttachmentDefinition CR and extend the capabilities of Multus CNI configurations.

===  "Spiderpool with Macvlan"

    Get the Spidermultusconfig CR and IPPool CR of the cluster

    ```bash
    ~# kubectl get spidermultusconfigs.spiderpool.spidernet.io -A
    NAMESPACE     NAME              AGE
    kube-system   macvlan-vlan0     1h
    kube-system   macvlan-vlan100   1h
    kube-system   macvlan-vlan200   1h

    ~# kubectl get spiderippool
    NAME                VERSION   SUBNET                    ALLOCATED-IP-COUNT   TOTAL-IP-COUNT   DEFAULT
    default-v4-ippool   4         172.18.0.0/16             5                    253              true      
    default-v6-ippool   6         fc00:f853:ccd:e793::/64   5                    253              true      
    ...
    ```

    Create an application. The following command will create a single NIC Deployment application:

    - `v1.multus-cni.io/default-network`：Specify Spidermultusconfig CR: `kube-system/macvlan-vlan0` through it, and use this configuration to create a default NIC (eth0) configured by Macvlan for the application.
    - `ipam.spidernet.io/ippool`：Used to specify Spiderpool's IP pool. Spiderpool will automatically select an IP in the pool to bind to the application's default NIC.
  
    ```shell
    cat <<EOF | kubectl create -f -
    apiVersion: apps/v1
    kind: Deployment
    metadata:
      name: test-app
    spec:
      replicas: 1
      selector:
        matchLabels:
          app: test-app
      template:
        metadata:
          labels:
            app: test-app
          annotations:
            ipam.spidernet.io/ippool: |-
              {      
                "ipv4": ["default-v4-ippool"],
                "ipv6": ["default-v6-ippool"]
              }
            v1.multus-cni.io/default-network: kube-system/macvlan-vlan0
        spec:
          containers:
          - name: test-app
            image: alpine
            imagePullPolicy: IfNotPresent
            command:
            - "/bin/sh"
            args:
            - "-c"
            - "sleep infinity"
    EOF
    ```

    Verify that the application was created successfully.

    ```shell
    ~# kubectl get po -owide
    NAME                        READY   STATUS    RESTARTS   AGE    IP              NODE            NOMINATED NODE   READINESS GATES
    test-app-7fdbb59666-4k5m7   1/1     Running       0          9s    172.18.40.223   spider-worker   <none>           <none>

    ~# kubectl exec -ti test-app-7fdbb59666-4k5m7 -- ip a
    ...
    3: eth0@if339: <BROADCAST,MULTICAST,UP,LOWER_UP,M-DOWN> mtu 1500 qdisc noqueue state UP 
        link/ether 0a:96:54:6f:76:b4 brd ff:ff:ff:ff:ff:ff
        inet 172.18.40.223/16 brd 172.18.255.255 scope global eth0
           valid_lft forever preferred_lft forever
    4: veth0@if11: <BROADCAST,MULTICAST,UP,LOWER_UP,M-DOWN> mtu 1500 qdisc noqueue state UP 
        link/ether 4a:8b:09:d9:4c:0a brd ff:ff:ff:ff:ff:ff
    ```

===  "Dual CNIs with Spiderpool and Calico"

    Get the Spidermultusconfig CR and IPPool CR of the cluster

    ```bash
    ~# kubectl get spidermultusconfigs.spiderpool.spidernet.io -A
    NAMESPACE     NAME              AGE
    kube-system   calico            3m11s
    kube-system   macvlan-vlan0     2m20s
    kube-system   macvlan-vlan100   2m19s
    kube-system   macvlan-vlan200   2m19s

    ~# kubectl get spiderippool
    NAME                VERSION   SUBNET                    ALLOCATED-IP-COUNT   TOTAL-IP-COUNT   DEFAULT
    default-v4-ippool   4         172.18.0.0/16             1                    253              true
    default-v6-ippool   6         fc00:f853:ccd:e793::/64   1                    253              true
    ...
    ```

    Create an application. The following command will create a Deployment application with two NICs.

    - The default NIC(eth0) is configured by the cluster default CNI Calico.

    - `k8s.v1.cni.cncf.io/networks`：Use this annotation to create an additional NIC (net1) configured by Macvlan for the application.

    - `ipam.spidernet.io/ippools`：Used to specify Spiderpool's IPPool. Spiderpool will automatically select an IP in the pool to bind to the application's net1 NIC

    ```shell
    cat <<EOF | kubectl create -f -
    apiVersion: apps/v1
    kind: Deployment
    metadata:
      name: test-app
    spec:
      replicas: 1
      selector:
        matchLabels:
          app: test-app
      template:
        metadata:
          labels:
            app: test-app
          annotations:
            ipam.spidernet.io/ippools: |-
              [{
                "interface": "net1",
                "ipv4": ["default-v4-ippool"],
                "ipv6": ["default-v6-ippool"]
              }]
            k8s.v1.cni.cncf.io/networks: kube-system/macvlan-vlan0
        spec:
          containers:
          - name: test-app
            image: alpine
            imagePullPolicy: IfNotPresent
            command:
            - "/bin/sh"
            args:
            - "-c"
            - "sleep infinity"
    EOF
    ```

    Verify that the application was created successfully.

    ```shell
    ~# kubectl get po -owide
    NAME                      READY   STATUS    RESTARTS   AGE   IP               NODE            NOMINATED NODE   READINESS GATES
    test-app-86dd478b-bv6rm   1/1     Running   0          12s   10.243.104.211   spider-worker   <none>           <none>

    ~# kubectl exec -ti test-app-7fdbb59666-4k5m7 -- ip a
    ...
    4: eth0@if148: <BROADCAST,MULTICAST,UP,LOWER_UP,M-DOWN> mtu 1480 qdisc noqueue state UP qlen 1000
        link/ether 1a:1e:e1:f3:f9:4b brd ff:ff:ff:ff:ff:ff
        inet 10.243.104.211/32 scope global eth0
    5: net1@if347: <BROADCAST,MULTICAST,UP,LOWER_UP,M-DOWN> mtu 1500 qdisc noqueue state UP 
        link/ether 56:b4:3d:a6:d2:d1 brd ff:ff:ff:ff:ff:ff
        inet 172.18.40.154/16 brd 172.18.255.255 scope global net1
    ```

===  "Dual CNIs with Spiderpool and Cilium"

    Get the Spidermultusconfig CR and IPPool CR of the cluster

    ```bash
    ~# kubectl get spidermultusconfigs.spiderpool.spidernet.io -A
    NAMESPACE     NAME              AGE
    kube-system   cilium            5m32s
    kube-system   macvlan-vlan0     5m12s
    kube-system   macvlan-vlan100   5m17s
    kube-system   macvlan-vlan200   5m18s

    ~# kubectl get spiderippool
    NAME                VERSION   SUBNET                    ALLOCATED-IP-COUNT   TOTAL-IP-COUNT   DEFAULT
    default-v4-ippool   4         172.18.0.0/16             1                    253              true
    default-v6-ippool   6         fc00:f853:ccd:e793::/64   1                    253              true
    ...
    ```

    Create an application. The following command will create a Deployment application with two NICs.

    - The default NIC(eth0) is configured by the cluster default CNI Cilium.

    - `k8s.v1.cni.cncf.io/networks`：Use this annotation to create an additional NIC (net1) configured by Macvlan for the application.

    - `ipam.spidernet.io/ippools`：Used to specify Spiderpool's IPPool. Spiderpool will automatically select an IP in the pool to bind to the application's net1 NIC

    ```shell
    cat <<EOF | kubectl create -f -
    apiVersion: apps/v1
    kind: Deployment
    metadata:
      name: test-app
    spec:
      replicas: 1
      selector:
        matchLabels:
          app: test-app
      template:
        metadata:
          labels:
            app: test-app
          annotations:
            ipam.spidernet.io/ippools: |-
              [{
                "interface": "net1",
                "ipv4": ["default-v4-ippool"],
                "ipv6": ["default-v6-ippool"]
              }]
            k8s.v1.cni.cncf.io/networks: kube-system/macvlan-vlan0
        spec:
          containers:
          - name: test-app
            image: alpine
            imagePullPolicy: IfNotPresent
            command:
            - "/bin/sh"
            args:
            - "-c"
            - "sleep infinity"
    EOF
    ```

    Verify that the application was created successfully.

    ```shell
    ~# kubectl get po -owide
    NAME                      READY   STATUS    RESTARTS   AGE   IP               NODE            NOMINATED NODE   READINESS GATES
    test-app-86dd478b-ml8d9   1/1     Running   0          58s   10.244.102.212   spider-worker   <none>           <none>

    ~# kubectl exec -ti test-app-7fdbb59666-4k5m7 -- ip a
    ...
    4: eth0@if148: <BROADCAST,MULTICAST,UP,LOWER_UP,M-DOWN> mtu 1480 qdisc noqueue state UP qlen 1000
        link/ether 26:f1:88:f9:7d:d7 brd ff:ff:ff:ff:ff:ff
        inet 10.244.102.212/32 scope global eth0
    5: net1@if347: <BROADCAST,MULTICAST,UP,LOWER_UP,M-DOWN> mtu 1500 qdisc noqueue state UP 
        link/ether ca:71:99:ec:ec:28 brd ff:ff:ff:ff:ff:ff
        inet 172.18.40.228/16 brd 172.18.255.255 scope global net1
    ```


Now you can test and experience Spiderpool's [more features](../readme.md) based on Kind.

## Uninstall

* Uninstall a Kind cluster

    Execute `make clean` to uninstall the Kind cluster.

* Delete test's images

    ```bash
    ~# docker rmi -f $(docker images | grep spiderpool | awk '{print $3}')
    ~# docker rmi -f $(docker images | grep multus | awk '{print $3}')
    ```
