# Weave Quick Start

[**English**](./get-started-weave.md) | **简体中文**

`Weave` 是一款开源的网络解决方案, 它通过创建一个虚拟网络、自动发现和连接不同的容器, 为容器提供网络连通和网络策略等能力。同时它可作为 Kubernetes 容器网络解决方案(CNI)的一种选择，`Weave` 默认使用内置的 `IPAM` 为 Pod 提供 IP 分配能力, 其 `IPAM` 能力对用户并不可见，缺乏 Pod IP 地址的管理分配能力。 本文将介绍 Spiderpool 搭配 `Weave`, 在保留 `Weave` 原有功能的基础上, 结合 `Spiderpool` 扩展 `Weave` 的 `IPAM` 能力。

## 先决条件

- [安装要求](./../system-requirements-zh_CN.md)
- 准备好一个 Kubernetes 集群, 没有安装任何的 CNI
- Helm、Kubectl、Jq(可选) 二进制工具

## 安装

1. 安装 Weave：

    ```shell
    kubectl apply -f  https://github.com/weaveworks/weave/releases/download/v2.8.1/weave-daemonset-k8s.yaml
    ```

    等待 Pod Running：

    ```shell
    [root@node1 ~]# kubectl get po -n kube-system  | grep weave
    weave-net-ck849                         2/2     Running     4     0   1m
    weave-net-vhmqx                         2/2     Running     4     0   1m
    ```

2. 安装 Spiderpool

    ```shell
    helm repo add spiderpool https://spidernet-io.github.io/spiderpool
    helm repo update spiderpool
    helm install spiderpool spiderpool/spiderpool --namespace kube-system --set multus.multusCNI.install=false
    ```

    > 如果您是中国用户，可以指定参数 `--set global.imageRegistryOverride=ghcr.m.daocloud.io` 来使用国内的镜像源。
    >
    > 通过 `multus.multusCNI.defaultCniCRName` 指定 multus 默认使用的 CNI 的 NetworkAttachmentDefinition 实例名。如果 `multus.multusCNI.defaultCniCRName` 选项不为空，则安装后会自动生成一个数据为空的 NetworkAttachmentDefinition 对应实例。如果 `multus.multusCNI.defaultCniCRName` 选项为空，会尝试通过 /etc/cni/net.d 目录下的第一个 CNI 配置来创建对应的 NetworkAttachmentDefinition 实例，否则会自动生成一个名为 `default` 的 NetworkAttachmentDefinition 实例，以完成 multus 的安装。

    等待 Pod Running， 创建 Pod 所使用的 IP 池:

    ```shell
    cat << EOF | kubectl apply -f -
    apiVersion: spiderpool.spidernet.io/v2beta1
    kind: SpiderIPPool
    metadata:
      name: weave-ippool-v4
      labels:  
        ipam.spidernet.io/subnet-cidr: 10-32-0-0-12
    spec:
      ips:
      - 10.32.0.100-10.32.50.200
      subnet: 10.32.0.0/12
    EOF
    ```

    > `Weave` 使用 `10.32.0.0/12` 作为集群默认子网。所以需要创建一个相同子网内 SpiderIPPool。

3. 验证安装

    ```shell
    [root@node1 ~]# kubectl get po -n kube-system | grep spiderpool
    spiderpool-agent-7hhkz                   1/1     Running     0              13m
    spiderpool-agent-kxf27                   1/1     Running     0              13m
    spiderpool-controller-76798dbb68-xnktr   1/1     Running     0              13m
    spiderpool-init                          0/1     Completed   0              13m
    [root@node1 ~]# kubectl get sp
    NAME               VERSION   SUBNET         ALLOCATED-IP-COUNT   TOTAL-IP-COUNT   DISABLE
    weave-ippool-v4    4         10.32.0.0/12   0                    12901            false
    ```

## 切换 `Weave` 的 `IPAM` 为 `Spiderpool`

