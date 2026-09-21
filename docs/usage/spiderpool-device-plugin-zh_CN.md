# Spiderpool Device Plugin

[**English**](./spiderpool-device-plugin.md) | **简体中文**

## 背景

在 Spiderpool 的二级网络中，SpiderMultusConfig 通过 `master` 指定 Pod 网络需要绑定的宿主机物理网卡，例如 `eth1`、`ens5` 或某张 VLAN 子接口。这个约束不是简单的配置项，它决定了 Pod 能否在目标节点上完成二级网络创建。

Kubernetes 默认调度主要依据 CPU、内存等通用资源做决策，并不知道某个 Pod 依赖哪张宿主机网卡，也不知道某个节点还剩多少可用于二级网络的云网卡容量。因此，Pod 可能先被调度到节点上，再在后续 CNI 或云资源分配阶段失败。

这类失败通常集中在两种场景：

* 不同节点的物理网卡命名或布局不同，例如部分节点存在 `eth1`，其它节点不存在。
* 在公有云环境中，每个节点可绑定的 ENI（Elastic Network Interface，弹性网卡）及辅助 ENI 数量通常受实例规格和云平台配额限制。如果 Pod 被调度到剩余 ENI 容量不足的节点，Pod 不仅会因无法获得网络资源而启动失败，还会产生无效的云平台 API 调用。部分云平台会限制 API 调用频率，这些无效请求可能消耗限流配额、延长后续正常请求的等待时间，并扩大批量创建 Pod 时的失败影响。

Spiderpool Device Plugin 运行在 `spiderpool-agent` 中，通过 Kubernetes device plugin API 将上述网络约束注册为 extended resource。Pod 请求这些资源后，Kubernetes scheduler 会在创建 Pod 网络之前先过滤不满足条件的节点。Device Plugin 只负责调度和 kubelet admission 阶段的约束，不负责配置 Pod 网卡，也不负责分配或释放云平台资源。

## 功能

### 基于 master 网卡名称调度

Spiderpool 可以发现节点上的物理网卡，并为每张选中的网卡通告到 Node.status.allocatable 中：

```text
status:
  allocatable:
    spidernet.io/<master>-nic: 10000
```

例如，节点存在 `eth1` 时会广告：

```text
status:
  allocatable:
    spidernet.io/eth1-nic: 10000
```

Pod 请求 `spidernet.io/eth1-nic: 1` 后，只能调度到存在 `eth1` 且广告了该资源的节点。默认数量 `10000` 来自 `masterNIC.rules[].defaultMaxCount`，是表示网卡存在的虚拟容量，不代表带宽、队列数量或可创建的 Pod 上限。

该能力不依赖 IaaS Network Provider，可用于 Macvlan、IPvlan、VLAN 等要求目标节点必须存在指定 `master` 网卡的网络。

如何配置：

```yaml
spiderpoolAgent:
  networkResourcePlugin:
    enabled: true
    resourceAdvertisement:
      masterNIC:
        rules:
          - nodeSelector:
              matchLabels:
                kubernetes.io/os: linux
            defaultMaxCount: 10000
            includeInterfaces:
              - "eth1"
              - "ens*"
            excludeInterfaces:
              - "ens10"
spiderpoolController:
  podResourceInject:
    enabled: true
```

配置项含义：

* `nodeSelector`：选择规则适用节点的 Kubernetes label selector。空 selector 匹配所有节点。可使用 `matchLabels` 和 `matchExpressions` 中的 `In`、`NotIn`、`Exists`、`DoesNotExist` 等 operator 表达包含或排除条件。
* `defaultMaxCount`：每个被选中 master 网卡广告的虚拟总容量，默认值为 `10000`。
* `includeInterfaces`：使用 shell 风格 glob 表达式选择网卡，例如 `eth*`、`ens[0-9]`。
* `excludeInterfaces`：排除同一规则内已选择的网卡，优先级高于 `includeInterfaces`。
* `masterNIC.rules[]`：配置至少一条规则时启用 master 网卡名称资源广告；规则为空时关闭该广告。
* `networkResourcePlugin.enabled`：启用整个 Device Plugin 功能；关闭后不会广告任何网络相关资源。
* 当 `masterNIC.rules` 为空时，Spiderpool 不会广告 master 网卡资源。规则未配置 `includeInterfaces` 时，会选择匹配节点上发现的所有物理 master 网卡。

