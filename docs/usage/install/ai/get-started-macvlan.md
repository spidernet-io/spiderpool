# AI Cluster With Macvlan(RoCE)

**English** | [**简体中文**](./get-started-macvlan-zh_CN.md)

**⚠️ Before proceeding, make sure your environment meets the [Requirements](./index.md#requirements), and finish the host preparation for shared RDMA mode in [Host preparation](./index.md#host-preparation).**

## Configure k8s-rdma-shared-dev-plugin

First, configure k8s-rdma-shared-dev-plugin to discover RDMA shared device resources on each host and report them to kubelet.

Edit the following ConfigMap to create 8 RDMA shared device resources, each affinitized to a GPU. For detailed configuration, refer to the [official documentation](https://github.com/Mellanox/k8s-rdma-shared-dev-plugin?tab=readme-ov-file#rdma-shared-device-plugin-configurations).

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

After the configuration is applied, check node allocatable resources to confirm each node reports 8 RDMA device resources.

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

## Create Spiderpool resources

### Create CNI config and IPPool

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
  rdmaResourceName: spidernet.io/shared_cx5_gpu1
  macvlan:
    master: ["enp11s0f0np0"]
    ippools:
      ipv4: ["gpu1-net11"]
EOF
```

By default, the Pod NIC MTU is the same as the Macvlan interface MTU. In some scenarios, you may want to customize Pod MTU:

```yaml
apiVersion: spiderpool.spidernet.io/v2beta1
kind: SpiderMultusConfig
metadata:
  name: gpu1-macvlan
  namespace: spiderpool
spec:
  cniType: macvlan
  macvlan:
    master: ["enp11s0f0np0"]
    rdmaResourceName: spidernet.io/shared_cx5_gpu1
    mtu: 1480
    ippools:
      ipv4: ["gpu1-net11"]
```

Note: MTU must not be greater than the MTU of the Macvlan master NIC, otherwise the Pod cannot be created.

## Create a test application

1. Create a DaemonSet on specified nodes

    In the following example, annotation `v1.multus-cni.io/default-network` specifies using the default Calico interface for control-plane communication. Annotation `k8s.v1.cni.cncf.io/networks` attaches 8 GPU-affinity interfaces for RDMA traffic, and requests 8 RDMA resources.

    > Tip: Webhook-based automatic injection is supported. See [Webhook-based Automatic RDMA Resource Injection](#webhook-based-automatic-rdma-resource-injection).

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

    # macvlan interfaces
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
      limits:
        spidernet.io/shared_cx5_gpu1: 1
        spidernet.io/shared_cx5_gpu2: 1
        spidernet.io/shared_cx5_gpu3: 1
        spidernet.io/shared_cx5_gpu4: 1
        spidernet.io/shared_cx5_gpu5: 1
        spidernet.io/shared_cx5_gpu6: 1
        spidernet.io/shared_cx5_gpu7: 1
        spidernet.io/shared_cx5_gpu8: 1
        #nvidia.com/gpu: 1
    EOF

    $ helm install rdma-tools spiderchart/rdma-tools -f ./values.yaml
    ```

    During creation of the Pod network namespace, Spiderpool will test gateway connectivity on the Macvlan interface. If all Pods start successfully, it indicates VF connectivity works and RDMA communication should be available.

    <a id="checking-pod-network"></a>

2. Check the Pod network namespace

    Enter any Pod and confirm there are 9 interfaces:

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

    Check routing. Spiderpool tunes policy routes to ensure return traffic symmetry:

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

    In the main routing table, ensure Calico/ClusterIP/local host traffic goes through the Calico interface:

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

    Confirm there are 8 RDMA devices:

    ```shell
    root@rdma-tools-4v8t8:/# rdma link
        link mlx5_27/1 state ACTIVE physical_state LINK_UP netdev net2
        link mlx5_54/1 state ACTIVE physical_state LINK_UP netdev net1
        link mlx5_67/1 state ACTIVE physical_state LINK_UP netdev net4
        link mlx5_98/1 state ACTIVE physical_state LINK_UP netdev net3
        .....
    ```

3. Validate RDMA connectivity between cross-node Pods

    In one Pod, start the service:

    ```shell
    # see 8 RDMA devices assigned to the Pod
    $ rdma link

    # Start an RDMA service
    $ ib_read_lat
    ```

    In another Pod, access the service:

    ```shell
    # You should be able to see all RDMA network cards on the host
    $ rdma link
        
    # Successfully access the RDMA service of the other Pod
    $ ib_read_lat 172.91.0.115
    ```

## Webhook-based Automatic RDMA Resource Injection

In the steps above, we demonstrated how to use SR-IOV technology to provide RDMA communication capabilities for containers in RoCE and Infiniband network environments. However, the process can become complex when configuring AI applications with multiple network cards. To simplify this process, Spiderpool supports classifying a set of network card configurations through annotations (`cni.spidernet.io/rdma-resource-inject` or `cni.spidernet.io/network-resource-inject`). Users only need to add the same annotation to the application, and Spiderpool will automatically inject all corresponding network cards and network resources with the same annotation into the application through a webhook. `cni.spidernet.io/rdma-resource-inject` annotation is only applicable to AI scenarios, automatically injecting RDMA network cards and RDMA resources. `cni.spidernet.io/network-resource-inject` annotation can be used not only for AI scenarios but also supports underlay scenarios. In the future, we hope to uniformly use `cni.spidernet.io/network-resource-inject` to support both of these scenarios.

> This feature only supports network card configurations with cniType of [ macvlan, ipvlan, sriov, ib-sriov, ipoib ].

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

2. When creating all SpiderMultusConfig instances for AI computing networks, add an annotation with the key "cni.spidernet.io/rdma-resource-inject" or "cni.spidernet.io/network-resource-inject" and a custom value.

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
        rdmaResourceName: spidernet.io/gpu1rdma
      ippools:
        ipv4: ["gpu1-net11"]
    ```

3. When creating an AI application, add the same annotation:

    ```yaml
    ...
    spec:
      template:
        metadata:
          annotations:
            cni.spidernet.io/rdma-resource-inject: rdma-network
    ```

   > Note: When using webhook-based injection, do not add other network configuration annotations (such as `k8s.v1.cni.cncf.io/networks` and `ipam.spidernet.io/ippools`) to the application, otherwise it may affect automatic injection.

4. After the Pod is created, you can observe that the Pod has been automatically injected with network annotations and RDMA resources:

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
