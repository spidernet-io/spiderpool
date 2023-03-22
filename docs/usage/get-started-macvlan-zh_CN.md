# 搭建 spiderpool

spiderpool 可用于 underlay 网络场景下提供固定 IP 的解决方案，本文将
以 [Multus](https://github.com/k8snetworkplumbingwg/multus-cni)、[Macvlan]((https://github.com/containernetworking/plugins/tree/main/plugins/main/macvlan))、
[Veth](https://github.com/spidernet-io/plugins)、Spiderpool 来来搭建一套完整的 underlay 网络解决方案，该方案能够满足以下各种功能需求：

* 通过简易运维，应用可分配到固定的 underlay IP 地址

* POD 具备多张 underlay 网卡，通达多个 underlay 子网

* POD 能够通过 POD IP、 clusterIP、nodePort 等方式通信

## 先决条件

1. 准备一个 Kubernetes 集群

2. [Helm](https://helm.sh/docs/intro/install/) 工具

## 安装 Macvlan 

[`Macvlan`](https://github.com/containernetworking/plugins/tree/main/plugins/main/macvlan) 是 一个 CNI 插件项目，能够为 POD 分配 macvlan 虚拟网卡，可用于对接 underlay 网络。

一些 kubernetes 安装器项目，默认安装了 macvlan 二进制文件，可确认节点上存在二进制文件 /opt/cni/bin/macvlan 。如果节点上不存在该二进制文件，可参考如下命令，在所有节点上下载安装：

    ~# wget https://github.com/containernetworking/plugins/releases/download/v1.2.0/cni-plugins-linux-amd64-v1.2.0.tgz 

    ~# tar xvfzp ./cni-plugins-linux-amd64-v1.2.0.tgz -C /opt/cni/bin

    ~# chmod +x /opt/cni/bin/macvlan

### 安装 Veth

[`Veth`](https://github.com/spidernet-io/plugins) 是一个 CNI 插件，它能够帮助一些 CNI （例如 macvlan、sriov等）解决如下问题：

* 在 macvlan CNI 场景下，帮助 POD 实现 clusterIP 通信

* 若 POD macvlan 的 IP 不能与本地宿主机通信，会影响 POD 的健康检测。Veth 插件能够帮助 POD 与宿主机通信能力，解决健康检测场景下的联通性问题

* 在 POD 多网卡场景下，Veth 能自动够协调多网卡间的策略路由，解决多网卡通信问题

请在所有的节点上，下载安装 Veth 二进制：

    ~# wget ./ https://github.com/spidernet-io/plugins/releases/download/v0.1.4/spider-plugins-linux-amd64-v0.1.4.tar

    ~# tar xvfzp ./spider-plugins-linux-amd64-v0.1.4.tar -C /opt/cni/bin

    ~# chmod +x /opt/cni/bin/veth

## 安装 Multus

[`Multus`](https://github.com/k8snetworkplumbingwg/multus-cni) 是一个 CNI 插件项目，它通过调度第三方 CNI 项目，能够实现为 POD 接入多张网卡。
并且，multus提供了 CRD 方式管理 macvlan 的 CNI 配置，避免在每个主机上手动编辑 CNI 配置文件。

1. 通过 manifest 安装 Multus 的组件。

    ```bash
    ~# kubectl apply -f https://raw.githubusercontent.com/k8snetworkplumbingwg/multus-cni/v3.9/deployments/multus-daemonset.yml
    ```

2. 确认 Multus 运维状态：

        ~# kubectl get pods -A | grep -i multus
        kube-system          kube-multus-ds-hfzpl                         1/1     Running   0   5m
        kube-system          kube-multus-ds-qm8j7                         1/1     Running   0   5m
   
    确认节点上存在 multus 的配置文件 ` ls /etc/cni/net.d/00-multus.conf`

3. 为 macvlan 创建 multus 的 NetworkAttachmentDefinition 配置

需要确认如下配置：

* 确认 macvlan 所需的宿主机父接口，本例子以宿主机 eth0 网卡为例，从该网卡创建 macvlan 子接口给 POD 使用

* 为使用 veth 插件来实现 clusterIP 通信，需确认集群 service 的 clusterIP CIDR， . 例如可基于命令 `kubectl -n kube-system get configmap kubeadm-config -oyaml | grep service-cluster-ip-range` 查询

以下创建 NetworkAttachmentDefinition 配置

```shell
MACLVAN_MASTER_INTERFACE="eth0"
CLUSTERIP_CIDR="10.96.0.0/16"

cat <<EOF | kubectl apply -f -
apiVersion: k8s.cni.cncf.io/v1
kind: NetworkAttachmentDefinition
metadata:
  name: macvlan-conf
  namespace: kube-system
spec:
  config: |-
    {
        "cniVersion": "0.3.1",
        "name": "macvlan-conf",
        "plugins": [
            {
                "type": "macvlan",
                "master": "${MACLVAN_MASTER_INTERFACE}",
                "mode": "bridge",
                "ipam": {
                    "type": "spiderpool"
                }
            },{
                  "type": "veth",
                  "service_cidr": ["${CLUSTERIP_CIDR}"]
              }
        ]
    }
EOF
```

## 安装 Spiderpool

1. 安装 spiderpool

        helm repo add spiderpool https://spidernet-io.github.io/spiderpool
        helm update repo spiderpool

        helm install spiderpool spiderpool/spiderpool --namespace kube-system \
          --set feature.enableIPv4=true --set feature.enableIPv6=false

2. 创建 SpiderSubnet 实例。
macvlan 是 以宿主机 eth0 为父接口，因此，需要创建 eth0 底层的 underlay 子网供 POD 使用。
以下创建相关的 SpiderSubnet 实例

        cat <<EOF | kubectl apply -f -
        apiVersion: spiderpool.spidernet.io/v2beta1
        kind: SpiderSubnet
        metadata:
        name: subnet-test
        spec:
          gateway: 172.18.0.1
          ips:
          - "172.18.30.131-172.18.30.140"
          subnet: 172.18.0.0/16
       EOF

## 创建应用

1. 使用如下命令创建测试 Pod：

    ```shell
    cat <<EOF | kubectl create -f -
    apiVersion: apps/v1
    kind: Deployment
    metadata:
      name: custom-ippool-deploy
    spec:
      replicas: 2
      selector:
        matchLabels:
          app: custom-ippool-deploy
      template:
        metadata:
          annotations:
            ipam.spidernet.io/ippool: |-
              {
                "ipv4": ["subnet-test"]
              }
            v1.multus-cni.io/default-network: kube-system/macvlan-conf
          labels:
            app: custom-ippool-deploy
        spec:
          containers:
          - name: custom-ippool-deploy
            image: ghcr.io/daocloud/dao-2048:v1.2.0  ?????????????????????
            imagePullPolicy: IfNotPresent
            ports:
            - name: http
              containerPort: 80
              protocol: TCP
    EOF
    ```

   必要参数说明：
   > `ipam.spidernet.io/ippool`：该 annotation 指定使用哪个 subnet 分配 IP 地址给 POD

   更多 Spiderpool 注解的使用请参考 [Spiderpool 注解](https://spidernet-io.github.io/spiderpool/concepts/annotation/)。

   > `v1.multus-cni.io/default-network`：该 annotation 指定了使用的 multus 的 CNI 配置。

   更多 Multus 注解使用请参考 [Multus 注解](https://github.com/k8snetworkplumbingwg/multus-cni/blob/master/docs/quickstart.md)

2. 查看 Pod 运行状态：

    ```bash
    ~# kubectl get po -l app=custom-ippool-deploy -o wide
    NAME                                    READY   STATUS    RESTARTS   AGE   IP              NODE                 NOMINATED NODE   READINESS GATES
    custom-ippool-deploy-66d4669dd5-j97rc   1/1     Running   0          61s   172.18.55.180   ipv4-worker          <none>           <none>
    custom-ippool-deploy-66d4669dd5-nswpq   1/1     Running   0          61s   172.18.53.49    ipv4-control-plane   <none>           <none>

    ```

3. spidepool 自动为应用创建了 IP 固定池，应用的 IP 将会自动固定在该 IP 范围内

    ？？？？？？？？？？？？？？？

4. 查看 Pod 的网络：

    ```bash
    > kubectl exec -ti custom-ippool-deploy-66d4669dd5-j97rc ip a

    1: lo: <LOOPBACK,UP,LOWER_UP> mtu 65536 qdisc noqueue qlen 1000
        link/loopback 00:00:00:00:00:00 brd 00:00:00:00:00:00
        inet 127.0.0.1/8 scope host lo
          valid_lft forever preferred_lft forever
        inet6 ::1/128 scope host
          valid_lft forever preferred_lft forever
    2: tunl0@NONE: <NOARP> mtu 1480 qdisc noop qlen 1000
        link/ipip 0.0.0.0 brd 0.0.0.0
    3: eth0@if96: <BROADCAST,MULTICAST,UP,LOWER_UP,M-DOWN> mtu 1500 qdisc noqueue
        link/ether d6:70:9f:bf:03:09 brd ff:ff:ff:ff:ff:ff
        inet 172.18.55.180/16 brd 172.18.255.255 scope global eth0
          valid_lft forever preferred_lft forever
        inet6 fe80::d470:9fff:febf:309/64 scope link
          valid_lft forever preferred_lft forever
    4: veth0@if10773: <BROADCAST,MULTICAST,UP,LOWER_UP,M-DOWN> mtu 1500 qdisc noqueue
        link/ether f2:58:b4:cd:a0:bc brd ff:ff:ff:ff:ff:ff
        inet6 fe80::f058:b4ff:fecd:a0bc/64 scope link
          valid_lft forever preferred_lft forever
    ```

   > eth0：是由 multus network-attachment-definition: macvlan-conf 所创建，其 IP 从 Spiderpool: default-v4-ippool IP池中分配。

    查看 Pod 的路由，网关等信息

    ```shell
    > kubectl exec -ti custom-ippool-deploy-66d4669dd5-j97rc ip r show

    default via 172.18.0.1 dev eth0
    10.96.0.0/16 dev veth0 scope link
    172.18.0.0/16 dev eth0 scope link  src 172.18.55.180
    172.18.0.3 dev veth0 scope link
    ```

5. 测试 Pod 与 Pod 的通讯情况

    ```shell
    # 172.18.53.49 为跨节点的 macvlan Pod IP
    > kubectl exec -ti custom-ippool-deploy-66d4669dd5-j97rc ping 172.18.53.49
    
    PING 172.18.53.49 (172.18.53.49): 56 data bytes
    64 bytes from 172.18.53.49: seq=0 ttl=64 time=0.253 ms
    64 bytes from 172.18.53.49: seq=1 ttl=64 time=0.189 ms
    ...
    --- 172.18.53.49 ping statistics ---
    2 packets transmitted, 2 packets received, 0% packet loss
    round-trip min/avg/max = 0.189/0.221/0.253 ms

    # 172.18.13.98 为同节点的 macvlan Pod IP
    > kubectl exec -ti custom-ippool-deploy-66d4669dd5-j97rc ping 172.18.13.98

    PING 172.18.13.98 (172.18.13.98): 56 data bytes
    64 bytes from 172.18.13.98: seq=0 ttl=64 time=0.540 ms
    64 bytes from 172.18.13.98: seq=1 ttl=64 time=0.125 ms
    ...
    --- 172.18.13.98 ping statistics ---
    2 packets transmitted, 2 packets received, 0% packet loss
    round-trip min/avg/max = 0.125/0.279/0.540 ms
    ```

6. 测试 Pod 与 service IP 的通讯情况

    - 使用如下命令创建测试 service：

      ```shell
      cat <<EOF | kubectl create -f -
      apiVersion: v1
      kind: Service
      metadata:
        name: custom-ippool-svc
        labels:
          app: custom-ippool-deploy
      spec:
        type: ClusterIP
        ports:
          - port: 80
            protocol: TCP
            targetPort: 80
        selector:
          app: custom-ippool-deploy
      EOF
      ```

    - 查看 service 的 IP：

      ```shell
      > kubectl get svc

      NAME                TYPE        CLUSTER-IP     EXTERNAL-IP   PORT(S)   AGE
      custom-ippool-svc   ClusterIP   10.96.100.43   <none>        80/TCP    7m45s
      ```

    - Pod 内访问集群其它应用的 cluster IP：

      ```shell
      > kubectl exec -ti  custom-ippool-deploy-66d4669dd5-j97rc -- sh

      / # curl 10.96.100.43:80 -I
      
      HTTP/1.1 200 OK
      Server: nginx/1.23.1
      Date: Thu, 09 Mar 2023 09:25:36 GMT
      Content-Type: text/html
      Content-Length: 4055
      Last-Modified: Fri, 23 Sep 2022 02:53:30 GMT
      Connection: keep-alive
      ETag: "632d1faa-fd7"
      Accept-Ranges: bytes
      ```