* `spiderpoolController.podResourceInject.enabled`：启用 Pod webhook，使其读取 Pod 引用的 SpiderMultusConfig，并注入对应的 master NIC resource request。

启用资源注入后，webhook 会检查 Pod 通过 `v1.multus-cni.io/default-network` 和 `k8s.v1.cni.cncf.io/networks` 引用的 SpiderMultusConfig。对于不由 SpiderMultusConfig 管理的普通 NetworkAttachmentDefinition，webhook 会忽略对应引用。对于 Macvlan、IPvlan、VLAN、eni-vlan 和 IPoIB 配置，webhook 会在第一个容器的 resource requests 和 limits 中注入 `spidernet.io/<master>-nic: 1`。重复的 master 网卡只注入一次。如果配置使用多个 master 网卡创建 bond，webhook 会为每个 bond 成员分别注入资源，从而要求目标节点同时具备所有成员网卡。

如果工作负载已经声明某个 master NIC 资源，webhook 会保留用户设置的值。

### 基于 Sub-ENI 数量调度

在启用 IaasNetworkProvider 模式下，Spiderpool 可以将节点可用的辅助 Sub-ENI 总数广告到 Node.status.allocatable 中：

```text
status:
  allocatable:
    spidernet.io/sub-eni: 10
```

`defaultMaxCount` 定义每个启用该功能的 agent 广告的节点总容量。

当 Pod 请求的 `spidernet.io/sub-eni` 超过节点剩余可调度容量时，scheduler 不会继续向该节点调度 Pod。

`Node.status.allocatable["spidernet.io/sub-eni"]` 是 kubelet 广告的健康总容量，不是剩余数量。剩余容量由 Kubernetes Scheduler 组件根据总容量和已调度 Pod 的 resource request 计算。

如何配置：

```yaml
iaasNetworkProvider:
  enabled: true
  service:
    name: "iaas-network-provider"
    namespace: "iaas-network-provider-system"
    port: 443
  tls:
    caSecret: "iaas-network-provider-tls"

spiderpoolAgent:
  networkResourcePlugin:
    enabled: true
    resourceAdvertisement:
      subENI:
        rules:
          - resourceName: spidernet.io/sub-eni
            defaultMaxCount: 10
            nodeSelector:
              matchLabels:
                key: value

spiderpoolController:
  podResourceInject:
    enabled: true
```

配置项含义：

* `iaasNetworkProvider.enabled` 和 `iaasNetworkProvider.service`：启用 IaaS Network Provider 模式，并指向 Provider 的 Kubernetes Service（name/namespace/port）；未启用 Provider 模式时，Sub-ENI 调度不会生效。
* `subENI.rules[]`：Sub-ENI 资源广告规则数组；规则为空时关闭 Sub-ENI 广告。每条规则对应一个 extended resource。
* `subENI.rules[].resourceName`：广告给 Kubernetes 的 extended resource 名称，缺省为 `spidernet.io/sub-eni`，需满足 `<domain>/<resource>` 命名规则。不同规则可以广告不同的资源名，例如按池模式各定义一个。
* `subENI.rules[].defaultMaxCount`：规则匹配节点上广告的可调度 Sub-ENI 总容量。规划时需与实例规格 / 父网卡的 Sub-ENI 上限以及实际云配额保持一致。该值是静态的，不会随池的 `readyIPCount` 自动变化，也不是从云平台实时查询的剩余数。
* `subENI.rules[].nodeSelector`：可选的 Kubernetes label selector；设置后仅匹配的节点会广告该 Sub-ENI 资源。支持 `matchLabels` 和 `matchExpressions`。
* `spiderpoolController.podResourceInject.enabled`：启用后，webhook 才会为符合条件的 Pod 自动注入 `spidernet.io/<master>-nic` request。Sub-ENI 资源不会被自动注入，因为准入阶段无法可靠判定 Pod 最终使用节点池还是全局池。

