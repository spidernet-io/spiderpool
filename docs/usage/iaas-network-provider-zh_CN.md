# IaaS Network Provider

[**English**](./iaas-network-provider.md) | **简体中文**

## 简介

Spiderpool 支持对接通用的 IaaS Network Provider。当 Spiderpool 分配或释放 Pod IP 地址时，会调用配置的 Provider 在云平台侧完成对应 IaaS IP 资源的绑定或解绑，并使用云平台返回的 MAC 地址和 VLAN ID 配置 Pod 网卡。

该能力适用于公有云或私有云环境：Spiderpool 分配出的 IP 地址需要先在外部云网络系统中完成注册、绑定或转发面配置，Pod 才能正常使用。典型使用场景包括：

* 从云平台申请辅助 IP 资源（sub-ENI）。
* 将 IP 绑定到节点、ENI、辅助网卡或 VLAN 子接口。
* 向 Spiderpool 返回 Pod 网卡所需的 MAC 地址、VLAN ID 等云平台属性。
* 当 Spiderpool 释放 Pod IP 时，同步释放 IaaS 侧的 IP 绑定关系。

IaaS Network Provider 是一个 HTTP 服务。Spiderpool 只定义通用 API 契约，不依赖某个具体云厂商实现。带 IaaS 后端的 `SpiderIPPool` 支持两种放置模式：

* **节点级池**：通过 `spec.nodeName` 固定到单个节点，Provider 提前在该节点上预热 sub-ENI，Pod 启动无需同步调用云平台，速度快。
* **全局池**：不设置 `spec.nodeName`，一个池服务一个 Pod 分布在多个节点上的工作负载，实时分配并配合粘性 sub-ENI 缓存。

关于两种模式的设计细节、分配调用流程、超时模型和 Provider API 契约，请参考 [IaaS Network Provider 架构](../concepts/iaas-network-provider-zh_CN.md)。

本文使用的术语：

* ENI: 弹性网卡 (Elastic Network Interface)
* Sub-ENI: 辅助弹性网卡 (Secondary Elastic Network Interface)
* VLAN: 虚拟局域网 (Virtual Local Area Network)

## 前提条件

安装前需要准备：

