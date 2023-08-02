# Weave Quick Start

[**English**](./get-started-weave.md) | **简体中文**

`Weave` 是一款开源的网络解决方案, 它通过创建一个虚拟网络、自动发现和连接不同的容器, 为容器提供网络连通和网络策略等能力。同时它可作为 Kubernetes 容器网络解决方案(CNI)的一种选择，`Weave` 默认使用内置的 `IPAM` 为 Pod 提供 IP 分配能力, 其 `IPAM` 能力对用户并不可见，缺乏 Pod IP 地址的管理分配能力。 本文将介绍 Spiderpool 搭配 `Weave`, 在保留 `Weave` 原有功能的基础上, 结合 `Spiderpool` 扩展 `Weave` 的 `IPAM` 能力。

## 先决条件

- 准备好一个 Kubernetes 集群, 没有安装任何的 CNI
- Helm、Kubectl、Jq(可选) 二进制工具

## 安装

1. 安装 Weave :

    ```shell
    kubectl apply -f  https://github.com/weaveworks/weave/releases/download/v2.8.1/weave-daemonset-k8s.yaml
    ```

    等待 Pod Running:

    ```shell
    [root@node1 ~]# kubectl get po -n kube-system  | grep weave
    weave-net-ck849                         2/2     Running     4     0   1m
    weave-net-vhmqx                         2/2     Running     4     0   1m
    ```

2. 安装 Spiderpool

    ```shell
    helm repo add spiderpool https://spidernet-io.github.io/spiderpool
    helm install spiderpool spiderpool/spiderpool --namespace kube-system --set ipam.enableSpiderSubnet=true
    ```
   
    > `ipam.enableSpiderSubnet=true`: SpiderPool 的 subnet 功能需要被打开。
    > 
    > 如果您是国内用户，可以指定参数 `--set global.imageRegistryOverride=ghcr.m.daocloud.io` 避免 Spiderpool 的镜像拉取失败。

    等待 Pod Running， 创建 Pod 的子网(SpiderSubnet):

     ```shell
     cat << EOF | kubectl apply -f -
     apiVersion: spiderpool.spidernet.io/v2beta1
     kind: SpiderSubnet
     metadata:
       name: weave-subnet-v4
       labels:  
         ipam.spidernet.io/subnet-cidr: 10-32-0-0-12
     spec:
       ips:
       - 10.32.0.100-10.32.50.200
       subnet: 10.32.0.0/12
     EOF
     ```

     > `Weave` 使用 `10.32.0.0/12` 作为集群默认子网。所以这里需要创建一个相同子网的 SpiderSubnet

3. 验证安装

   ```shell
    [root@node1 ~]# kubectl get po -n kube-system | grep spiderpool
    spiderpool-agent-lgdw7                  1/1     Running   0          65s
    spiderpool-agent-x974l                  1/1     Running   0          65s
    spiderpool-controller-9df44bc47-hbhbg   1/1     Running   0          65s
    [root@node1 ~]# kubectl get ss
    NAME               VERSION   SUBNET         ALLOCATED-IP-COUNT   TOTAL-IP-COUNT   DISABLE
    weave-subnet-v4    4         10.32.0.0/12   0                    12901            false
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

修改为:

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

使用注解: `ipam.spidernet.io/subnet` 指定 Pod 从该 SpiderSubnet 中分配 IP:

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
          ipam.spidernet.io/subnet: '{"ipv4":["weave-subnet-v4"]}'
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

> _spec.template.metadata.annotations.ipam.spidernet.io/subnet_：指定 Pod 从 SpiderSubnet:  `weave-subnet-v4` 中分配 IP

Pod 成功创建, 并且从 Spiderpool Subnet 中分配 IP 地址:

  ```shell
  [root@node1 ~]# kubectl get po  -o wide
  NAME                     READY   STATUS    RESTARTS   AGE   IP             NODE    NOMINATED NODE   READINESS GATES
  nginx-5745d9b5d7-2rvn7   1/1     Running   0          8s    10.32.22.190   node1   <none>           <none>
  nginx-5745d9b5d7-5ssck   1/1     Running   0          8s    10.32.35.87    node2   <none>           <none>
  ```

Spiderpool 为该 Nginx 应用自动创建了一个 IP 池: `auto-deployment-default-nginx-v4-a0ae75eb5d47`, 池的 IP 数量为 2:

  ```shell
  [root@node1 ~]# kubectl get sp
  NAME                                            VERSION   SUBNET          ALLOCATED-IP-COUNT   TOTAL-IP-COUNT   DISABLE
  auto-deployment-default-nginx-v4-a0ae75eb5d47   4         10.32.0.0/12    2                    2                false
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

测试结果表明，IP 分配正常、网络连接正常。 通过`Spiderpool`, 确实扩展了 `Weave` 的 `IPAM` 能力。接下来，你可以参考 [Spiderpool 使用](https://spidernet-io.github.io/spiderpool/)，体验 `Spiderpool` 其他的功能。
