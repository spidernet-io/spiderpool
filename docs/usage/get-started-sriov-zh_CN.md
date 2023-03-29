# Quick Start

[**English**](./get-started-sriov.md) | **简体中文**

Spiderpool 可用作 underlay 网络场景下提供固定 IP 的一种解决方案，本文将以 [Multus](https://github.com/k8snetworkplumbingwg/multus-cni)、[Sriov](https://github.com/k8snetworkplumbingwg/sriov-cni) 、[Veth](https://github.com/spidernet-io/plugins)、[Spiderpool](https://github.com/spidernet-io/spiderpool) 为例，搭建一套完整的 underlay 网络解决方案，该方案能够满足以下各种功能需求：

* 通过简易运维，应用可分配到固定的 Underlay IP 地址

* Pod 的网卡具有 Sriov 的网络加速功能

* Pod 能够通过 Pod IP、clusterIP、nodePort 等方式通信

## 先决条件

1. 一个 Kubernetes 集群
2. [Helm 工具](https://helm.sh/docs/intro/install/)
3. [支持 SR-IOV 功能的网卡](https://github.com/k8snetworkplumbingwg/sriov-network-device-plugin#supported-sr-iov-nics)
   
   * 查询网卡 bus-info：

   ```shell
   ~# ethtool -i enp4s0f0np0 |grep bus-info
   bus-info: 0000:04:00.0
   ```
   
   * 通过 bus-info 查询网卡是否支持 SR-IOV 功能，出现 `Single Root I/O Virtualization (SR-IOV)` 字段表示网卡支持 SR-IOV 功能：
   
   ```shell
   ~# lspci -s 0000:04:00.0 -v |grep SR-IOV
   Capabilities: [180] Single Root I/O Virtualization (SR-IOV)      
   ```
   

## 安装 Veth

[`Veth`](https://github.com/spidernet-io/plugins) 是一个 CNI 插件，它能够帮助一些 CNI （例如 Macvlan、SR-IOV 等）解决如下问题：

* 在 Sriov CNI 场景下，帮助 Pod 实现 clusterIP 通信

* 在 Pod 多网卡场景下，Veth 能自动够协调多网卡间的策略路由，解决多网卡通信问题

请在所有的节点上，下载安装 Veth 二进制：

```shell
~# wget https://github.com/spidernet-io/plugins/releases/download/v0.1.4/spider-plugins-linux-amd64-v0.1.4.tar

~# tar xvfzp ./spider-plugins-linux-amd64-v0.1.4.tar -C /opt/cni/bin

~# chmod +x /opt/cni/bin/veth
```

## 创建与网卡配置匹配的 Sriov Configmap

* 查询网卡 vendor、deviceID 和 driver 信息：

    ```shell
    ~# ethtool -i enp4s0f0np0 |grep -e driver -e bus-info
    driver: mlx5_core
    bus-info: 0000:04:00.0
    ~#
    ~# lspci -s 0000:04:00.0 -n
    04:00.0 0200: 15b3:1018
    ```

    > 本示例中，vendor 为 15b3，deviceID 为 1018，driver 为 mlx5_core

* 创建 Configmap

    ```shell
    vendor="15b3"
    deviceID="1018"
    driver="mlx5_core"
    cat <<EOF | kubectl apply -f -
    apiVersion: v1
    kind: ConfigMap
    metadata:
        name: sriovdp-config
        namespace: kube-system
    data:
        config.json: |
        {
            "resourceList": [{
                    "resourceName": "mlnx_sriov",
                    "selectors": {
                        "vendors": [ "$vendor" ],
                        "devices": [ "$deviceID" ],
                        "drivers": [ "$driver" ]
                        }
                }
            ]
        }
    EOF
    ```

    > resourceName 为 sriov 资源名称，在 configmap 声明后，在 sriov-plugin 生效后，会在 node 上产生一个名为 `intel.com/mlnx_sriov` 的 sriov 资源供 Pod 使用，前缀 `intel.com` 可通过 `resourcePrefix` 字段定义
    >具体配置规则参考 [Sriov Configmap](https://github.com/k8snetworkplumbingwg/sriov-network-device-plugin#configurations)


## 创建 Sriov VF

1. 查询当前 VF 数量

    ```shell
    ~# cat /sys/class/net/enp4s0f0np0/device/sriov_numvfs
    0
    ```
   
2. 创建 8个 VF 

    ```shell
    ~# echo 8 > /sys/class/net/enp4s0f0np0/device/sriov_numvfs
    ```

    >具体配置参考 sriov 官方文档 [Setting up Virtual Functions](https://github.com/k8snetworkplumbingwg/sriov-network-device-plugin/blob/master/docs/vf-setup.md)

## 安装 Sriov Device Plugin 

```shell
~# kubectl apply -f https://raw.githubusercontent.com/k8snetworkplumbingwg/sriov-network-device-plugin/v3.5.1/deployments/k8s-v1.16/sriovdp-daemonset.yaml
```

安装完成后，等待插件生效。

* 查看 Node 发现在 configmap 中定义的名为 `intel.com/mlnx_sriov` 的 sriov 资源已经生效，其中 8 为 VF 的数量：

    ```shell
    ~# kubectl get  node  master-11 -ojson |jq '.status.allocatable'
    {
      "cpu": "24",
      "ephemeral-storage": "94580335255",
      "hugepages-1Gi": "0",
      "hugepages-2Mi": "0",
      "intel.com/mlnx_sriov": "8",
      "memory": "16247944Ki",
      "pods": "110"
    }
    ```

## 安装 Sriov CNI

1. 通过 manifest 安装 Sriov CNI

    ```shell
    ~# kubectl apply -f https://raw.githubusercontent.com/k8snetworkplumbingwg/sriov-cni/v2.7.0/images/k8s-v1.16/sriov-cni-daemonset.yaml
    ```

## 安装 Multus

1. 通过 manifest 安装 Multus

    ```shell
    ~# kubectl apply -f https://raw.githubusercontent.com/k8snetworkplumbingwg/multus-cni/v3.9/deployments/multus-daemonset.yml
    ```

2. 为 Sriov 创建 Multus 的 NetworkAttachmentDefinition 配置

   因为使用 Veth 插件来实现 clusterIP 通信，需确认集群的 service CIDR，例如可基于命令 `kubectl -n kube-system get configmap kubeadm-config -oyaml | grep service` 查询

    ```shell
    SERVICE_CIDR="10.43.0.0/16"
    cat <<EOF | kubectl apply -f -
    apiVersion: k8s.cni.cncf.io/v1
    kind: NetworkAttachmentDefinition
    metadata:
      annotations:
        k8s.v1.cni.cncf.io/resourceName: intel.com/mlnx_sriov
      name: sriov-test
      namespace: kube-system
    spec:
      config: |-
        {
            "cniVersion": "0.3.1",
            "name": "sriov-test",
            "plugins": [
                {
                    "type": "sriov",
                    "ipam": {
                        "type": "spiderpool"
                    }
                },{
                      "type": "veth",
                      "service_cidr": ["${SERVICE_CIDR}"]
                  }
            ]
        }
   EOF
    ```

   > `k8s.v1.cni.cncf.io/resourceName: intel.com/mlnx_sriov` 该annotations 表示要使用的 sriov 资源名称


## 安装 Spiderpool

1. 安装 Spiderpool CRD

    ```shell
    helm repo add spiderpool https://spidernet-io.github.io/spiderpool
    helm repo update spiderpool
    helm install spiderpool spiderpool/spiderpool --namespace kube-system \
        --set feature.enableIPv4=true --set feature.enableIPv6=false 
    ```
   
2. 创建 SpiderSubnet 实例。
   
   Pod 会从该子网中获取 IP，进行 Underlay 的网络通讯，所以该子网需要与接入的 Underlay 子网对应。
   以下是创建相关的 SpiderSubnet 示例
    
   ```shell
   cat <<EOF | kubectl apply -f -
   apiVersion: spiderpool.spidernet.io/v2beta1
   kind: SpiderSubnet
   metadata:
     name: subnet-test
   spec:
     ipVersion: 4
     ips:
     - "10.20.168.190-10.20.168.199"
     subnet: 10.20.0.0/16
     gateway: 10.20.0.1
     vlan: 0
   EOF
   ```

## 创建应用

1. 使用如下命令创建测试 Pod 和 Service：

   ```shell
   cat <<EOF | kubectl create -f -
   apiVersion: apps/v1
   kind: Deployment
   metadata:
     name: sriov-deploy
   spec:
     replicas: 2
     selector:
       matchLabels:
         app: sriov-deploy
     template:
       metadata:
         annotations:
           ipam.spidernet.io/subnet: |-
             {
               "ipv4": ["subnet-test"]
             }
           v1.multus-cni.io/default-network: kube-system/sriov-test
         labels:
           app: sriov-deploy
       spec:
         containers:
         - name: sriov-deploy
           image: nginx
           imagePullPolicy: IfNotPresent
           ports:
           - name: http
             containerPort: 80
             protocol: TCP
           resources:
             requests:
               intel.com/mlnx_sriov: '1' 
             limits:
               intel.com/mlnx_sriov: '1'  
   ---
   apiVersion: v1
   kind: Service
   metadata:
     name: sriov-deploy-svc
     labels:
       app: sriov-deploy
   spec:
     type: ClusterIP
     ports:
       - port: 80
         protocol: TCP
         targetPort: 80
     selector:
       app: sriov-deploy 
   EOF
   ```
    
   必要参数说明：
   > `intel.com/mlnx_sriov`: 该参数表示使用 Sriov 资源。
   > 
   >`v1.multus-cni.io/default-network`：该 annotation 指定了使用的 Multus 的 CNI 配置。
   >
   > 更多 Multus 注解使用请参考 [Multus 注解](https://github.com/k8snetworkplumbingwg/multus-cni/blob/master/docs/quickstart.md)

2. 查看 Pod 运行状态

   ```shell
   ~# kubectl get pod -l app=sriov-deploy -owide
   NAME                           READY   STATUS    RESTARTS   AGE     IP              NODE        NOMINATED NODE   READINESS GATES
   sriov-deploy-9b4b9f6d9-mmpsm   1/1     Running   0          6m54s   10.20.168.191   worker-12   <none>           <none>
   sriov-deploy-9b4b9f6d9-xfsvj   1/1     Running   0          6m54s   10.20.168.190   master-11   <none>           <none>
   ```

3. Spiderpool 自动为应用创建了 IP 固定池，应用的 IP 将会自动固定在该 IP 范围内

   ```shell
   ~# kubectl get spiderippool
   NAME                                     VERSION   SUBNET         ALLOCATED-IP-COUNT   TOTAL-IP-COUNT   DEFAULT   DISABLE
   auto-sriov-deploy-v4-eth0-f5488b112fd9   4         10.20.0.0/16   2                    2                false     false
   
   ~#  kubectl get spiderendpoints
   NAME                           INTERFACE   IPV4POOL                                 IPV4               IPV6POOL   IPV6   NODE
   sriov-deploy-9b4b9f6d9-mmpsm   eth0        auto-sriov-deploy-v4-eth0-f5488b112fd9   10.20.168.191/16                     worker-12
   sriov-deploy-9b4b9f6d9-xfsvj   eth0        auto-sriov-deploy-v4-eth0-f5488b112fd9   10.20.168.190/16                     master-11
   ```
   
4. 测试 Pod 与 Pod 的通讯

   ```shell
   ~# kubectl exec -it sriov-deploy-9b4b9f6d9-mmpsm -- ping 10.20.168.190 -c 3
   PING 10.20.168.190 (10.20.168.190) 56(84) bytes of data.
   64 bytes from 10.20.168.190: icmp_seq=1 ttl=64 time=0.162 ms
   64 bytes from 10.20.168.190: icmp_seq=2 ttl=64 time=0.138 ms
   64 bytes from 10.20.168.190: icmp_seq=3 ttl=64 time=0.191 ms
   
   --- 10.20.168.190 ping statistics ---
   3 packets transmitted, 3 received, 0% packet loss, time 2051ms
   rtt min/avg/max/mdev = 0.138/0.163/0.191/0.021 ms
   ```

5. 测试 Pod 与 Service 通讯

* 查看 Service 的 IP：

   ```shell
   ~# kubectl get svc
   NAME               TYPE        CLUSTER-IP     EXTERNAL-IP   PORT(S)              AGE
   kubernetes         ClusterIP   10.43.0.1      <none>        443/TCP              23d
   sriov-deploy-svc   ClusterIP   10.43.54.100   <none>        80/TCP               20m
   ```

* Pod 内访问自身的 Service ：

   ```shell
   ~# kubectl exec -it sriov-deploy-9b4b9f6d9-mmpsm -- curl 10.43.54.100 -I
   HTTP/1.1 200 OK
   Server: nginx/1.23.3
   Date: Mon, 27 Mar 2023 08:22:39 GMT
   Content-Type: text/html
   Content-Length: 615
   Last-Modified: Tue, 13 Dec 2022 15:53:53 GMT
   Connection: keep-alive
   ETag: "6398a011-267"
   Accept-Ranges: bytes
   ```
