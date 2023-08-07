# Macvlan Quick Start

[**English**](./get-started-macvlan.md) | **简体中文**

Spiderpool 可用作 Underlay 网络场景下提供固定 IP 的一种解决方案，本文将以 [Multus](https://github.com/k8snetworkplumbingwg/multus-cni)、[Macvlan](https://github.com/containernetworking/plugins/tree/main/plugins/main/macvlan)、[Spiderpool](https://github.com/spidernet-io/spiderpool) 为例，搭建一套完整的 underlay 网络解决方案，该方案能够满足以下各种功能需求：

* 通过简易运维，应用可分配到固定的 Underlay IP 地址

* Pod 具备多张 Underlay 网卡，通达多个 Underlay 子网

* Pod 能够通过 Pod IP、clusterIP、nodePort 等方式通信

## 先决条件

1. 准备一个 Kubernetes 集群

2. 已安装 [Helm](https://helm.sh/docs/intro/install/)

## 安装 Macvlan

[`Macvlan`](https://github.com/containernetworking/plugins/tree/main/plugins/main/macvlan) 是一个 CNI 插件项目，能够为 Pod 分配 Macvlan 虚拟网卡，可用于对接 Underlay 网络。

一些 Kubernetes 安装器项目，默认安装了 Macvlan 二进制文件，可确认节点上存在二进制文件 /opt/cni/bin/macvlan 。如果节点上不存在该二进制文件，可参考如下命令，在所有节点上下载安装：

```shell
~# wget https://github.com/containernetworking/plugins/releases/download/v1.2.0/cni-plugins-linux-amd64-v1.2.0.tgz 

~# tar xvfzp ./cni-plugins-linux-amd64-v1.2.0.tgz -C /opt/cni/bin

~# chmod +x /opt/cni/bin/macvlan
```

## 安装 Spiderpool

1. 安装 Spiderpool。

    ```bash
    helm repo add spiderpool https://spidernet-io.github.io/spiderpool

    helm repo update spiderpool

    helm install spiderpool spiderpool/spiderpool --namespace kube-system --set multus.multusCNI.defaultCniCRName="macvlan-conf"
    ```

    > 如果您是国内用户，可以指定参数 `--set global.imageRegistryOverride=ghcr.m.daocloud.io` 避免 Spiderpool 的镜像拉取失败。
    >
    > 通过 `multus.multusCNI.defaultCniCRName` 指定集群的 Multus clusterNetwork，clusterNetwork 是 Multus 插件的一个特定字段，用于指定 Pod 的默认网络接口。

2. 创建 SpiderIPPool 实例。

    创建与网络接口 `eth0` 在同一个子网的 IP 池以供 Pod 使用，以下是创建相关的 SpiderIPPool 示例：

    ```shell
    cat <<EOF | kubectl apply -f -
    apiVersion: spiderpool.spidernet.io/v2beta1
    kind: SpiderIPPool
    metadata:
      name: ippool-test
    spec:
      ips:
      - "172.18.30.131-172.18.30.140"
      subnet: 172.18.0.0/16
      gateway: 172.18.0.1
      multusName: 
      - macvlan-conf
    EOF
    ```

3. 验证安装

   ```shell
    ~# kubectl get po -n kube-system | grep spiderpool
    spiderpool-agent-7hhkz                   1/1     Running     0              13m
    spiderpool-agent-kxf27                   1/1     Running     0              13m
    spiderpool-controller-76798dbb68-xnktr   1/1     Running     0              13m
    spiderpool-init                          0/1     Completed   0              13m
    spiderpool-multus-7vkm2                  1/1     Running     0              13m
    spiderpool-multus-rwzjn                  1/1     Running     0              13m
    ~# kubectl get sp
    NAME            VERSION   SUBNET          ALLOCATED-IP-COUNT   TOTAL-IP-COUNT   DISABLE
    ippool-test     4         172.18.0.0/16   0                    10               false
   ```

## 创建 CNI 配置

Spiderpool 为简化书写 JSON 格式的 Multus CNI 配置，它提供了 SpiderMultusConfig CR 来自动管理 Multus NetworkAttachmentDefinition CR。如下是创建 Macvlan SpiderMultusConfig 配置的示例：

* 确认 Macvlan 所需的宿主机父接口，本例子以宿主机 eth0 网卡为例，从该网卡创建 Macvlan 子接口给 Pod 使用

    ```shell
    MACVLAN_MASTER_INTERFACE="eth0"
    cat <<EOF | kubectl apply -f -
    apiVersion: spiderpool.spidernet.io/v2beta1
    kind: SpiderMultusConfig
    metadata:
      name: macvlan-conf
      namespace: kube-system
    spec:
      cniType: macvlan
      macvlan:
        master:
        - ${MACVLAN_MASTER_INTERFACE}
    EOF
    ```

在本文示例中，使用如上配置，创建如下的 Macvlan SpiderMultusConfig，将基于它自动生成的 Multus NetworkAttachmentDefinition CR，它对应了宿主机的 eth0 网卡。

```bash
~# kubectl get spidermultusconfigs.spiderpool.spidernet.io -n kube-system
NAME           AGE
macvlan-conf   10m

~# kubectl get network-attachment-definitions.k8s.cni.cncf.io -n kube-system
NAME           AGE
macvlan-conf   10m
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
            ipam.spidernet.io/ippool: |-
              {
                "ipv4": ["ippool-test"]
              }
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

2. 查看 Pod 运行状态：

    ```bash
    ~# kubectl get po -l app=test-app -o wide
    NAME                      READY   STATUS    RESTARTS   AGE     IP              NODE                 NOMINATED NODE   READINESS GATES
    test-app-f9f94688-2srj7   1/1     Running   0          2m13s   172.18.30.139   ipv4-worker          <none>           <none>
    test-app-f9f94688-8982v   1/1     Running   0          2m13s   172.18.30.138   ipv4-control-plane   <none>           <none>
    ```

3. 应用的 IP 将会固定在该 IP 范围内：

    ```bash
    ~# kubectl get spiderippool
    NAME          VERSION   SUBNET           ALLOCATED-IP-COUNT   TOTAL-IP-COUNT   DEFAULT    
    ippool-test   4         172.18.0.0/16    2                    10                false
    
    ~#  kubectl get spiderendpoints
    NAME                      INTERFACE   IPV4POOL      IPV4               IPV6POOL   IPV6   NODE                 CREATETION TIME
    test-app-f9f94688-2srj7   eth0        ippool-test   172.18.30.139/16                     ipv4-worker          3m5s
    test-app-f9f94688-8982v   eth0        ippool-test   172.18.30.138/16                     ipv4-control-plane   3m5s
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
