# Dynamic-Resource-Allocation

## 介绍

动态资源分配（DRA）是 Kubernetes 推出的一项新 feature，它将资源调度交到第三方开发人员手中。它摒弃了之前 device-plugin 请求访问资源时的可计数的模式（例如 "nvidia.com/gpu: 2"），提供了更类似于存储持久卷的 API。它的主要好处是更加灵活、动态的分配硬件资源，提高了资源的利用率。并且增强资源调度、使 Pod 能够调度最佳节点。目前在 Nvidia 和 Intel 的推动下，DRA 已经作为 Kubernetes 1.26（2022 年 12 月发布）的 alpha 功能。

目前 Spiderpool 已经集成 DRA 框架，基于该功能可实现以下但不限于的能力:

* 可根据每个节点上报的网卡和子网信息，并结合 Pod 使用的 SpiderMultusConfig 配置，自动调度到合适的节点，避免 Pod 调度到节点之后无法启动
* 在 SpiderClaimParameter 中统一多个 device-plugin 如 [sriov-network-device-plugin](https://github.com/k8snetworkplumbingwg/sriov-network-device-plugin), [k8s-rdma-shared-dev-plugin](https://github.com/Mellanox/k8s-rdma-shared-dev-plugin) 的资源使用方式
* 持续更新, 详见 [RoadMap](../develop/roadmap.md)

## 名称解释

* ResourceClaimTemplate: resourceclaim 模版，用于生成 resourceclaim 资源。一份 resourceClaimTemplate 可以生成多个 resourceclaim.
* ResourceClaim: ResourceClaim 绑定一组特定的节点资源，供于 Pod 使用。
* ResourceClass: 一种 ResourceClass 代表一种资源(比如 GPU), 一种 DRA 插件负责驱动一种 ResourceClass 所代表的资源。

## 环境准备

1. 准备一个高版本的 Kubernetes 集群, 推荐版本大于 v1.29.0, 并且开启集群的 dra feature-gate 功能
2. 已安装 Kubectl、[Helm](https://helm.sh/docs/intro/install/)

## 快速开始

1. 目前 DRA 作为 Kubernetes 的 Alpha 功能，默认不打开。所以我们需要以手动方式开启，步骤如下:

    在 kube-apiserver 的启动参数中加入:

    ```shell
        - --feature-gates=DynamicResourceAllocation=true
        - --runtime-config=resource.k8s.io/v1alpha2=true
    ```

    在 kube-controller-manager 的启动参数中加入:

    ```shell
        - --feature-gates=DynamicResourceAllocation=true
    ```

    在 kube-scheduler 的启动参数中加入:

    ```shell
        - --feature-gates=DynamicResourceAllocation=true
    ```

2. DRA 需要依赖 [CDI](https://github.com/cncf-tags/container-device-interface), 所以需要容器运行时支持。本文以 containerd 为例，需要手动开启 cdi 功能:

    修改 containerd 的配置文件，配置 CDI:

    ```shell
    ~# vim /etc/containerd/config.toml
    ...
    [plugins."io.containerd.grpc.v1.cri"]
    enable_cdi = true
    cdi_spec_dirs = ["/etc/cdi", "/var/run/cdi"]

    ~# systemctl restart containerd
    ```

    > 建议 containerd 版本大于 v1.7.0, 此后版本才支持 CDI 功能。不同运行时支持的版本不一致，请先检查是否支持。

3. 安装 Spiderpool, 注意开启 CDI 功能

    ```shell
    helm repo add spiderpool https://spidernet-io.github.io/spiderpool

    helm repo update spiderpool

    helm install spiderpool spiderpool/spiderpool --namespace kube-system --set dra.enabled=true 
    ```

4. 验证安装

    检查 Spiderpool pod 是否正常 running, 并检查是否存在 resourceclass 资源:

    ```shell
    ~# kubectl get po -n kube-system | grep spiderpool
    spiderpool-agent-hqt2b                                  1/1     Running     0             20d
    spiderpool-agent-nm9vl                                  1/1     Running     0             20d
    spiderpool-controller-7d7f4f55d4-w2rv5                  1/1     Running     0             20d
    spiderpool-init                                         0/1     Completed   0             21d
    ~# kubectl get resourceclass
    NAME                        DRIVERNAME                  AGE
    netresources.spidernet.io   netresources.spidernet.io   20d
    ```

    > netresources.spidernet.io 为 Spiderpool 的 resourceclass, Spiderpool 将会关注属于该 resourceclass 的 resourceclaim 的创建与分配

5. 创建 SpiderIPPool 和 SpiderMultusConfig 实例:

    > 注意: 如果您的集群已经安装了其他 CNI 或不需要使用 Macvlan 的 underlay CNI，这一步可以跳过。

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

    > SpiderMultusConfig 将会自动创建 Multus network-attachment-definetion 实例

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
      - kube-system/macvlan-conf
    EOF
    ```

6. 创建工作负载和 resourceClaim 等资源文件:

    ```shell
    ~# export NAME=demo
    ~# cat <<EOF | kubectl apply -f - 
    apiVersion: spiderpool.spidernet.io/v2beta1
    kind: SpiderClaimParameter
    metadata:
      name: ${NAME}
    ---
    apiVersion: resource.k8s.io/v1alpha2
    kind: ResourceClaimTemplate
    metadata:
      name: ${NAME}
    spec:
      spec:
        resourceClassName: netresources.spidernet.io
        parametersRef:
          apiGroup: spiderpool.spidernet.io
          kind: SpiderClaimParameter
          name: ${NAME}
    ---
    apiVersion: apps/v1
    kind: Deployment
    metadata:
      name: ${NAME}
    spec:
      replicas: 2
      selector:
        matchLabels:
          app: ${NAME}
      template:
        metadata:
          annotations:
            v1.multus-cni.io/default-network: kube-system/macvlan-conf
        labels:
            app: ${NAME}
        spec:
          containers:
          - name: ctr
            image: nginx
            resources:
              claims:
              - name: ${NAME}
          resourceClaims:
          - name: ${NAME}
            source:
              resourceClaimTemplateName: ${NAME}
    EOF
    ```

    > 创建一个 ResourceClaimTemplate, K8s 将会根据这个 ResourceClaimTemplate 为每个 Pod 创建自己独有的 Resourceclaim。该 Resourceclaim 的声明周期与该 Pod保持一致。
    >
    > SpiderClaimParameter 用于扩展 ResourceClaim 的配置参数，将会影响 ResourceClaim 的调度以及其 CDI 文件的生成。
    >
    > Pod 的 container 通过在 Resources 中声明 claims 的使用，这将影响 containerd 所需要的资源。容器运行时会将该 claim 对应的 CDI 文件翻译为 OCI Spec配置，从而决定container的创建。
    >
    > 如果创建 Pod 失败，提示 “unresolvable CDI devices: xxxx”, 这可能是容器运行时支持的 CDI 版本过低，导致容器运行时无法解析 cdi 文件。目前 Spiderpool 默认的 CDI 版本为最新。可以通过在 SpiderClaimParameter 实例中通过 annotation: "dra.spidernet.io/cdi-version" 指定较低版本，比如: dra.spidernet.io/cdi-version: 0.5.0

7. 验证

    创建 Pod 之后, 查看生成的 ResourceClaim 等资源文件:

    ```shell
    ~# kubectl get resourceclaim
    NAME                                                           RESOURCECLASSNAME           ALLOCATIONMODE         STATE                AGE
    demo-745fb4c498-72g7g-demo-7d458                               netresources.spidernet.io   WaitForFirstConsumer   allocated,reserved   20d
    ~# cat /var/run/cdi/k8s.netresources.spidernet.io-claim_1e15705a-62fe-4694-8535-93a5f0ccf996.yaml
    ---
    cdiVersion: 0.6.0
    containerEdits: {}
    devices:
    - containerEdits:
        env:
        - DRA_CLAIM_UID=1e15705a-62fe-4694-8535-93a5f0ccf996
      name: 1e15705a-62fe-4694-8535-93a5f0ccf996
    kind: k8s.netresources.spidernet.io/claim 
    ```

    这里显示 ResourceClaim 已经被创建，并且 STATE 显示 allocated 和 reserverd，说明已经被 pod 使用。并且 spiderpool 已经为该 ResourceClaim 生成了对应的 CDI 文件。CDI 文件描述了需要挂载的文件和环境变量等。

    检查 Pod 是否 Running，并且验证 Pod 是否指定了环境变量 `DRA_CLAIM_UID`:

    ```shell
    ~# kubectl get po
    NAME                        READY   STATUS    RESTARTS      AGE
    nginx-745fb4c498-72g7g      1/1     Running   0             20m
    nginx-745fb4c498-s92qr      1/1     Running   0             20m
    ~# kubectl exec -it nginx-745fb4c498-72g7g sh
    ~# printenv DRA_CLAIM_UID
    1e15705a-62fe-4694-8535-93a5f0ccf996
    ```

    可以看到 Pod 的容器已经正确写入环境变量，说明 DRA 工作正常。

## 欢迎试用

目前 DRA 作为 Spiderpool 的 Alpha 功能，在未来我们会扩展更多能力，欢迎试用。如果您有更多问题或需求，请告诉我们。