规则匹配语义：

* 同名 `resourceName` 配置多条规则时，节点按规则顺序取第一条匹配的规则生效——可用于给不同节点组设置不同容量（例如大规格实例 16、小规格实例 4）。
* 节点同时匹配不同 `resourceName` 的规则时，会同时广告这些资源。调度器对每个资源名独立计账，不同资源名之间没有共享额度的联动机制。

需要 Sub-ENI 容量调度的 Pod 必须在容器 resources 中显式声明对应的资源请求。extended resource 要求 `requests` 与 `limits` 相等且为整数；单张二级网卡的 Pod 通常声明 `1`，有多张二级网卡时按实际占用的 Sub-ENI 数量声明。未声明该资源的 Pod 不受 Sub-ENI 容量约束，调度器不会为其预留容量——请确保集群内使用 Sub-ENI 的工作负载全部声明，否则容量核算会失真。Provider 模式的完整配置请参考 [IaaS Network Provider](./iaas-network-provider-zh_CN.md)。

#### 池模式与节点规划

IaaS Network Provider 支持两种池放置模式，二者消耗的都是宿主机同一份物理 Sub-ENI 槽位：

* **节点级池**：通过 `spec.nodeName` 固定到单个节点，Provider 提前预热 Sub-ENI，Pod 启动快。适合规模相对稳定、对 Pod 启动速度要求较高的业务。
* **全局池**：不设置 `spec.nodeName`，Sub-ENI 按需创建并通过粘性缓存复用。适合副本数波动较大、需要弹性伸缩的业务。

可以分别指定不同节点用于两种模式，也可以让同一节点同时承载两种模式。两种池在同一主机上使用时，共享该主机的 Sub-ENI 总额度，广告的容量需要在主机总容量内统一规划。

推荐做法是按模式划分专用节点组，并为每组广告独立的资源名。先为节点打标签：

```bash
# 节点级池节点
kubectl label node node1 spiderpool.io/node-pool=true --overwrite
# 全局池节点
kubectl label node node2 spiderpool.io/global-pool=true --overwrite
```

再为每种模式配置一条规则：

```yaml
spiderpoolAgent:
  networkResourcePlugin:
    enabled: true
    resourceAdvertisement:
      subENI:
        rules:
          - resourceName: spidernet.io/node-pool-sub-eni
            defaultMaxCount: 6
            nodeSelector:
              matchLabels:
                spiderpool.io/node-pool: "true"
          - resourceName: spidernet.io/global-pool-sub-eni
            defaultMaxCount: 4
            nodeSelector:
              matchLabels:
                spiderpool.io/global-pool: "true"
```

使用节点级池的工作负载声明 `spidernet.io/node-pool-sub-eni`，使用全局池的声明 `spidernet.io/global-pool-sub-eni`。专用节点组能把两份容量预算彻底隔离：节点池的预热消耗不会挤占全局池节点的按需余量。

节点标签只影响资源上报和调度，不决定池模式：IPPool 的 `spec.nodeName` 非空才是节点级预热池；全局池必须不配置该字段。

当某个节点必须同时承载两种模式时，可选择两种记账方式之一：

* **静态切分（两个资源名）**：给节点同时打上两个标签使其匹配两条规则，同时广告两个资源。由于不同资源名独立计账，需要把主机总额度静态切分到两份容量上——例如主机上限 10 切分为 `node-pool-sub-eni: 6` 加 `global-pool-sub-eni: 4`。两份广告容量之和绝不能超过主机的物理 Sub-ENI 上限。这种方式隔离清晰，但额度无法在两种模式间流动。
* **共享额度（一个资源名）**：只广告一个资源（如 `spidernet.io/sub-eni`），`defaultMaxCount` 设为主机总额度，两种模式的工作负载都声明同一资源名。额度可以按需在两种模式间弹性流动。注意预热发生在 Pod 创建之前：预热的 Sub-ENI 会先占用真实槽位，而调度器只统计运行中 Pod 的 request，因此节点池未跑满时调度视图偏乐观。请让预热数量尽量接近实际并发 Pod 数以缩小偏差。

