# AI Cluster With SR-IOV (Infiniband)

**⚠️ 操作以下步骤之前，请确保您的环境已经达到 [环境要求](./index-zh_CN.md#环境要求)，并且按照 [主机准备](./index-zh_CN.md#主机准备) 完成 Infiniband RDMA 模式下的主机配置。**

## 配置 Sriov-operator

SR-IOV operator 的通用安装整体流程请参考 [配置 Sriov-operator](./get-started-sriov-roce-zh_CN.md#配置-sr-iov-operator) 。在 Infiniband 网络场景下，需要注意以下差异：

- 创建 SriovNetworkNodePolicy 时，需要设置 `linkType: ib`

以下用 eno3np2 为例：

```shell
$ LINK_TYPE=ib NIC_NAME=eno3np2 VF_NUM=12
$ cat <<EOF | kubectl apply -f -
apiVersion: sriovnetwork.openshift.io/v1
kind: SriovNetworkNodePolicy
metadata:
  name: ib-${NIC_NAME}
  namespace: spiderpool
spec:
  nodeSelector:
    kubernetes.io/os: "linux"
  resourceName: eno3np2
  priority: 99
  numVfs: ${VF_NUM}
  nicSelector:
    pfNames:
      - ${NIC_NAME}
  linkType: ${LINK_TYPE}
  deviceType: netdevice
  isRdma: true
EOF
```

## 配置 Spiderpool 资源

创建 CNI 配置和对应的 IPPool（IB-SRIOV CNI）

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
  name: gpu1-ib-sriov
  namespace: spiderpool
spec:
  cniType: ib-sriov
  ibsriov:
    resourceName: spidernet.io/eno3np2
    rdmaIsolation: true
    ippools:
      ipv4: ["gpu1-net11"]
EOF
```

> 如果集群中部署了 ib-kubernetes 组件并且对接了 UFM 管理平台，为 SpiderMultusConfig 配置 IP 池是可选的。

## （可选）Infiniband 网络下对接 UFM

对于使用了 Infiniband 网络的集群，如果网络中有 [UFM 管理平台](https://www.nvidia.com/en-us/networking/infiniband/ufm/)，可使用 [ib-kubernetes](https://github.com/Mellanox/ib-kubernetes) 插件，它以 daemonset 形式运行，监控所有使用 SRIOV 网卡的容器，把 VF 设备的 Pkey 和 GUID 上报给 UFM 。

1. 在 UFM 主机上创建通信所需要的证书：

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

2. 在 kubernetes 集群上，创建 ib-kubernetes 所需的通信证书。把 UFM 主机上生成的 ufm.crt 文件传输至 kubernetes 节点上，并使用如下命令创建证书

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

3. 在 kubernetes 集群上安装 ib-kubernetes

    ```shell
    git clone https://github.com/Mellanox/ib-kubernetes.git && cd ib-kubernetes
    $ kubectl create -f deployment/ib-kubernetes-configmap.yaml
    kubectl create -f deployment/ib-kubernetes.yaml 
    ```

4. 在 Infiniband 网络下，创建 Spiderpool 的 SpiderMultusConfig 时，可配置 pkey，使用该配置创建的 POD 将生效 pkey 配置，且被 ib-kubernetes 同步给 UFM

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

    > 注意：在 Infiniband Kubernetes 部署中，每个节点最多可以关联 128 个 PKey，这是内核的限制。

## 创建测试应用

1. 在指定节点上创建一组 DaemonSet 应用，测试指定节点上的 SR-IOV 设备的可用性
    如下例子，通过 annotations `v1.multus-cni.io/default-network` 指定使用 calico 的缺省网卡，用于进行控制面通信，annotations `k8s.v1.cni.cncf.io/networks` 接入 8 个 GPU 亲和网卡的 VF 网卡，用于 RDMA 通信，并配置 8 种 RDMA resources 资源

    > 注：支持自动为应用注入 RDMA 网络资源，参考 [基于 Webhook 自动为应用注入 RDMA 网络资源](./get-started-sriov-roce-zh_CN.md#基于-webhook-自动注入-rdma-网络资源)。

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

    在容器的网络命名空间创建过程中，Spiderpool 会对 sriov 接口上的网关进行连通性测试，如果如上应用的所有 POD 都启动成功，说明了每个节点上的 VF 设备的连通性成功，可进行正常的 RDMA 通信。

    <a id="checking-pod-network"></a>

2. 查看容器的网络命名空间状态

    可进入任一一个 POD 的网络命名空间中，确认具备 9 个网卡：

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

    确认具备 8 个 RDMA 设备

    ```shell
    root@rdma-tools-4v8t8:/# rdma link
        link mlx5_27/1 state ACTIVE physical_state LINK_UP netdev net2
        link mlx5_54/1 state ACTIVE physical_state LINK_UP netdev net1
        link mlx5_67/1 state ACTIVE physical_state LINK_UP netdev net4
        link mlx5_98/1 state ACTIVE physical_state LINK_UP netdev net3
        .....
    ```

3. 在跨节点的 Pod 之间，确认 RDMA 收发数据正常

    开启一个终端，进入一个 Pod 启动服务

    ```shell
    # see 8 RDMA devices assigned to the Pod
    $ rdma link

    # Start an RDMA service
    $ ib_read_lat
    ```

    开启一个终端，进入另一个 Pod 访问服务：

    ```shell
    # You should be able to see all RDMA network cards on the host
    $ rdma link
        
    # Successfully access the RDMA service of the other Pod
    $ ib_read_lat 172.91.0.115
    ```

    观察 RDMA 流量统计可通过进入到容器执行 `rdma statistic` 或参考 [RDMA监控](../../rdma-metrics-zh_CN.md).

## 其他

如果您需要为 SR-IOV InfiniBand VF 自定义 MTU，可以参考 [自定义 VF 的 MTU](./get-started-sriov-roce-zh_CN.md#自定义-vf-的-mtu)。
