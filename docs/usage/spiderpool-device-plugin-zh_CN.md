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
    devicePluginAffinity:
      nodeSelector:
        matchExpressions:
          - key: node-role.kubernetes.io/control-plane
            operator: DoesNotExist
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

* `devicePluginAffinity.nodeSelector`：选择会广告 Spiderpool 网络资源的节点。空 selector 匹配所有节点。可使用 `matchLabels` 和 `matchExpressions` 中的 `In`、`NotIn`、`Exists`、`DoesNotExist` 等 operator 表达包含或排除条件。
* `nodeSelector`：选择规则适用节点的 Kubernetes label selector。空 selector 匹配所有节点。可使用 `matchLabels` 和 `matchExpressions` 中的 `In`、`NotIn`、`Exists`、`DoesNotExist` 等 operator 表达包含或排除条件。
* `defaultMaxCount`：每个被选中 master 网卡广告的虚拟总容量，默认值为 `10000`。
* `includeInterfaces`：使用 shell 风格 glob 表达式选择网卡，例如 `eth*`、`ens[0-9]`。
* `excludeInterfaces`：排除同一规则内已选择的网卡，优先级高于 `includeInterfaces`。
* `masterNIC.rules[]`：配置至少一条规则时启用 master 网卡名称资源广告；规则为空时关闭该广告。
* `networkResourcePlugin.enabled`：启用整个 Device Plugin 功能；关闭后不会广告任何网络相关资源。
* 当 `masterNIC.rules` 为空时，Spiderpool 不会广告 master 网卡资源。规则未配置 `includeInterfaces` 时，会选择匹配节点上发现的所有物理 master 网卡。

* `spiderpoolController.podResourceInject.enabled`：启用 Pod webhook，使其读取 Pod 引用的 SpiderMultusConfig，并注入对应的 master NIC resource request。

启用资源注入后，webhook 会检查 Pod 通过 `v1.multus-cni.io/default-network` 和 `k8s.v1.cni.cncf.io/networks` 引用的 SpiderMultusConfig。对于不由 SpiderMultusConfig 管理的普通 NetworkAttachmentDefinition，webhook 会忽略对应引用。对于 Macvlan、IPvlan、VLAN 和 IPoIB 配置，webhook 会在第一个容器的 resource requests 和 limits 中注入 `spidernet.io/<master>-nic: 1`。重复的 master 网卡只注入一次。如果配置使用多个 master 网卡创建 bond，webhook 会为每个 bond 成员分别注入资源，从而要求目标节点同时具备所有成员网卡。

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
  serverUrl: "http://iaas-network-provider.example.svc:80"

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

* `iaasNetworkProvider.serverUrl`：IaaS Network Provider 的服务地址；未启用 Provider 模式时，Sub-ENI 调度不会生效。
* `subENI.rules[]`：Sub-ENI 资源广告规则数组；规则为空时关闭 Sub-ENI 广告。
* `subENI.rules[].resourceName`：广告给 Kubernetes 的 extended resource 名称，通常保持默认值 `spidernet.io/sub-eni`。
* `subENI.rules[].defaultMaxCount`：节点默认可调度的辅助 ENI 总容量。
* `subENI.rules[].nodeSelector`：可选的 Kubernetes label selector；设置后仅匹配的节点会广告该 Sub-ENI 资源。支持 `matchLabels` 和 `matchExpressions`。
* `spiderpoolController.podResourceInject.enabled`：启用后，webhook 才会为符合条件的 Pod 自动注入 `spidernet.io/sub-eni` request。

当 `spiderpoolController.podResourceInject.enabled=true` 时，webhook 会为符合以下条件的 Pod 自动注入 `spidernet.io/sub-eni`：

* 已启用 IaaS Network Provider。
* Pod 引用了未设置 `vlanID` 的 VLAN SpiderMultusConfig。
* Pod 尚未声明同名资源。

注入数量等于 Pod 引用的合格 VLAN SpiderMultusConfig 数量。Provider 模式的完整配置请参考 [IaaS Network Provider](./iaas-network-provider-zh_CN.md)。

## 快速开始

以下步骤仅验证 master 网卡名称调度。Sub-ENI 数量调度的快速开始请参考 [IaaS Network Provider](./iaas-network-provider-zh_CN.md)。

### 1. 准备 Helm values

创建 `device-plugin-values.yaml`：

```yaml
spiderpoolAgent:
  networkResourcePlugin:
    enabled: true
    kubeletRootDir: /var/lib/kubelet
    devicePluginAffinity:
      nodeSelector: {}
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
* `devicePluginAffinity.nodeSelector` 控制哪些节点会广告 Device Plugin 资源。留空表示匹配所有节点，也可使用 `matchExpressions` 排除节点。
* `masterNIC.rules` 中的 `eth1` 必须替换为需要调度的实际物理网卡。仅当广告的虚拟容量需要不同于 `10000` 时，才需要调整 `defaultMaxCount`。
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
      "spidernet.io/eth2-nic": "10k",
      "spidernet.io/sub-eni": "0"
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
      "spidernet.io/eth1-nic": "10k",
      "spidernet.io/sub-eni": "0"
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
* kubelet 或 spiderpool-agent 重启后，资源可能短暂消失，待 Device Plugin 重新注册后会恢复。

### master NIC 资源缺失

* 在目标节点执行 `ip link show`，确认网卡名称存在。
* 检查 `masterNIC.rules`、`nodeSelector`、`includeInterfaces` 和 `excludeInterfaces`。
* 检查节点是否匹配 `devicePluginAffinity.nodeSelector`。
* 注意虚拟网卡和常见 CNI 网卡不会作为物理 master NIC 自动广告。

### Pod 一直处于 Pending

```bash
kubectl describe pod <pod-name>
kubectl get events \
  --field-selector involvedObject.kind=Pod,involvedObject.name=<pod-name> \
  --sort-by=.metadata.creationTimestamp
```

* `Insufficient spidernet.io/<master>-nic`：没有候选节点提供指定 master 网卡资源。
* Pod 中没有 `spidernet.io/<master>-nic`：确认 `podResourceInject.enabled=true`、`networkResourcePlugin.enabled=true` 且 `masterNIC.rules` 非空；确认 Pod 引用了 master 非空的 Macvlan、IPvlan、VLAN 或 IPoIB SpiderMultusConfig。
* 如果网络 annotation 中的 SpiderMultusConfig namespace 或名称错误，该引用会被当作普通 NetworkAttachmentDefinition 处理，不会注入对应的 master NIC 资源。