## 快速开始

以下步骤仅验证 master 网卡名称调度。Sub-ENI 数量调度的快速开始请参考 [IaaS Network Provider](./iaas-network-provider-zh_CN.md)。

### 1. 准备 Helm values

创建 `device-plugin-values.yaml`：

```yaml
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

spiderpoolController:
  podResourceInject:
    enabled: true
```

注意：

* `kubeletRootDir` 必须与节点上的 kubelet 根目录一致。
* `masterNIC.rules` 中的 `eth1` 必须替换为需要调度的实际物理网卡。仅当广告的虚拟容量需要不同于 `10000` 时，才需要调整 `defaultMaxCount`。可通过规则的 `nodeSelector` 限制广告资源的节点范围；空 selector 匹配所有节点。
* `podResourceInject.enabled` 用于根据 Pod 引用的 SpiderMultusConfig 自动注入 master NIC 资源。

### 2. 安装或更新 Spiderpool

首次安装：

```bash
helm repo add spiderpool https://spidernet-io.github.io/spiderpool
helm repo update
helm upgrade --install spiderpool spiderpool/spiderpool \
  --namespace kube-system \
  --reuse-values \
  --values device-plugin-values.yaml \
  --wait
```

如果 release 或安装 namespace 不同，请修改命令中的 `spiderpool` 和 `kube-system`。

### 3. 检查安装成功

确认 spiderpool-agent 正常运行：

```bash
kubectl get pod -n kube-system -l app.kubernetes.io/component=spiderpool-agent -o wide
```

检查各节点广告的 Spiderpool 网络资源：

```bash
kubectl get nodes -o json | jq '[.items[] | {name: .metadata.name, allocatable: .status.allocatable}]'
```

预期结果：

```
[
  {
    "name": "spiderpool0522022016-control-plane",
    "allocatable": {
      "cpu": "56",
      "ephemeral-storage": "860377048Ki",
      "hugepages-1Gi": "0",
      "hugepages-2Mi": "0",
      "memory": "131885828Ki",
      "pods": "110",
      "spidernet.io/eth1-nic": "10k",
      "spidernet.io/eth2-nic": "10k"
    }
  },
  {
    "name": "spiderpool0522022016-worker",
    "allocatable": {
      "cpu": "56",
      "ephemeral-storage": "860377048Ki",
      "hugepages-1Gi": "0",
      "hugepages-2Mi": "0",
      "memory": "131885828Ki",
      "pods": "110",
      "spidernet.io/eth1-nic": "10k"
    }
  }
]
```

如果资源未出现，检查 agent 注册日志：

```bash
kubectl logs -n kube-system \
  -l app.kubernetes.io/component=spiderpool-agent \
  --tail=200 | grep "network resource plugin"
```

### 4. 验证 master 网卡名称调度

创建 `master` 为 `eth1` 的 SpiderMultusConfig：

```yaml
apiVersion: spiderpool.spidernet.io/v2beta1
kind: SpiderMultusConfig
metadata:
  name: master-nic-network
  namespace: default
spec:
  cniType: macvlan
  disableIPAM: true
  macvlan:
    master:
      - eth1
```

```bash
kubectl apply -f master-nic-network.yaml
```

创建引用该网络的 Pod。Pod 不需要声明 `spidernet.io/eth1-nic`，webhook 会自动注入：

```yaml
apiVersion: v1
kind: Pod
metadata:
  name: master-nic-scheduling
  annotations:
    k8s.v1.cni.cncf.io/networks: default/master-nic-network
spec:
  containers:
    - name: test
      image: busybox:1.36
      command: ["sh", "-c", "sleep 3600"]
```

```bash
kubectl apply -f master-nic-pod.yaml
```

持续观察带时间戳的 Pod Events：