修改每个节点上: `/etc/cni/net.d/10-weave.conflist` 的 `ipam`字段:

```shell
[root@node1 ~]# cat /etc/cni/net.d/10-weave.conflist
{
    "cniVersion": "0.3.0",
    "name": "weave",
    "plugins": [
        {
            "name": "weave",
            "type": "weave-net",
            "hairpinMode": true
        },
        {
            "type": "portmap",
            "capabilities": {"portMappings": true},
            "snat": true
        }
    ]
}
```

修改为：

```json
{
    "cniVersion": "0.3.0",
    "name": "weave",
    "plugins": [
        {
            "name": "weave",
            "type": "weave-net",
            "ipam": {
              "type": "spiderpool"
            },
            "hairpinMode": true
        },
        {
            "type": "portmap",
            "capabilities": {"portMappings": true},
            "snat": true
        }
    ]
}
```

或可通过 `jq`  工具一键修改。如没有 `jq` 可先使用以下命令安装:

```shell
# 以 centos7 为例
yum -y install jq
```

修改 CNI 配置文件:

```shell
cat <<< $(jq '.plugins[0].ipam.type = "spiderpool" ' /etc/cni/net.d/10-weave.conflist) > /etc/cni/net.d/10-weave.conflist
```

> 注意需要在每个节点上执行

## 创建应用

使用注解: `ipam.spidernet.io/ippool` 指定 Pod 从该 SpiderIPPool 中分配 IP:

```shell
[root@node1 ~]# cat << EOF | kubectl apply -f -
apiVersion: apps/v1
kind: Deployment
metadata:
  name: nginx
spec:
  replicas: 2
  selector:
    matchLabels:
      app: nginx
  template:
    metadata:
      annotations:
        ipam.spidernet.io/ippool: '{"ipv4":["weave-ippool-v4"]}'
      labels:
        app: nginx
    spec:
      containers:
      - image: nginx
        imagePullPolicy: IfNotPresent
        lifecycle: {}
        name: container-1
EOF
```

> _spec.template.metadata.annotations.ipam.spidernet.io/ippool_：指定 Pod 从 SpiderIPPool:  `weave-ippool-v4` 中分配 IP

Pod 成功创建, 并且从 Spiderpool 中分配 IP 地址:

```shell
[root@node1 ~]# kubectl get po  -o wide
NAME                     READY   STATUS    RESTARTS   AGE   IP             NODE    NOMINATED NODE   READINESS GATES
nginx-5745d9b5d7-2rvn7   1/1     Running   0          8s    10.32.22.190   node1   <none>           <none>
nginx-5745d9b5d7-5ssck   1/1     Running   0          8s    10.32.35.87    node2   <none>           <none>

[root@node1 ~]# kubectl get sp
NAME              VERSION   SUBNET          ALLOCATED-IP-COUNT   TOTAL-IP-COUNT   DISABLE
weave-ippool-v4   4         10.32.0.0/12    2                    2                false
```

测试连通性，以 Pod 跨节点通信为例:

```shell
[root@node1 ~]# kubectl exec  nginx-5745d9b5d7-2rvn7 -- ping 10.32.35.87 -c 2
PING 10.32.35.87 (10.32.35.87): 56 data bytes
64 bytes from 10.32.35.87: seq=0 ttl=64 time=4.561 ms
64 bytes from 10.32.35.87: seq=1 ttl=64 time=0.632 ms

--- 10.32.35.87 ping statistics ---
2 packets transmitted, 2 packets received, 0% packet loss
round-trip min/avg/max = 0.632/2.596/4.561 ms
```

测试结果表明，IP 分配正常、网络连接正常。通过 `Spiderpool`，确实扩展了 `Weave` 的 `IPAM` 能力。接下来，你可以参考 [Spiderpool 使用](https://spidernet-io.github.io/spiderpool/)，体验 `Spiderpool` 其他的功能。
