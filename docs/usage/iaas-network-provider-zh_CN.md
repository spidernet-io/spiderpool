# IaaS Network Provider

[**English**](./iaas-network-provider.md) | **简体中文**

## 概述

Spiderpool 支持对接通用的 IaaS Network Provider。当 Spiderpool 分配或释放 Pod IP 地址时，可以调用配置的 Provider，在云平台侧完成对应 IaaS IP 资源的绑定或解绑。

该能力适用于公有云或私有云环境。在这些环境中，Spiderpool 分配出的 IP 地址可能还需要在外部云网络系统中完成注册、绑定或转发面配置后，Pod 才能正常使用。

典型使用场景包括：

- 从云平台申请辅助 IP 资源。
- 将 IP 绑定到节点、ENI、辅助网卡、VLAN 子接口或其它云网络资源。
- 向 Spiderpool 返回 Pod 网卡所需的 MAC 地址、VLAN ID 等云平台属性。
- 当 Spiderpool 释放 Pod IP 时，同步释放 IaaS 侧的 IP 绑定关系。

## 工作原理

启用该能力后，Spiderpool 会执行以下流程：

1. Pod IP 分配阶段，Spiderpool 先从 Spiderpool IP 池中分配 IP，然后调用 IaaS Network Provider 的分配接口。
2. IaaS Network Provider 在云平台侧完成 IP 绑定，并返回云平台侧的网络属性。
3. Spiderpool 将返回的 MAC 地址和 VLAN ID 写入分配结果，后续 VLAN CNI 流程使用这些信息配置 Pod 网卡。
4. Pod IP 释放阶段，Spiderpool 会针对每个需要释放的 IPv4 地址调用 IaaS Network Provider 的释放接口。
5. IaaS 释放接口调用成功后，Spiderpool 再从内部 IP 池中释放该 IP。这里的“调用成功”代表 IaaS Network Provider 已成功接收释放请求并开始云平台侧清理，并不保证云平台侧 IP 资源已经彻底释放完成（云平台可能因限速或异步机制仍在处理）。

IaaS Network Provider 是一个 HTTP 服务。Spiderpool 只定义通用 API 契约，不依赖某个具体云厂商实现。

## 使用方式

通过 Helm values 配置 Provider URL 和 HTTP 超时：

```yaml
ipam:
  enableGatewayDetection: false
  enableIPConflictDetection: false
plugins:
  installVlanCNI: true
iaasNetworkProvider:
  serverUrl: "http://iaas-network-provider.iaas-network-provider-system.svc:80"
  httpRequestTimeout: "50s"
  eniDevPlugin:
    enabled: false
    resourceName: spidernet.io/sub-eni
    maxSlotsPerNode: 0
    kubeletRootDir: /var/lib/kubelet
    injectPodENIResources: true
```

- 如果 `iaasNetworkProvider.serverUrl` 为空，Spiderpool 不会调用 IaaS Network Provider。
- `iaasNetworkProvider.eniDevPlugin.enabled` 控制 spiderpool-agent 中的辅助 ENI slot device plugin。只有 `iaasNetworkProvider.serverUrl` 非空且该开关为 `true` 时，device plugin 才会启动。
- `iaasNetworkProvider.eniDevPlugin.maxSlotsPerNode` 是每个节点向调度器暴露的辅助 ENI slot 总容量。默认值 `0` 表示该插件启动后向 kubelet 广告 0 个可调度 slot；如果 Pod 请求 `spidernet.io/sub-eni`，调度器不会把它调度到这些节点。生产环境应按每个节点可用的辅助 ENI 容量设置为大于 0 的值。
- `iaasNetworkProvider.eniDevPlugin.kubeletRootDir` 用于推导挂载的 `device-plugins` 和 `plugins_registry` 目录，默认值为 `/var/lib/kubelet`。
- `iaasNetworkProvider.eniDevPlugin.injectPodENIResources` 控制是否由 Pod webhook 自动注入 `spidernet.io/sub-eni`。默认值为 `true`。设置为 `false` 时，device plugin 仍可启动并向节点广告容量，但 Spiderpool 不会自动给 Pod 添加该 resource request；需要用户在 Pod 资源里手动声明，否则调度器不会基于 ENI slot 做容量约束。
- 必须同时启用 `plugins.installVlanCNI`。
- 必须关闭 `ipam.enableGatewayDetection` 和 `ipam.enableIPConflictDetection` 关闭网关可达性检测和 IP 冲突检测。此模式和传统先调用 CNI 后调用 IPAM 方式不同，必须先调用 IPAM 获取 Iaas IP 信息才能调用 CNI 完成 Pod 网络设置。所以网关可达性检测和 IP 冲突检测在此模式下无法工作。

