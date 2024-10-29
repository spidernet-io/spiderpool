# IPAM 对 operator 支持

**简体中文** ｜ [**English**](./operator.md)

## 描述

Operator 通常用于实现自定义控制器。Spiderpool 支持为非 Kubernetes 原生控制器创建的 Pod 分配 IP。可以通过以下两种方式实现：

1. 手动 IPPool

    管理员可以创建 IPPool 对象并为 Pod 分配 IP。

2. 自动 IPPool

    Spiderpool 支持为应用程序自动管理 IPPool，它可以为一个应用程序创建、删除、扩展和缩小一个专用的 SpiderIPPool 对象，并为其分配静态 IP 地址。

    此功能使用 informer 技术来监视应用程序，解析其副本数量并管理 SpiderIPPool 对象，它与 Kubernetes 原生控制器（如 Deployment、ReplicaSet、StatefulSet、Job、CronJob、DaemonSet）配合良好。

    此功能也支持非 Kubernetes 原生控制器，但 Spiderpool 无法解析非 Kubernetes 原生控制器的对象 yaml，存在一些限制：

    * 不支持自动扩展和缩小 IP
    * 不支持自动删除 IPPool

    未来，Spiderpool 可能会支持自动 IPPool 的所有操作。

另一个关于非 Kubernetes 原生控制器的问题是有状态或无状态。因为 Spiderpool 无法判断由非 Kubernetes 原生控制器创建的应用程序是否有状态。
所以 Spiderpool 将它们视为 `无状态` Pod，如 `Deployment`，这意味着由非 Kubernetes 原生控制器创建的 Pod 能够像 `Deployment` 一样固定 IP 范围，但不能像 `Statefulset` 一样将每个 Pod 绑定到特定的 IP 地址。

## 入门

将使用 [OpenKruise](https://openkruise.io/zh/docs/) 来演示 Spiderpool 如何支持 operator。

### 设置 Spiderpool

请参阅 [安装](./install/get-started-kind.md) 以获取更多详细信息。

### 设置 OpenKruise

请参考 [OpenKruise](https://openkruise.io/docs/installation/)

### 通过 `手动 IPPool` 方式创建 Pod

1. 创建一个自定义 IPPool。

    ```bash
    kubectl apply -f https://raw.githubusercontent.com/spidernet-io/spiderpool/main/docs/example/basic/custom-ipv4-ippool.yaml
    ```

    ```yaml
    apiVersion: spiderpool.spidernet.io/v2beta1
    kind: SpiderIPPool
    metadata:
      name: custom-ipv4-ippool
    spec:
      subnet: 172.18.41.0/24
      ips:
        - 172.18.41.40-172.18.41.50
    ```

2. 创建一个具有 3 个副本的 OpenKruise CloneSet，并通过注释 `ipam.spidernet.io/ippool` 指定 IPPool

    ```yaml
    apiVersion: apps.kruise.io/v1alpha1
    kind: CloneSet
    metadata:
      name: custom-kruise-cloneset
    spec:
      replicas: 3
      selector:
        matchLabels:
          app: custom-kruise-cloneset
      template:
        metadata:
          annotations:
            ipam.spidernet.io/ippool: |-
              {
                "ipv4": ["custom-ipv4-ippool"]
              }
          labels:
            app: custom-kruise-cloneset
        spec:
          containers:
          - name: custom-kruise-cloneset
            image: busybox
            imagePullPolicy: IfNotPresent
            command: ["/bin/sh", "-c", "trap : TERM INT; sleep infinity & wait"]
    ```

    如预期，OpenKruise CloneSet `custom-kruise-cloneset` 的 Pod 将从 IPPool `custom-ipv4-ippool` 中分配 IP 地址。

    ```bash
    kubectl get po -l app=custom-kruise-cloneset -o wide
    NAME                           READY   STATUS    RESTARTS   AGE   IP             NODE            NOMINATED NODE   READINESS GATES
    custom-kruise-cloneset-8m9ls   1/1     Running   0          96s   172.18.41.44   spider-worker   <none>           2/2
    custom-kruise-cloneset-c4z9f   1/1     Running   0          96s   172.18.41.50   spider-worker   <none>           2/2
    custom-kruise-cloneset-w9kfm   1/1     Running   0          96s   172.18.41.46   spider-worker   <none>           2/2
    ```

## 通过 `自动 IPPool` 方式创建 Pod

1. 创建一个具有 3 个副本的 OpenKruise CloneSet，并通过注释 `ipam.spidernet.io/subnet` 指定子网

    ```yaml
    apiVersion: apps.kruise.io/v1alpha1
    kind: CloneSet
    metadata:
      name: custom-kruise-cloneset
    spec:
      replicas: 3
      selector:
        matchLabels:
          app: custom-kruise-cloneset
      template:
        metadata:
          annotations:
            ipam.spidernet.io/subnet: |- 
              {"ipv4": ["subnet-demo-v4"], "ipv6": ["subnet-demo-v6"]}
            ipam.spidernet.io/ippool-ip-number: "5"
          labels:
            app: custom-kruise-cloneset
        spec:
          containers:
          - name: custom-kruise-cloneset
            image: busybox
            imagePullPolicy: IfNotPresent
            command: ["/bin/sh", "-c", "trap : TERM INT; sleep infinity & wait"]
    ```

    > 注意：
    >
    > 1. 必须为自动创建的 IPPool 指定固定的 IP 数量，如 `ipam.spidernet.io/ippool-ip-number: "5"`。
      因为 Spiderpool 无法知道副本数量，所以不支持类似 `ipam.spidernet.io/ippool-ip-number: "+5"` 的注释。

2. 检查状态

    如预期，Spiderpool 将从 `subnet-demo-v4` 和 `subnet-demo-v6` 对象中创建自动创建的 IPPool。
    OpenKruise CloneSet `custom-kruise-cloneset` 的 Pod 将从创建的 IPPool 中分配 IP 地址。

    ```text
    $ kubectl get sp | grep kruise
    NAME                                      VERSION   SUBNET                    ALLOCATED-IP-COUNT   TOTAL-IP-COUNT   DEFAULT   DISABLE   APP-NAMESPACE
    auto4-custom-kruise-cloneset-eth0-028d6   4         172.16.0.0/16             3                    5                false     false     default
    auto6-custom-kruise-cloneset-eth0-028d6   6         fc00:f853:ccd:e790::/64   3                    5                false     false     default
    ------------------------------------------------------------------------------------------
    $ kubectl get po -l app=custom-kruise-cloneset -o wide
    NAME                           READY   STATUS    RESTARTS   AGE   IP            NODE            NOMINATED NODE   READINESS GATES
    custom-kruise-cloneset-f52dn   1/1     Running   0          61s   172.16.41.4   spider-worker   <none>           2/2
    custom-kruise-cloneset-mq67v   1/1     Running   0          61s   172.16.41.5   spider-worker   <none>           2/2
    custom-kruise-cloneset-nprpf   1/1     Running   0          61s   172.16.41.1   spider-worker   <none>           2/2
    ```