```bash
kubectl get events \
  --field-selector involvedObject.kind=Pod,involvedObject.name=master-nic-scheduling \
  --sort-by=.metadata.creationTimestamp \
  -o custom-columns='TIME:.metadata.creationTimestamp,TYPE:.type,REASON:.reason,MESSAGE:.message' \
  --watch
```

调度成功时会看到 `Scheduled` 事件。检查 Pod 所在节点：

```bash
kubectl get pod master-nic-scheduling -o wide
```

检查 webhook 注入的 request，并确认该节点确实广告了 `eth1` 资源：

```bash
kubectl get pod master-nic-scheduling \
  -o jsonpath='{.spec.containers[0].resources.requests.spidernet\.io/eth1-nic}{"\n"}'

NODE_NAME=$(kubectl get pod master-nic-scheduling -o jsonpath='{.spec.nodeName}')
kubectl get node "${NODE_NAME}" \
  -o jsonpath='{.status.allocatable.spidernet\.io/eth1-nic}{"\n"}'
```

Pod request 的预期输出为 `1`，节点容量的预期输出为 `10000`。

如果没有节点广告该资源，Pod 会保持 `Pending`，Events 中会出现 `FailedScheduling` 和 `Insufficient spidernet.io/eth1-nic`。

## 排障

### 节点没有广告任何资源

检查以下配置和状态：

```bash
helm get values spiderpool -n kube-system
kubectl get daemonset spiderpool-agent -n kube-system
kubectl logs -n kube-system -l app.kubernetes.io/component=spiderpool-agent --tail=200
```

* 确认 `networkResourcePlugin.enabled=true`。
* 确认 `kubeletRootDir` 与节点实际配置一致。
* 确认 agent 挂载了 `{kubeletRootDir}/device-plugins` 和 `{kubeletRootDir}/plugins_registry`。
* kubelet 或 spiderpool-agent 重启后，资源可能短暂消失，待 Device Plugin 重新注册后会恢复；期间新 Pod 不可调度到该节点。

### Sub-ENI 资源缺失

* Sub-ENI 资源仅在启用 IaaS Network Provider 模式（`iaasNetworkProvider.enabled=true` 且 Service 配置有效）时上报。
* 确认 `subENI.rules` 非空，且节点标签匹配规则的 `nodeSelector`。
* 同名 `resourceName` 配置多条规则时，节点只取第一条匹配的规则；容量不符合预期时检查规则顺序。

### master NIC 资源缺失

* 在目标节点执行 `ip link show`，确认网卡名称存在。
* 检查 `masterNIC.rules`、`nodeSelector`、`includeInterfaces` 和 `excludeInterfaces`。
* 注意虚拟网卡和常见 CNI 网卡不会作为物理 master NIC 自动广告。

### Pod 一直处于 Pending

```bash
kubectl describe pod <pod-name>
kubectl get events \
  --field-selector involvedObject.kind=Pod,involvedObject.name=<pod-name> \
  --sort-by=.metadata.creationTimestamp
```

* `Insufficient spidernet.io/<master>-nic`：没有候选节点提供指定 master 网卡资源。
* `Insufficient spidernet.io/...sub-eni`：所有候选节点上已调度 Pod 的 requests 之和已达 `defaultMaxCount`，或工作负载声明的资源名与规则中的 `resourceName` 不一致。可通过 `kubectl describe node <node>` 查看 `Allocated resources`。在调度阶段被拦截正是该功能的目的——不会产生任何无效的云 API 调用。
* Pod 中没有 `spidernet.io/<master>-nic`：确认 `podResourceInject.enabled=true`、`networkResourcePlugin.enabled=true` 且 `masterNIC.rules` 非空；确认 Pod 引用了 master 非空的 Macvlan、IPvlan、VLAN、eni-vlan 或 IPoIB SpiderMultusConfig。
* 如果网络 annotation 中的 SpiderMultusConfig namespace 或名称错误，该引用会被当作普通 NetworkAttachmentDefinition 处理，不会注入对应的 master NIC 资源。