### 配置 HTTP 请求超时

`iaasNetworkProvider.httpRequestTimeout` 控制 Spiderpool 等待单次 Provider HTTP 调用（分配或释放）的最长时间，超时后该次调用被视为失败。

#### Provider 请求时序模型

一次 Provider 请求需要经历两个阶段：

| 阶段 | 最大耗时 | 说明 |
| --- | --- | --- |
| 限流等待 | 30 s | Provider 检查令牌桶是否有可用槽位，如果没有则最多等待 30 s 后再接受请求。 |
| Cloud API 调用 | 16 s | Provider 向底层云平台发起请求，网络延迟和云平台侧处理最多需要 16 s。 |
| **最坏情况合计** | **~48 s** | 两个阶段之和加上少量网络往返余量。 |

如果 `httpRequestTimeout` 设置低于 ~48 s，可能会在 Provider 已接受请求并开始在云平台侧执行时将其取消。这会导致状态不一致：Spiderpool 视为失败，但云平台侧的操作可能已经成功或正在进行中。

#### 建议值

| 场景 | 建议的 `httpRequestTimeout` |
| --- | --- |
| 默认 / 通用场景 | `50s`（默认值） |
| 低延迟私有云、无限流 | `20s` |
| 高竞争场景、限流等待时间较长 | `55s`–`59s`（必须保持 `< 100s`） |

#### 校验规则

- 必须是合法的 Go duration 字符串（例如 `50s`、`1m`）。
- 必须大于 `0`。
- 必须小于 `2m`（静态安全上限）。
- 必须小于 `100s`（CNI 插件调用 agent 的超时上限，适用于 ADD 和 DEL）。
- 为空时默认使用 `50s`。
- 校验失败是**致命错误**：agent 和 controller 将无法启动。

#### 时间预算层级

理解完整的预算链有助于说明 `httpRequestTimeout` 各项约束的来源：

| 层级 | 默认超时 | 说明 |
| --- | --- | --- |
| kubelet Sandbox 操作 | **2 min** | kubelet 为整个 Sandbox 创建（Pod 网络初始化）设置的默认超时。若 CNI 流水线在此窗口内未完成，Pod 启动失败。这是最外层的时间预算。 |
| Spiderpool CNI 插件 → agent 调用 | **100 s** | Spiderpool CNI 二进制调用 spiderpool-agent gRPC 接口时使用的超时。这是 agent 完成所有 IPAM 和 IaaS 工作的总预算，超时后 CNI 插件将放弃等待。 |
| IaaS Provider HTTP 调用 | **50 s**（默认） | 由 `httpRequestTimeout` 配置的单次调用超时。需要在 100 s agent 预算内，与其他 IPAM 工作共享预算。 |
| Provider 最坏情况完成时间 | **~48 s** | 单次 Provider 请求的最长耗时（30 s 限流等待 + 16 s Cloud API）。这是 `httpRequestTimeout` 有意义的最小值。 |

#### 运行时行为

每次发起 Provider HTTP 调用之前，Spiderpool 会检查父 CNI 操作 context（即 100 s agent 预算）的剩余时间：

- 如果剩余时间**小于 Provider 最坏情况耗时**（~48 s），Spiderpool **不会发起调用**，直接返回 `parent budget insufficient` 错误。这样可以避免 Provider 已消耗令牌桶但 Spiderpool 收到取消错误的状态不一致。
- 如果剩余时间充足，Spiderpool 会派生一个以 `httpRequestTimeout` 为上限的子 context 执行 HTTP 请求。实际生效的截止时间为 `min(当前时间 + httpRequestTimeout, 父 context 截止时间)`。

#### 错误信息说明

| 错误信息 | 含义 | 建议操作 |
| --- | --- | --- |
| `parent budget insufficient: Xs remaining is less than provider worst-case 48s` | CNI 流水线在到达 IaaS 调用之前已消耗了大部分预算。 | 检查流水线延迟；考虑提高 CNI 超时或降低 `httpRequestTimeout`。 |
| `provider-interaction timeout: ... exceeded configured timeout 50s` | Provider 未在 `httpRequestTimeout` 内响应。 | 检查 Provider 健康状态；如果 Provider 负载持续偏高，考虑适当提高 `httpRequestTimeout`。 |
| `parent budget exhausted: ... cancelled by parent context deadline` | Provider 正在响应时父 context 截止时间到达。 | 同上，父预算耗尽先于配置的超时触发。 |

