# IPVlan 带宽管理

**简体中文** ｜ [**English**](./ipvlan_bandwidth.md)

本文将展示如何借助 [cilium-chaining](https://github.com/spidernet-io/cilium-chaining) 这个项目，实现 IPVlan CNI 的带宽管理能力。

## 背景

Kubernetes 官方支持向 Pod 中注入 Annotations 的方式设置 Pod 的入口出口带宽，参考 [带宽限制](https://kubernetes.io/zh-cn/docs/concepts/extend-kubernetes/compute-storage-net/network-plugins/#support-traffic-shaping)

当我们使用 IPVlan 作为 CNI 时，它本身不具备管理 Pod 的入口/出口流量带宽管理能力。开源项目 Cilium 支持以 cni-chaining 的方式与 IPVlan 一起联动工作，基于 eBPF 技术可以帮助 IPVlan 实现加速访问 Service, 带宽能力管理等功能。但目前 Cilium 已经移除支持 IPVlan 的 Dataplane。[cilium-chaining](https://github.com/spidernet-io/cilium-chaining) 项目基于 cilium v1.12.7 构建，并支持 IPVlan 的 Dataplane, 我们可以借助它帮助 IPVlan 实现 Pod 的网络带宽管理能力。

## 预置条件

* Helm 和 Kubectl 二进制工具
* 要求节点内核至大于 4.19

## 安装 Spiderpool

安装 Spiderpool 可参考文档: [安装 Spiderpool](./install/underlay/get-started-macvlan-zh_CN.md)

## 安装 Cilium-chaining 项目

使用以下命令安装 cilium-chainging 项目:

```shell
kubectl apply -f https://raw.githubusercontent.com/spidernet-io/cilium-chaining/main/manifests/cilium-chaining.yaml
```

安装状态检查:

```shell
~# kubectl get po -n kube-system | grep cilium-chain
cilium-chaining-gl76b                        1/1     Running     0              137m
cilium-chaining-nzvrg                        1/1     Running     0              137m
```

## 创建 CNI 配置文件和 IP 池

参考以下命令创建 CNI 配置:

```shell
# 创建 Multus Network-attachement-definition CR
IPVLAN_MASTER_INTERFACE=ens192
IPPOOL_NAME=ens192-v4
cat << EOF | kubectl apply -f -
apiVersion: k8s.cni.cncf.io/v1
kind: NetworkAttachmentDefinition
metadata:
  name: ipvlan
  namespace: kube-system
spec:
  config: ' { "cniVersion": "0.4.0", "name": "terway-chainer", "plugins": [ { "type":
    "ipvlan", "mode": "l2", "master": "${IPVLAN_MASTER_INTERFACE}", "ipam": { "type":"spiderpool", "default_ipv4_ippool":
    ["${IPPOOL_NAME}"] } }, { "type": "cilium-cni" }, { "type": "coordinator" }
    ] }'
EOF
```

> 配置 cilium-cni 以 cni-chain 的模式搭配 ipvlan cni

参考以下命令创建 CNI 配置:

```shell
# 创建 IP 池
cat << EOF | kubectl apply -f -
apiVersion: spiderpool.spidernet.io/v2beta1
kind: SpiderIPPool
metadata:
  name: ${IPPOOL_NAME}
spec:
  default: false
  disable: false
  gateway: 172.51.0.1
  ipVersion: 4
  ips:
  - 172.51.0.100-172.51.0.108
  subnet: 172.51.0.230/16
```

> 注意 ens192 需要存在于主机上，并且配置 IP 池的网段需要和 ens192 所在物理网络保持一致

## 创建应用测试带宽限制

使用上面创建的 CNI 配置以及 IP 池创建测试应用，验证 Pod 的带宽是否受到限制:

```shell
cat << EOF | kubectl apply -f -
apiVersion: apps/v1
kind: Deployment
metadata:
  name: test
spec:
  replicas: 2
  selector:
    matchLabels:
      app: test
  template:
    metadata:
      annotations:
        v1.multus-cni.io/default-network: kube-system/ipvlan
        kubernetes.io/ingress-bandwidth: 100M
        kubernetes.io/egress-bandwidth: 100M
      labels:
        app: test
    spec:
      containers:
      - env:
        - name: NODE_NAME
          valueFrom:
            fieldRef:
              apiVersion: v1
              fieldPath: spec.nodeName
        - name: POD_NAME
          valueFrom:
            fieldRef:
              apiVersion: v1
              fieldPath: metadata.name
        - name: POD_NAMESPACE
          valueFrom:
            fieldRef:
              apiVersion: v1
              fieldPath: metadata.namespace
        image: nginx
        imagePullPolicy: IfNotPresent
        name: nginx
        ports:
        - containerPort: 80
          name: http
          protocol: TCP
        resources: {}
```

几个 annotations 介绍:

* `v1.multus-cni.io/default-network: ipvlan`: 指定 Pod 的缺省 CNI 为 之前创建的 ipvlan
* `kubernetes.io/ingress-bandwidth: 100m`: 设置 Pod 的入口带宽为 100M
* `kubernetes.io/egress-bandwidth: 100m`: 设置 Pod 的出口带宽为 100M

```shell
~# kubectl get po -o wide
NAME                     READY   STATUS        RESTARTS   AGE    IP               NODE          NOMINATED NODE   READINESS GATES
test-58d785fb4c-b9cld    1/1     Running       0          175m   172.51.0.102     10-20-1-230   <none>           <none>
test-58d785fb4c-kwh4h    1/1     Running       0          175m   172.51.0.100     10-20-1-220   <none>           <none>
```

当 Pod 创建完成，分别进入到 Pod 的网络命名空间，使用 `iperf3` 工具测试其网络带宽:

```shell
~# crictl ps | grep test
b2b60a6e14e21       8f2213208a7f5       10 seconds ago      Running             nginx                     0                   f46848c1a713a       test-58d785fb4c-kwh4h
~# crictl inspect b2b60a6e14e21 | grep pid
    "pid": 284529,
            "pid": 1
            "type": "pid"
~# nsenter -t 284529 -n
~# iperf3 -s
```

在另外一个节点的 Pod 作为 Client 访问:

```shell
root@10-20-1-230:~# crictl ps | grep test
0e3e211f83723       8f2213208a7f5       39 seconds ago      Running             nginx                      0                   3f668220e8349       test-58d785fb4c-b9cld
root@10-20-1-230:~# crictl inspect 0e3e211f83723 | grep pid
    "pid": 976027,
            "pid": 1
            "type": "pid"
root@10-20-1-230:~# nsenter -t 976027 -n
root@10-20-1-230:~#
root@10-20-1-230:~# iperf3 -c 172.51.0.100
Connecting to host 172.51.0.100, port 5201
[  5] local 172.51.0.102 port 50504 connected to 172.51.0.100 port 5201
[ ID] Interval           Transfer     Bitrate         Retr  Cwnd
[  5]   0.00-1.00   sec  37.1 MBytes   311 Mbits/sec    0   35.4 KBytes
[  5]   1.00-2.00   sec  11.2 MBytes  94.4 Mbits/sec    0    103 KBytes
[  5]   2.00-3.00   sec  11.2 MBytes  94.4 Mbits/sec    0   7.07 KBytes
[  5]   3.00-4.00   sec  11.2 MBytes  94.4 Mbits/sec    0   29.7 KBytes
[  5]   4.00-5.00   sec  11.2 MBytes  94.4 Mbits/sec    0   33.9 KBytes
[  5]   5.00-6.00   sec  12.5 MBytes   105 Mbits/sec    0   29.7 KBytes
[  5]   6.00-7.00   sec  10.0 MBytes  83.9 Mbits/sec    0   62.2 KBytes
[  5]   7.00-8.00   sec  12.5 MBytes   105 Mbits/sec    0   22.6 KBytes
[  5]   8.00-9.00   sec  10.0 MBytes  83.9 Mbits/sec    0   69.3 KBytes
[  5]   9.00-10.00  sec  10.0 MBytes  83.9 Mbits/sec    0   52.3 KBytes
- - - - - - - - - - - - - - - - - - - - - - - - -
[ ID] Interval           Transfer     Bitrate         Retr
[  5]   0.00-10.00  sec   137 MBytes   115 Mbits/sec    0             sender
[  5]   0.00-10.00  sec   134 MBytes   113 Mbits/sec                  receiver

iperf Done.
```

可以看到结果为 115 Mbits/sec ，说明 Pod 的带宽已经被限制为我们通过 annotations 中定义的大小。