1. **已部署的 IaaS Network Provider。** Provider 实现 [Spiderpool 的 Provider API 契约](../concepts/iaas-network-provider-zh_CN.md#api-契约)，并通过 Kubernetes Service 暴露。Provider 必须**先于** Spiderpool 安装，因为 Helm 在安装/升级时会 lookup Provider 的 TLS Secret 以快照其 CA 证书。

2. **IaaS 侧网络资源。** 平台管理员需要提前：

    * 创建 VPC 子网并绑定到节点的弹性网卡。例如将 VPC 子网 `172.91.0.0/24` 绑定到节点的物理网卡 `eth1`。
    * 确认每个节点可绑定的辅助 ENI 上限，用于配置下文的 Sub-ENI 调度容量。
    * 建议各节点的扩展弹性网卡不配置 IP 地址，避免回程路径不一致导致的通信问题。

3. **VLAN CNI。** Provider 模式使用 [vlan-cni](https://github.com/spidernet-io/vlan-cni)（Spiderpool 基于社区 cni-plugin 项目开发的 VLAN CNI 插件）为 Pod 创建 VLAN 子接口，并配置云平台分配的 VLAN ID 和 MAC 地址。它随 Spiderpool plugins 镜像发布，通过 `plugins.installVlanCNI` 安装。

4. **双栈规划（可选）。** 双栈 Pod 通过*配对池*支持：一个 IPv4 池和一个 IPv6 池通过 `ipam.spidernet.io/pair-pool` 注解相互引用，两个地址族在同一个 sub-ENI 上原子化配置。如需双栈，请连同 IPv4 子网一起规划 IPv6 子网。

## 安装配置 Spiderpool

### Helm values

创建 `iaas-values.yaml`：

```yaml
ipam:
  enableGatewayDetection: false
  enableIPConflictDetection: false

plugins:
  installVlanCNI: true

iaasNetworkProvider:
  enabled: true
  service:
    name: "iaas-network-provider"
    namespace: "iaas-network-provider-system"
    port: 8443
  tls:
    caSecret: "iaas-network-provider-tls"
  httpRequestTimeout: "50s"

spiderpoolController:
  podResourceInject:
    enabled: true

spiderpoolAgent:
  networkResourcePlugin:
    enabled: true
    kubeletRootDir: /var/lib/kubelet
    resourceAdvertisement:
      masterNIC:
        rules:
          - defaultMaxCount: 10000
            includeInterfaces:
              - "eth1"
      subENI:
        rules:
          - resourceName: spidernet.io/sub-eni
            defaultMaxCount: 256
```

配置项含义：

* `iaasNetworkProvider.enabled`：开启 IaaS Network Provider 集成（默认 `false`）。关闭时其余 `iaasNetworkProvider` 配置项均被忽略。
* `iaasNetworkProvider.service`：Provider 的 Kubernetes Service（name/namespace/port）。开启时 `service.name` 必须非空（默认 `iaas-network-provider`）。
* `iaasNetworkProvider.tls`：连接采用单向 TLS —— Spiderpool 校验 Provider 的服务端证书。安装/升级时 Helm 会 lookup Provider 的 TLS Secret（`service.namespace` 下的 `tls.caSecret`），只复制其中的 `ca.crt` 到本地 Secret `iaas-provider-ca`。GitOps 或 `helm template` 场景（lookup 不可用）需要显式设置 `tls.ca`（base64 PEM CA bundle），它优先于 lookup。`tls.insecureSkipVerify=true` 会跳过证书校验，仅作为灰度回退使用。如果 Provider 被卸载重装（生成新 CA），需要对 Spiderpool 重新执行 `helm upgrade` 刷新快照。
* `iaasNetworkProvider.httpRequestTimeout`：Spiderpool 等待单次 Provider 调用的最长时间。默认 `50s` 已覆盖 Provider 最坏情况；调整前请参考[超时模型](../concepts/iaas-network-provider-zh_CN.md#http-请求超时模型)。
* 必须开启 `plugins.installVlanCNI`：Provider 模式通过 VLAN CNI 配置 Pod 网卡。
* 必须关闭 `ipam.enableGatewayDetection` 和 `ipam.enableIPConflictDetection`。与传统先调用 CNI 后调用 IPAM 的方式不同，此模式必须先调用 IPAM 获取 IaaS 网络属性，再由 CNI 完成 Pod 网络配置，因此网关可达性检测和 IP 冲突检测在此模式下无法工作。
* `spiderpoolAgent.networkResourcePlugin` 开启 device-plugin 资源广告，供调度器做放置约束：
    * `subENI.rules[]` 广告 `spidernet.io/sub-eni` 资源，按节点声明辅助 ENI 容量（`defaultMaxCount` 应设置为节点实际的 sub-ENI 上限）。规则为空时关闭 Sub-ENI 广告。可选的 `nodeSelector`（支持 `matchLabels` 和 `matchExpressions`）限定广告该资源的节点。
    * `masterNIC.rules[]` 在拥有 `includeInterfaces`/`excludeInterfaces`（shell 风格 glob，如 `eth*`；排除优先）选中的物理网卡的节点上广告 `spidernet.io/<master>-nic` 资源，使工作负载只调度到实际存在 SpiderMultusConfig `master` 字段所指网卡的节点。`defaultMaxCount`（默认 `10000`）是虚拟容量，仅表示网卡存在。详见 [Spiderpool Device Plugin](./spiderpool-device-plugin.md)。
* `spiderpoolController.podResourceInject.enabled` 允许 webhook 自动为符合条件的 Pod 注入 `spidernet.io/<master>-nic` request。`spidernet.io/sub-eni` **不会**自动注入 —— 用户必须在 Pod 上显式声明，调度器才会做 ENI 容量约束。

### 安装

```bash
helm upgrade --install spiderpool spiderpool/spiderpool \
  --namespace kube-system \
  --values iaas-values.yaml \
  --wait
```

### 验证安装

确认功能已生效：

```bash
# 1. 仅在 enabled=true 时 ConfigMap 渲染非空的 provider service name
kubectl get configmap spiderpool-conf -n kube-system -o yaml | grep -A3 iaasNetworkProvider

# 2. agent 日志显示 IaaS client 初始化成功
kubectl logs -n kube-system -l app.kubernetes.io/component=spiderpool-agent | grep "IaaS"
```

预期 `iaasNetworkProvider.service.name` 非空，且 agent 日志包含 `IaaS provider configured and client created successfully`。如果看到 `IaaS provider configuration validation failed`，请检查 `iaasNetworkProvider.service` 和 `iaasNetworkProvider.tls` 配置。

确认节点已广告调度资源：

```bash
kubectl get nodes -o custom-columns='NAME:.metadata.name,SUB_ENI:.status.allocatable.spidernet\.io/sub-eni,MASTER_NIC:.status.allocatable.spidernet\.io/eth1-nic'
```

匹配的节点显示 `SUB_ENI=256` 和 `MASTER_NIC=10000`；不满足规则的节点显示 `<none>`。

## 创建 SpiderMultusConfig

Provider 模式使用 VLAN 类型的 SpiderMultusConfig。VLAN ID 由云平台动态分配，因此**不能设置 `vlanID`** —— vlan-cni 在 Pod 创建时通过 Unix socket 向本地 spiderpool-agent 查询从 IaaS 分配的 VLAN ID 和 MAC 地址，然后据此在 Pod 网络命名空间中创建 VLAN 子接口。

```yaml
apiVersion: spiderpool.spidernet.io/v2beta1
kind: SpiderMultusConfig
metadata:
  name: iaas-vlan-config
  namespace: spiderpool
spec:
  cniType: vlan
  vlan:
    master:
      - eth1
    ippools:
      ipv4:
        - pool-node1        # 引用下文创建的 SpiderIPPool
```

注意：

* `master` 为必填项，必须与目标节点上的物理网卡名一致，同时与 `masterNIC.rules[].includeInterfaces` 选中的网卡一致（本例为 `eth1`）。请保持候选节点上网卡命名一致，或依赖 [master 网卡调度](./spiderpool-device-plugin.md) 避免工作负载落到没有该网卡的节点。
* 绝不能在 `vlan` 配置中设置 `vlanID`；静态配置的 VLAN ID 会与云平台动态分配的不一致，导致 Pod 网络异常。

## 创建 SpiderIPPool

IaaS 池就是普通的 `SpiderIPPool` 加上 `ipam.spidernet.io/iaas-provider` 注解（取值为 Provider 名称）。`subnet` 必须与云平台的 VPC 子网一致。池的模式由形态推导：设置 `spec.nodeName` 为节点级池，不设置为全局池。模式在池的生命周期内固定 —— webhook 会拒绝事后增删 `spec.nodeName`。

### 节点级池

节点级池固定到单个节点（webhook 拒绝多个 `nodeName`），且**必须**带 `ipam.spidernet.io/parent-nic` 注解，指定该节点上池的单个 guest-OS 父网卡名：

```yaml
apiVersion: spiderpool.spidernet.io/v2beta1
kind: SpiderIPPool
metadata:
  name: pool-node1
  annotations:
    ipam.spidernet.io/iaas-provider: huaweicloud
    ipam.spidernet.io/parent-nic: eth1
spec:
  ipVersion: 4
  subnet: 172.91.0.0/24
  gateway: 172.91.0.1
  ips:
    - 172.91.0.100-172.91.0.120
  nodeName:
    - node1
```

创建后需要关注：

1. **标记 label 已同步**：mutating webhook 会将 `iaas-provider` 注解镜像为同名 label。

    ```bash
    kubectl get spiderippool pool-node1 -o jsonpath='{.metadata.labels}'
    ```

2. **父网卡已发布**：池所在节点上的 spiderpool-agent 会本地解析注解指定网卡的 MAC 地址，并发布到 `status.parentNic`，Provider 由此读取云侧父端口 MAC。

    ```bash
    kubectl get spiderippool pool-node1 -o jsonpath='{.status.parentNic}'
    # {"mac":"fa:16:3e:11:22:33","name":"eth1"}
    ```

3. **预热已完成**：节点级池是严格 prewarm-only 的 —— Spiderpool 只分配 Provider 已准备好并写入 `status.ipMetaData` 的地址。请等待 Provider 回写 metadata 条目后再创建 Pod；否则 Pod 创建会以 IP 用尽错误失败，kubelet 会持续重试，直到预热地址出现后自动恢复。

    ```bash
    kubectl get spiderippool pool-node1 -o jsonpath='{.status.ipMetaData}'
    ```

### 全局池

全局池**不**设置 `spec.nodeName`，服务一个 Pod 分布在多个节点上的工作负载。`parent-nic` 注解为可选：带注解时，分配路径在 Pod 所在节点上直接按名字解析父网卡 MAC；不带时回退到从 SpiderMultusConfig 的 `master` 接口解析。

```yaml
apiVersion: spiderpool.spidernet.io/v2beta1
kind: SpiderIPPool
metadata:
  name: pool-global
  annotations:
    ipam.spidernet.io/iaas-provider: huaweicloud
    ipam.spidernet.io/parent-nic: eth1   # 可选；网卡须在所有节点上同名存在
spec:
  ipVersion: 4
  subnet: 172.91.0.0/24
  gateway: 172.91.0.1
  ips:
    - 172.91.0.121-172.91.0.180
  podAffinity:
    matchLabels:
      app: my-app
```

全局池**无需预热** —— 每个节点上的第一个 Pod 会触发一次同步 Provider 调用创建 sub-ENI，之后该节点上的 Pod 直接复用缓存的 sub-ENI，无需再调用云平台。建议配置 `podAffinity` 使池专属于一个工作负载。创建后同样检查标记 label；全局池上永远不会出现 `status.parentNic`。

### 双栈配对池（可选）

双栈场景需要创建一对通过 `ipam.spidernet.io/pair-pool` 注解相互引用的 v4/v6 池，使一个 sub-ENI 原子化承载两个地址族：

```yaml
apiVersion: spiderpool.spidernet.io/v2beta1
kind: SpiderIPPool
metadata:
  name: pool-node1-v4
  annotations:
    ipam.spidernet.io/iaas-provider: huaweicloud
    ipam.spidernet.io/pair-pool: pool-node1-v6
    ipam.spidernet.io/parent-nic: eth1
spec:
  ipVersion: 4
  subnet: 172.91.0.0/24
  ips: ["172.91.0.100-172.91.0.120"]
  nodeName: ["node1"]
---
apiVersion: spiderpool.spidernet.io/v2beta1
kind: SpiderIPPool
metadata:
  name: pool-node1-v6
  annotations:
    ipam.spidernet.io/iaas-provider: huaweicloud
    ipam.spidernet.io/pair-pool: pool-node1-v4
    ipam.spidernet.io/parent-nic: eth1
spec:
  ipVersion: 6
  subnet: fd00:172:91::/112
  ips: ["fd00:172:91::100-fd00:172:91::120"]
  nodeName: ["node1"]
---
```

webhook 会校验配对关系（相互引用、形态一致、v6 容量覆盖 v4）。配对池的 `status.parentNic` 和预热 metadata 都只存在于主池（v4）上。

## 创建 Pod

### 使用节点级池的 Pod

Pod 通过 Multus 注解引用 VLAN SpiderMultusConfig，并声明一个 `spidernet.io/sub-eni` request，使调度器做辅助 ENI 容量约束。开启 `podResourceInject` 后 webhook 会自动注入 `spidernet.io/eth1-nic`：

```yaml
apiVersion: v1
kind: Pod
metadata:
  name: iaas-demo
  annotations:
    k8s.v1.cni.cncf.io/networks: spiderpool/iaas-vlan-config
    ipam.spidernet.io/ippool: '{"ipv4": ["pool-node1"]}'
spec:
  containers:
    - name: demo
      image: busybox:1.36
      command: ["sh", "-c", "sleep 3600"]
      resources:
        requests:
          spidernet.io/sub-eni: "1"
        limits:
          spidernet.io/sub-eni: "1"
```

创建的同时观察调度事件：

```bash
kubectl get events \
  --field-selector involvedObject.kind=Pod,involvedObject.name=iaas-demo \
  --sort-by=.metadata.creationTimestamp --watch
```

容量充足时事件显示 `Scheduled`；若 `sub-eni` request 总量超过节点容量，多余的 Pod 保持 `Pending`，事件显示 `FailedScheduling` 和 `Insufficient spidernet.io/sub-eni`。

### 使用全局池的 Deployment

跨节点多副本的工作负载适合全局池，Pod label 需匹配池的 `podAffinity`：

```yaml
apiVersion: apps/v1
kind: Deployment
metadata:
  name: my-app
spec:
  replicas: 3
  selector:
    matchLabels:
      app: my-app
  template:
    metadata:
      labels:
        app: my-app
      annotations:
        k8s.v1.cni.cncf.io/networks: spiderpool/iaas-vlan-config
        ipam.spidernet.io/ippool: '{"ipv4": ["pool-global"]}'
    spec:
      containers:
        - name: app
          image: busybox:1.36
          command: ["sh", "-c", "sleep 3600"]
          resources:
            requests:
              spidernet.io/sub-eni: "1"
            limits:
              spidernet.io/sub-eni: "1"
```

每个节点上的第一个副本触发一次同步 Provider 调用；后续副本（以及先前 Pod 删除后新调度来的 Pod）直接复用该节点缓存的 sub-ENI，无需任何云平台调用即可启动。

## 验证结果

Pod Running 后：

```bash
# Pod 从池中拿到 IP，网卡配置了云平台分配的 MAC 和 VLAN
kubectl get pod iaas-demo -o wide
kubectl exec iaas-demo -- ip addr show

# 已声明和已注入的调度资源
kubectl get pod iaas-demo -o jsonpath='{.spec.containers[0].resources.requests}'

# 池中记录了分配
kubectl get spiderippool pool-node1 -o jsonpath='{.status.allocatedIPCount}'

# SpiderEndpoint 记录了 Pod 的分配详情
kubectl get spiderendpoint iaas-demo -o yaml
```

## 排障

* **功能未生效**：确认 `iaasNetworkProvider.enabled=true` 且 `iaasNetworkProvider.service.name` 非空；检查 agent 日志中是否有 `IaaS provider configuration validation failed`。
* **Pod Pending，`Insufficient spidernet.io/sub-eni`（或 `<master>-nic`）**：确认 `subENI.rules`/`masterNIC.rules` 非空，检查 `defaultMaxCount`、`nodeSelector`、`includeInterfaces`/`excludeInterfaces`，并在节点上执行 `ip link show` 确认物理网卡存在。若 Pod 缺少注入的 `<master>-nic` request，检查 `podResourceInject.enabled`；`sub-eni` request 必须由用户显式声明。
* **节点级池 Pod 报 IP 用尽**：池尚未完成预热（或预热条目已全部占用）—— 检查 `status.ipMetaData` 和 Provider 日志。metadata 条目出现后 Pod 创建会自动恢复。
* **Provider 调用超时**：参考[超时模型与错误信息](../concepts/iaas-network-provider-zh_CN.md#http-请求超时模型)。
* **分配/释放语义**（同步分配、幂等释放、父网卡 MAC 解析）：参考[特殊场景处理](../concepts/iaas-network-provider-zh_CN.md#特殊场景处理)。