> **注意**：[VLAN-CNI](https://github.com/spidernet-io/vlan-cni) 是 Spiderpool 基于社区 cni-plugin 项目开发的 VLAN CNI 插件，用于对接第三方云平台 IaaS Network Provider，为容器创建 IaaS 层的 VLAN 子网卡。

### 检查功能是否已启用

安装后可以通过以下方式确认该功能是否已生效：

1. **查看 ConfigMap**

   ```bash
   kubectl get configmap spiderpool-conf -n <spiderpool-namespace> -o yaml | grep iaasNetworkProvider
   ```

   如果输出中包含 `iaasNetworkProvider.serverUrl` 且值非空，说明功能已启用。

2. **查看 agent 启动日志**

   ```bash
   kubectl logs spiderpool-agent-xxx -n <spiderpool-namespace>
   ```

   在 agent 启动日志中搜索 `IaaS client created successfully`。如果看到该日志，说明 agent 已成功初始化 IaaS client，功能已启用。如果看到 `IaaS provider configuration validation failed`，说明配置存在问题，需要检查 `serverUrl` 格式是否正确。

### 配置 VLAN CNI

对接 IaaS Network Provider 时，必须使用 VLAN CNI 为 Pod 创建 VLAN 子接口，并将云平台分配的 VLAN ID 和 MAC 地址等属性配置到该子接口上，以确保 Pod 网卡配置与云平台侧保持一致，从而实现正常通信。

如果手动静态配置 VLAN ID，将与云平台动态分配的 VLAN ID 不一致，导致网络通信异常。因此 **SpiderMultusConfig 的 `vlan` 配置中不能填写 `vlanID`**，否则 [vlan-cni](https://github.com/spidernet-io/vlan-cni) 将无法为 Pod 创建配置正确的 VLAN 子接口。

> [vlan-cni](https://github.com/spidernet-io/vlan-cni) 在 Pod 创建时通过 Unix socket 向本地 spiderpool-agent 查询从 IaaS 分配的 VLAN ID 和 MAC 地址等信息，然后基于这些信息在 Pod 网络命名空间中创建 VLAN 子接口。

### 辅助 ENI slot 调度

当 provider 模式使用云平台分配的辅助 ENI 时，可以开启 `iaasNetworkProvider.eniDevPlugin`，让 spiderpool-agent 通过 Kubernetes device plugin API 暴露 `spidernet.io/sub-eni`。Kubernetes 会将该资源记录到节点的 capacity 和 allocatable 中，调度器会基于已经调度的 Pod resource request 做容量扣减。

`spidernet.io/sub-eni` 表示 kubelet 暴露的健康可调度总容量，不是由 Spiderpool 手动维护的 `node.status` 空闲计数。kubelet 或 spiderpool-agent 重启后，device plugin 会重新注册，kubelet 会基于当前健康设备列表重建节点容量。

如果 `maxSlotsPerNode=0`，device plugin 会正常注册，但不会广告任何健康 slot。该配置适用于默认关闭容量或临时阻止需要辅助 ENI 的 Pod 调度；启用实际调度保护时应将其设置为节点真实可用的辅助 ENI slot 数。

只有同时满足以下条件时，ENI device plugin 才会启动并挂载 kubelet plugin 目录：

- `iaasNetworkProvider.serverUrl` 非空，即 provider 模式已启用。
- `iaasNetworkProvider.eniDevPlugin.enabled=true`。

启动后，spiderpool-agent 会挂载 `{kubeletRootDir}/device-plugins` 和 `{kubeletRootDir}/plugins_registry`。Kubernetes v1.13 起，kubelet 外部插件注册目录从 `{kubeletRootDir}/plugins/` 调整为 `{kubeletRootDir}/plugins_registry/`；但 device plugin v1beta1 API 中 kubelet 注册 socket 的历史路径仍是 `{kubeletRootDir}/device-plugins/kubelet.sock`。因此 Spiderpool 同时挂载 `device-plugins` 和 `plugins_registry` 以兼容不同节点环境：运行时优先使用 `{kubeletRootDir}/plugins_registry`，只有该目录不存在时才回退到 `{kubeletRootDir}/device-plugins`。如果节点使用非默认 kubelet root，需要将 `iaasNetworkProvider.eniDevPlugin.kubeletRootDir` 设置为对应的宿主机路径。

如果 `injectPodENIResources` 开启，Pod mutating webhook 可以为已经引用合格 VLAN `SpiderMultusConfig` 的 Pod 自动注入 ENI slot resource request。合格的 VLAN `SpiderMultusConfig` 指 provider 模式下使用的 VLAN 配置，且未设置 VLAN ID，让 Provider 在分配阶段返回 VLAN ID。如果 Pod 已经声明了相同的 ENI slot resource key，Spiderpool 会保留用户原始配置，不会覆盖。

如果 `injectPodENIResources=false`，自动注入逻辑会关闭。此时只有用户显式在 Pod container 的 `resources.requests` 和 `resources.limits` 中声明 `spidernet.io/sub-eni`，调度器才会执行 ENI slot 容量检查；未声明该资源的 Pod 会按普通 provider-mode Pod 处理。

例如，Pod 通过 Multus 注解引用了两个合格的 VLAN `SpiderMultusConfig`，则 webhook 会在第一个容器的 requests 和 limits 中注入 `spidernet.io/sub-eni: 2`。如果 Pod 已经声明 `spidernet.io/sub-eni`，webhook 不会修改该 Pod。

slot 释放遵循 Kubernetes extended-resource accounting。Pod 删除或启动前失败后，kubelet 和调度器会通过正常 Pod 生命周期释放 device plugin allocation 和 resource request。Spiderpool 仍然通过已有 IPAM/IaaS 清理路径负责 provider IP/ENI 的分配和释放；device plugin 只负责调度和 kubelet admission 阶段的容量门控。

平台管理员需要提前在 IaaS 侧完成以下准备：

- 创建 VPC 子网并绑定到节点弹性网卡。例如，将 VPC 子网 `172.91.0.0/24` 绑定到节点 ECS-01 的网卡 `enp0s28`。

然后在 PaaS 侧创建对应的 SpiderMultusConfig 和 SpiderIPPool 资源，示例如下：

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
      - enp0s28
    ippools:
      ipv4:
        - pool-enp0s28
---
apiVersion: spiderpool.spidernet.io/v2beta1
kind: SpiderIPPool
metadata:
  name: pool-enp0s28
spec:
  gateway: 172.91.0.1
  ips:
    - 172.91.0.100-172.91.0.120
  subnet: 172.91.0.0/24
```

- `master` 为必填字段，必须与节点上的物理网卡名称一致，且要求集群内各节点的网卡名称保持统一。
- `subnet` 为必填字段，必须与云平台侧的 VPC 子网保持一致。

## API 契约

Provider 需要实现以下 HTTP API。

### 分配 IP

#### 请求

```text
POST /v1/apis/network.iaas.io/ipam/allocate-ips
Content-Type: application/json
```

请求体：

```json
{
  "podName": "example-pod",
  "podNamespace": "default",
  "podUID": "9f8b7c6d-xxxx-xxxx-xxxx-xxxxxxxxxxxx",
  "nodeName": "worker-1",
  "iaasIPsAllocationRequest": [
    {
      "ipAddress": "10.0.0.10",
      "subnet": "10.0.0.0/24",
      "parentNicMac": "fa:16:3e:11:22:33"
    }
  ]
}
```

字段说明：

| 字段 | 是否必填 | 说明 |
| --- | --- | --- |
| `podName` | 否 | Pod 名称。 |
| `podNamespace` | 否 | Pod 命名空间。 |
| `podUID` | 否 | Pod UID。 |
| `nodeName` | 是 | Pod 所在节点。 |
| `iaasIPsAllocationRequest` | 是 | Spiderpool 已分配、期望 Provider 绑定的 IP 列表。 |
| `ipAddress` | 是 | 不带 CIDR 前缀的 IP 地址。 |
| `subnet` | 是 | IP 所属的子网 CIDR。 |
| `parentNicMac` | 是 | 承载该 Pod 网络的父网卡 MAC 地址。 |

#### 响应

任意 HTTP `2xx` 状态码都会被 Spiderpool 视为成功。

响应体：

```json
{
  "podName": "example-pod",
  "podNamespace": "default",
  "nodeName": "worker-1",
  "iaasIPsAllocationResponse": [
    {
      "parentNicMac": "fa:16:3e:11:22:33",
      "subnet": "10.0.0.0/24",
      "ipAddress": "10.0.0.10",
      "macAddress": "fa:16:3e:aa:bb:cc",
      "vlanId": 100
    }
  ]
}
```

字段说明：

| 字段 | 是否必填 | 说明 |
| --- | --- | --- |
| `iaasIPsAllocationResponse` | 是 | Provider 返回的分配结果列表。 |
| `parentNicMac` | 是 | Provider 使用的父网卡 MAC 地址。 |
| `subnet` | 是 | IP 所属的子网 CIDR。 |
| `ipAddress` | 是 | Provider 已完成绑定的 IP 地址。 |
| `macAddress` | 否 | 云平台为 Pod 网卡分配的 MAC 地址。 |
| `vlanId` | 否 | 云平台分配的 VLAN ID。 |

如果 `macAddress` 或 `vlanId` 为空，Spiderpool 会保留原始分配结果中的对应字段。

### 释放 IP

#### 请求

```text
POST /v1/apis/network.iaas.io/ipam/release-ip
Content-Type: application/json
```

请求体：

```json
{
  "podName": "example-pod",
  "podNamespace": "default",
  "podUID": "9f8b7c6d-xxxx-xxxx-xxxx-xxxxxxxxxxxx",
  "nodeName": "worker-1",
  "parentNicMac": "fa:16:3e:11:22:33",
  "subnet": "10.0.0.0/24",
  "ipAddress": "10.0.0.10"
}
```

字段说明：

| 字段 | 是否必填 | 说明 |
| --- | --- | --- |
| `podName` | 否 | Pod 名称。 |
| `podNamespace` | 否 | Pod 命名空间。 |
| `podUID` | 否 | Pod UID。 |
| `nodeName` | 是 | Pod 原本所在节点。 |
| `parentNicMac` | 否 | 父网卡 MAC 地址。在 controller 侧 GC 场景下可能为空。 |
| `subnet` | 是 | IP 所属的子网 CIDR。 |
| `ipAddress` | 是 | 需要释放的 IP 地址。 |

#### 响应

Spiderpool 会忽略响应体。任意 HTTP `2xx` 状态码都会被视为成功。

## 特殊场景处理

### 分配接口必须同步成功

Spiderpool 在分配 IP 时采用同步调用方式：只有 Provider 完成 IaaS 侧 IP 绑定并正常返回网络配置后，Spiderpool 才会更新该 IP 在 SpiderIPPool 中的状态，并创建或更新对应的 SpiderEndpoint 对象。

在一些异常场景下：

- 如果 Provider 或云平台对 API 进行限流，处理时间过长导致 Spiderpool 等待 HTTP 响应超时，本次分配将被视为失败。
- 如果 Provider 侧故障无法响应，Spiderpool 会等待超时时间后将本次分配视为失败。

如果 Spiderpool-agent 在配置的 `httpRequestTimeout` 时间内（默认 `50s`）没有收到 Provider 的成功响应，那么本次分配将被视为失败，会阻止 Pod 创建，Pod 会遵循 K8s 的重试机制进行重试。

### 释放接口应该具备幂等性

释放接口应该是幂等的。如果 IP 已经释放，或者云平台侧已经不存在该 IP 绑定关系，只要可以安全地认为该 IP 已释放，Provider 就应该返回 `2xx` 状态码。

这样可以避免 CNI DEL 重复调用或 GC 重试时产生不必要的失败。

### 释放操作支持最终一致

某些云平台的 IP 释放操作较慢，受限速或异步清理机制影响，Provider 收到释放请求后，云平台侧资源不一定立即完成清理。

Spiderpool 要求 Provider 能够接收释放请求并启动云平台侧清理流程。只要释放请求已被接受，或 IP 已处于已释放状态，Provider 即可返回成功。

Spiderpool 会先调用 IaaS 释放接口，再释放 Spiderpool 内部 IP 池中的 IP。这个顺序可以避免 Spiderpool 在云平台尚未接受释放请求前重新分配同一个 IP。如果云平台在此之后异步完成最终清理，不会阻塞 Spiderpool 当前的 IP 释放流程。

### 父网卡 MAC 地址

当 Spiderpool 能够解析父网卡 MAC 地址时，会在请求中携带 `parentNicMac`。在 agent 侧的分配和释放场景下，Spiderpool 通常可以通过运行时网络环境或本地缓存获取该值。

在 controller 侧 GC 场景中，Spiderpool 不一定运行在各节点的 host network namespace 中，因此可能无法获取父网卡 MAC 地址。此时，Spiderpool 发送的释放请求中 `parentNicMac` 字段可能为空，Provider 的释放接口需要能够容忍该字段缺失。

## 异常场景处理

Spiderpool 会将以下情况视为失败：

- HTTP 请求失败。
- HTTP 响应状态码不是 `2xx`。
- 分配响应 JSON 无法解析。
- 分配响应中包含 Spiderpool 未请求的 IP。

当释放失败时，Spiderpool 可能根据触发释放的路径，在后续清理流程中进行重试。因此 Provider 的释放接口应支持幂等重试。
