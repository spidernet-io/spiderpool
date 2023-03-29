# Macvlan Quick Start

[**English**](./get-started-macvlan.md) | **简体中文**

[**English**](./get-started-macvlan.md) | **简体中文**

[**English**](./get-started-macvlan.md) | **简体中文**

Spiderpool 可用作 Underlay 网络场景下提供固定 IP 的一种解决方案，本文将以 [Multus](https://github.com/k8snetworkplumbingwg/multus-cni)、[Macvlan](https://github.com/containernetworking/plugins/tree/main/plugins/main/macvlan)、[Veth](https://github.com/spidernet-io/plugins)、[Spiderpool](https://github.com/spidernet-io/spiderpool) 为例，搭建一套完整的 underlay 网络解决方案，该方案能够满足以下各种功能需求：

* 通过简易运维，应用可分配到固定的 Underlay IP 地址

* Pod 具备多张 Underlay 网卡，通达多个 Underlay 子网

* Pod 能够通过 Pod IP、clusterIP、nodePort 等方式通信

## 先决条件

1. 准备一个 Kubernetes 集群

2. 已安装 [Helm](https://helm.sh/docs/intro/install/)

## 安装 Macvlan 

[`Macvlan`](https://github.com/containernetworking/plugins/tree/main/plugins/main/macvlan) 是一个 CNI 插件项目，能够为 Pod 分配 Macvlan 虚拟网卡，可用于对接 Underlay 网络。

一些 Kubernetes 安装器项目，默认安装了 Macvlan 二进制文件，可确认节点上存在二进制文件 /opt/cni/bin/macvlan 。如果节点上不存在该二进制文件，可参考如下命令，在所有节点上下载安装：

```
~# wget https://github.com/containernetworking/plugins/releases/download/v1.2.0/cni-plugins-linux-amd64-v1.2.0.tgz 

~# tar xvfzp ./cni-plugins-linux-amd64-v1.2.0.tgz -C /opt/cni/bin

~# chmod +x /opt/cni/bin/macvlan
```


## 安装 Veth

[`Veth`](https://github.com/spidernet-io/plugins) 是一个 CNI 插件，它能够帮助一些 CNI （例如 Macvlan、SR-IOV 等）解决如下问题：

* 在 Macvlan CNI 场景下，帮助 Pod 实现 clusterIP 通信

* 若 Pod 的 Macvlan IP 不能与本地宿主机通信，会影响 Pod 的健康检测。Veth 插件能够帮助 Pod 与宿主机通信，解决健康检测场景下的联通性问题

* 在 Pod 多网卡场景下，Veth 能自动够协调多网卡间的策略路由，解决多网卡通信问题

请在所有的节点上，下载安装 Veth 二进制：

```
~# wget https://github.com/spidernet-io/plugins/releases/download/v0.1.4/spider-plugins-linux-amd64-v0.1.4.tar

~# tar xvfzp ./spider-plugins-linux-amd64-v0.1.4.tar -C /opt/cni/bin

~# chmod +x /opt/cni/bin/veth
```

## 安装 Multus

[`Multus`](https://github.com/k8snetworkplumbingwg/multus-cni) 是一个 CNI 插件项目，它通过调度第三方 CNI 项目，能够实现为 Pod 接入多张网卡。并且，Multus 提供了 CRD 方式管理 Macvlan 的 CNI 配置，避免在每个主机上手动编辑 CNI 配置文件。

1. 通过 manifest 安装 Multus 的组件。

    ```bash
    ~# kubectl apply -f https://raw.githubusercontent.com/k8snetworkplumbingwg/multus-cni/v3.9/deployments/multus-daemonset.yml
    ```

2. 确认 Multus 运维状态：

    ```bash
    ~# kubectl get pods -A | grep -i multus
    kube-system          kube-multus-ds-hfzpl                         1/1     Running   0   5m
    kube-system          kube-multus-ds-qm8j7                         1/1     Running   0   5m
    ```

    确认节点上存在 Multus 的配置文件 `ls /etc/cni/net.d/00-multus.conf`

3. 为 Macvlan 创建 Multus 的 NetworkAttachmentDefinition 配置。

    需要确认如下参数：

    * 确认 Macvlan 所需的宿主机父接口，本例子以宿主机 eth0 网卡为例，从该网卡创建 Macvlan 子接口给 Pod 使用

    * 为使用 Veth 插件来实现 clusterIP 通信，需确认集群的 service CIDR，例如可基于命令 `kubectl -n kube-system get configmap kubeadm-config -oyaml | grep service` 查询

    以下为创建 NetworkAttachmentDefinition 的配置：

    ```shell
    MACVLAN_MASTER_INTERFACE="eth0"
    SERVICE_CIDR="10.96.0.0/16"

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
                      "service_cidr": ["${SERVICE_CIDR}"]
                  }
            ]
        }
    EOF
    ```

## 安装 Spiderpool

1. 安装 Spiderpool CRD。

    ```bash
    helm repo add spiderpool https://spidernet-io.github.io/spiderpool

    helm repo update spiderpool

    helm install spiderpool spiderpool/spiderpool --namespace kube-system \
        --set feature.enableIPv4=true --set feature.enableIPv6=false 
    ```

2. 创建 SpiderSubnet 实例。

    Macvlan 是以宿主机 eth0 为父接口，因此，需要创建 eth0 底层的 Underlay 子网供 Pod 使用。
    以下是创建相关的 SpiderSubnet 示例：

    ```shell
    cat <<EOF | kubectl apply -f -
    apiVersion: spiderpool.spidernet.io/v2beta1
    kind: SpiderSubnet
    metadata:
      name: subnet-test
    spec:
      ipVersion: 4
      ips:
      - "172.18.30.131-172.18.30.140"
      subnet: 172.18.0.0/16
      gateway: 172.18.0.1
      vlan: 0
    EOF
    ```

## 创建应用

1. 使用如下命令创建测试 Pod 和 service：

    ```shell
    cat <<EOF | kubectl create -f -
    apiVersion: apps/v1
    kind: Deployment
    metadata:
      name: test-app
    spec:
      replicas: 2
      selector:
        matchLabels:
          app: test-app
      template:
        metadata:
          annotations:
            ipam.spidernet.io/subnet: |-
              {
                "ipv4": ["subnet-test"]
              }
            v1.multus-cni.io/default-network: kube-system/macvlan-conf
          labels:
            app: test-app
        spec:
          containers:
          - name: test-app
            image: nginx
            imagePullPolicy: IfNotPresent
            ports:
            - name: http
              containerPort: 80
              protocol: TCP
    ---
    apiVersion: v1
    kind: Service
    metadata:
      name: test-app-svc
      labels:
        app: test-app
    spec:
      type: ClusterIP
      ports:
        - port: 80
          protocol: TCP
          targetPort: 80
      selector:
        app: test-app 
    EOF
    ```

    必要参数说明：
    * `ipam.spidernet.io/subnet`：该 annotation 指定使用哪个 subnet 分配 IP 地址给 Pod
        > 更多 Spiderpool 注解的使用请参考 [Spiderpool 注解](https://spidernet-io.github.io/spiderpool/concepts/annotation/)。

    * `v1.multus-cni.io/default-network`：该 annotation 指定了使用的 Multus 的 CNI 配置。
        > 更多 Multus 注解使用请参考 [Multus 注解](https://github.com/k8snetworkplumbingwg/multus-cni/blob/master/docs/quickstart.md)。

2. 查看 Pod 运行状态：

    ```bash
    ~# kubectl get po -l app=test-app -o wide
    NAME                      READY   STATUS    RESTARTS   AGE     IP              NODE                 NOMINATED NODE   READINESS GATES
    test-app-f9f94688-2srj7   1/1     Running   0          2m13s   172.18.30.139   ipv4-worker          <none>           <none>
    test-app-f9f94688-8982v   1/1     Running   0          2m13s   172.18.30.138   ipv4-control-plane   <none>           <none>
    ```

3. Spiderpool 自动为应用创建了 IP 固定池，应用的 IP 将会自动固定在该 IP 范围内：

    ```bash
    ~# kubectl get spiderippool
    NAME                                               VERSION   SUBNET          ALLOCATED-IP-COUNT   TOTAL-IP-COUNT   DISABLE
    auto-deployment-default-test-app-v4-a0ae75eb5d47   4         172.18.0.0/16   2                    2                false
    
    ~#  kubectl get spiderendpoints
    NAME                      INTERFACE   IPV4POOL                                           IPV4               IPV6POOL   IPV6   NODE                 CREATETION TIME
    test-app-f9f94688-2srj7   eth0        auto-deployment-default-test-app-v4-a0ae75eb5d47   172.18.30.139/16                     ipv4-worker          3m5s
    test-app-f9f94688-8982v   eth0        auto-deployment-default-test-app-v4-a0ae75eb5d47   172.18.30.138/16                     ipv4-control-plane   3m5s
    ```


4. 测试 Pod 与 Pod 的通讯情况：

    ```shell
    ~# kubectl exec -ti test-app-f9f94688-2srj7 -- ping 172.18.30.138 -c 2
    
    PING 172.18.30.138 (172.18.30.138): 56 data bytes
    64 bytes from 172.18.30.138: seq=0 ttl=64 time=1.524 ms
    64 bytes from 172.18.30.138: seq=1 ttl=64 time=0.194 ms

    --- 172.18.30.138 ping statistics ---
    2 packets transmitted, 2 packets received, 0% packet loss
    round-trip min/avg/max = 0.194/0.859/1.524 ms
    ```

5. 测试 Pod 与 service IP 的通讯情况：

    * 查看 service 的 IP：

        ```shell
        ~# kubectl get service

        NAME           TYPE        CLUSTER-IP    EXTERNAL-IP   PORT(S)   AGE
        kubernetes     ClusterIP   10.96.0.1     <none>        443/TCP   20h
        test-app-svc   ClusterIP   10.96.190.4   <none>        80/TCP    109m
        ```

    * Pod 内访问自身的 service ：

        ```bash
        ~# kubectl exec -ti  test-app-85cf87dc9c-7dm7m -- curl 10.96.190.4:80 -I

        HTTP/1.1 200 OK
        Server: nginx/1.23.1
        Date: Thu, 23 Mar 2023 05:01:04 GMT
        Content-Type: text/html
        Content-Length: 4055
        Last-Modified: Fri, 23 Sep 2022 02:53:30 GMT
        Connection: keep-alive
        ETag: "632d1faa-fd7"
        Accept-Ranges: bytes
        ```
