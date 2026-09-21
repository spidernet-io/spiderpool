# IaaS Network Provider 架构

[**English**](./iaas-network-provider.md) | **简体中文**

本文介绍 Spiderpool IaaS Network Provider 集成的设计与内部机制：分配/释放调用流程、池的放置模式、候选池选择规则、HTTP 超时模型，以及 Provider 需要实现的 API 契约。

安装与使用步骤请参考 [IaaS Network Provider](../usage/iaas-network-provider-zh_CN.md)。

## 工作原理

启用该能力后，Spiderpool 会执行以下流程：

1. Pod IP 分配阶段，Spiderpool 先从 Spiderpool IP 池中分配 IP，然后调用 IaaS Network Provider 的分配接口。
2. IaaS Network Provider 在云平台侧完成 IP 绑定，并返回云平台侧的网络属性。
3. Spiderpool 将返回的 MAC 地址和 VLAN ID 写入分配结果，后续 VLAN CNI 流程使用这些信息配置 Pod 网卡。
4. Pod IP 释放阶段，Spiderpool 会针对每个需要释放的地址调用 IaaS Network Provider 的释放接口。
5. IaaS 释放接口调用成功后，Spiderpool 再从内部 IP 池中释放该 IP。这里的“调用成功”代表 IaaS Network Provider 已成功接收释放请求并开始云平台侧清理，并不保证云平台侧 IP 资源已经彻底释放完成（云平台可能因限速或异步机制仍在处理）。

IaaS Network Provider 是一个 HTTP 服务。Spiderpool 只定义通用 API 契约，不依赖某个具体云厂商实现。

## 池的放置模式

带 IaaS 后端的 `SpiderIPPool` 支持两种放置模式：

* **节点级池**（默认）：池通过 `spec.nodeName` 固定到单个节点，Provider 会提前在该节点上预热 IP 资源。分配时优先使用已预热、即拿即用的地址，并跳过同步的 Provider 调用。
* **全局池**：池带有 `iaas-provider` 标记但**不**设置 `spec.nodeName`。一个池服务一个 Deployment（或类似工作负载），其 Pod 分布在多个节点上，因此按节点预热不再适用，改为实时分配加粘性子网卡（sub-ENI）缓存。

池的模式完全由池的形态推导，并在池的生命周期内保持不变：validating webhook 会拒绝在已创建的 IaaS 池上增加或删除 `spec.nodeName`。

### 节点级池的预热机制

节点级池是严格 prewarm-only 的。Provider 提前在池所在节点上创建 sub-ENI，并将结果写入池的 `status.ipMetaData`。分配时，候选地址集合是池 `spec.ips` 派生的空闲地址与 `status.ipMetaData` 中 ready 条目的**交集**；Spiderpool 绝不会分配一个云侧尚未准备好的节点级地址。如果没有任何 ready 条目（例如池刚创建、Provider 尚未回写 metadata），分配会以 IP 用尽错误失败，kubelet 会持续重试，直到预热地址出现。

为了让 Provider 获取云侧父端口信息，池所在节点上的 spiderpool-agent 会解析池注解 `ipam.spidernet.io/parent-nic` 指定网卡的 MAC 地址，并将名字与 MAC 一起发布到池的 `status.parentNic`。对于配对的双栈池，只有主池（IPv4）携带 `status.parentNic`。

### 全局池模式

全局模式下：

1. 当 Pod 调度到的节点上，池里恰好有一个已绑定到该节点的空闲 IP（缓存的子网卡）时，Spiderpool 直接复用它，**无需**调用 Provider —— Pod 快速启动。
2. 否则 Spiderpool 选择一个空闲地址（优先选择尚未创建子网卡的地址，其次才从其他节点上“偷取”空闲子网卡，以尽量减少云 API 调用），并同步调用 Provider 在 Pod 所在节点上创建/挂载子网卡。
3. Pod 删除时，Spiderpool 侧释放该 IP，但云侧子网卡仍保留在节点上作为缓存，供该节点的下一个 Pod 使用。
4. 当池的使用率超过水位线时，回收空闲子网卡是 **Provider** 的职责。Provider 在解绑空闲子网卡前，会先将其元数据条目标记为 `vlan: -1`；Spiderpool 绝不会分配处于该状态的条目，从而避免 Pod 拿到一个正在被解绑的 IP。
5. 双栈场景下，子网卡创建时选定的 IPv6 地址在该子网卡的整个生命周期内保持粘性。

如果全局池分配过程中同步 Provider 调用失败，Spiderpool 会回滚刚占用的地址，保证重试从干净状态开始。

## 候选池类别排他

当 Pod 网卡的候选池混合了不同类别时，Spiderpool 只保留最高类别的池并忽略其余候选（打印告警日志并向 Pod 发送 Warning 事件），确保 IaaS 分配绝不静默降级到低类别池：

1. **配对 IaaS 主池**（双栈开启时）：配对分配是“要么成对、要么失败”，若降级到非配对池会静默产生单栈 Pod。
2. **IaaS 池**（节点预热池或全局池）：其地址是云端子网卡，MAC/VLAN 由 Provider 管理，与静态池降级语义不兼容。节点预热池与全局池同属该类别、允许混用——预热池排序在前，全局池作为兜底。
3. **传统（非 IaaS）池**。

类别判定基于用户配置的候选池集合，在按节点过滤之前完成，因此在任何节点上行为都是确定的。若保留类别的池全部分配失败，则直接分配失败，绝不回落到被忽略的池。

## HTTP 请求超时模型

`iaasNetworkProvider.httpRequestTimeout` 控制 Spiderpool 等待单次 Provider HTTP 调用（分配或释放）的最长时间，超时后该次调用被视为失败。

### Provider 请求时序模型

一次 Provider 请求需要经历两个阶段：

| 阶段 | 最大耗时 | 说明 |
| --- | --- | --- |
| 限流等待 | 30 s | Provider 检查令牌桶是否有可用槽位，如果没有则最多等待 30 s 后再接受请求。 |
| Cloud API 调用 | 16 s | Provider 向底层云平台发起请求，网络延迟和云平台侧处理最多需要 16 s。 |
| **最坏情况合计** | **~48 s** | 两个阶段之和加上少量网络往返余量。 |

如果 `httpRequestTimeout` 设置低于 ~48 s，可能会在 Provider 已接受请求并开始在云平台侧执行时将其取消。这会导致状态不一致：Spiderpool 视为失败，但云平台侧的操作可能已经成功或正在进行中。

### 建议值

| 场景 | 建议的 `httpRequestTimeout` |
| --- | --- |
| 默认 / 通用场景 | `50s`（默认值） |
| 低延迟私有云、无限流 | `20s` |
| 高竞争场景、限流等待时间较长 | `55s`–`59s`（必须保持 `< 100s`） |

### 校验规则

* 必须是合法的 Go duration 字符串（例如 `50s`、`1m`）。
* 必须大于 `0`。
* 必须小于 `2m`（静态安全上限）。
* 必须小于 `100s`（CNI 插件调用 agent 的超时上限，适用于 ADD 和 DEL）。
* 为空时默认使用 `50s`。
* 校验失败是**致命错误**：agent 和 controller 将无法启动。

### 时间预算层级

理解完整的预算链有助于说明 `httpRequestTimeout` 各项约束的来源：

| 层级 | 默认超时 | 说明 |
| --- | --- | --- |
| kubelet Sandbox 操作 | **2 min** | kubelet 为整个 Sandbox 创建（Pod 网络初始化）设置的默认超时。若 CNI 流水线在此窗口内未完成，Pod 启动失败。这是最外层的时间预算。 |
| Spiderpool CNI 插件 → agent 调用 | **100 s** | Spiderpool CNI 二进制调用 spiderpool-agent gRPC 接口时使用的超时。这是 agent 完成所有 IPAM 和 IaaS 工作的总预算，超时后 CNI 插件将放弃等待。 |
| IaaS Provider HTTP 调用 | **50 s**（默认） | 由 `httpRequestTimeout` 配置的单次调用超时。需要在 100 s agent 预算内，与其他 IPAM 工作共享预算。 |
| Provider 预算校验 | Provider 侧配置 | Provider 根据自身配置的限流排队等待和云事务超时，校验调用方通过 `X-Request-Timeout-Ms` 传递的预算，预算不足的请求会被直接拒绝。 |

### 运行时行为

每次发起 Provider HTTP 调用时：

* Spiderpool 会派生一个以 `httpRequestTimeout` 为上限的子 context 执行 HTTP 请求。实际生效的截止时间为 `min(当前时间 + httpRequestTimeout, 父 context 截止时间)`。
* Spiderpool 会通过 `X-Request-Timeout-Ms` HTTP header 传递本次请求的有效剩余预算。该值是正整数，单位为毫秒，由 Spiderpool 在发送 HTTP 请求前基于请求 context 计算。
* Provider 是自身限制的唯一事实源：它将收到的预算与自身配置的限流排队等待加云事务超时进行比较，预算不足时**在消耗限流令牌之前直接拒绝请求**。这样既避免了中途取消导致云侧操作状态不一致，也能自动跟随 Provider 侧配置变更，无需在 Spiderpool 侧同步维护对应配置。

### 错误信息说明

| 错误信息 | 含义 | 建议操作 |
| --- | --- | --- |
| Provider 返回预算/限流超时类错误 | CNI 流水线在到达 IaaS 调用之前已消耗了大部分预算，或 Provider 排队已饱和。 | 检查流水线延迟和 Provider 负载；考虑提高 CNI 超时或调整 Provider 侧限流配置。 |
| `provider-interaction timeout: ... exceeded configured timeout 50s` | Provider 未在 `httpRequestTimeout` 内响应。 | 检查 Provider 健康状态；如果 Provider 负载持续偏高，考虑适当提高 `httpRequestTimeout`。 |
| `parent budget exhausted: ... cancelled by parent context deadline` | Provider 正在响应时父 context 截止时间到达。 | 同上，父预算耗尽先于配置的超时触发。 |

## API 契约

Provider 需要实现以下 HTTP API。

### 分配 IP

分配 API 用于创建 sub-ENI。每个请求项对应一个 sub-network-interface，携带该工作负载网卡实际分配到的地址族：仅 IPv4、仅 IPv6，或原子化配置的一对 IPv4/IPv6。Spiderpool 按实际分配结果透传地址族，不做任何强制限制；地址族要求（例如 sub-ENI 需要 IPv4 标识）由 Provider 自行校验。

#### 请求

```text
POST /v1/apis/network.iaas.io/ipam/allocate-ips
Content-Type: application/json
X-Request-Timeout-Ms: 50000
```

请求 Header：

| Header | 是否必填 | 说明 |
| --- | --- | --- |
| `X-Request-Timeout-Ms` | 是 | 本次请求的剩余预算，单位为毫秒。Provider 应将其视为从收到请求开始可用的最大处理时间，并应在该预算耗尽前返回。 |

请求体：

```json
{
  "podName": "example-pod",
  "podNamespace": "default",
  "podUID": "9f8b7c6d-xxxx-xxxx-xxxx-xxxxxxxxxxxx",
  "nodeName": "worker-1",
  "subEniRequests": [
    {
      "parentNicMac": "fa:16:3e:11:22:33",
      "subnet": "10.0.0.0/24",
      "ipv4Address": "10.0.0.10",
      "ipv6Address": "fd00::10",
      "ipv4PoolName": "example-pool-v4",
      "ipv6PoolName": "example-pool-v6"
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
| `subEniRequests` | 是 | Spiderpool 期望 Provider 创建的 sub-ENI 列表，每一项对应一个 sub-ENI，携带实际分配到的地址族。 |
| `parentNicMac` | 是 | 承载该 Pod 网络的父网卡 MAC 地址。 |
| `subnet` | 是 | sub-ENI 所属云子网。分配了 IPv4 时以其 IPv4 CIDR 标识（双栈时两个地址族共享），否则以 IPv6 CIDR 标识。 |
| `ipv4Address` | 否 | 不带 CIDR 前缀的 IPv4 地址，仅 IPv6 分配时为空。 |
| `ipv6Address` | 否 | 不带 CIDR 前缀的 IPv6 地址，双栈时与 `ipv4Address` 配对绑定到同一个 sub-ENI，仅 IPv4 分配时为空。 |
| `ipv4PoolName` | 否 | IPv4 地址所属的 SpiderIPPool 名称。Provider 可据此将 sub-ENI 归因到具体池（如全局池的 ownership 打标与 metadata 回写），无需按 `{subnet, ip}` 反查。 |
| `ipv6PoolName` | 否 | IPv6 地址所属的 SpiderIPPool 名称，用途同 `ipv4PoolName`。 |

#### 响应

任意 HTTP `2xx` 状态码都会被 Spiderpool 视为成功。

响应体：

```json
{
  "podName": "example-pod",
  "podNamespace": "default",
  "nodeName": "worker-1",
  "subEniResponses": [
    {
      "parentNicMac": "fa:16:3e:11:22:33",
      "subnet": "10.0.0.0/24",
      "ipv4Address": "10.0.0.10",
      "ipv6Address": "fd00::10",
      "macAddress": "fa:16:3e:aa:bb:cc",
      "vlanId": 100
    }
  ]
}
```

字段说明：

| 字段 | 是否必填 | 说明 |
| --- | --- | --- |
| `subEniResponses` | 是 | Provider 返回的 sub-ENI 创建结果列表。 |
| `parentNicMac` | 是 | Provider 使用的父网卡 MAC 地址。 |
| `subnet` | 是 | sub-ENI 所属的子网 CIDR。 |
| `ipv4Address` | 否 | Provider 已完成绑定的 IPv4 地址，仅 IPv6 的 sub-ENI 为空。 |
| `ipv6Address` | 否 | Provider 已完成绑定的 IPv6 地址，仅 IPv4 的 sub-ENI 为空。 |
| `macAddress` | 否 | sub-ENI 的 MAC 地址，由两个地址族共享。 |
| `vlanId` | 否 | 云平台分配的 VLAN ID，由两个地址族共享。 |

如果 `macAddress` 或 `vlanId` 为空，Spiderpool 会保留原始分配结果中的对应字段；否则该 sub-ENI 上所有已分配地址族的结果都会使用共享的 `macAddress`/`vlanId`。

### 释放 IP

释放双栈 sub-ENI 的任一地址都会删除整个云侧 sub-ENI 资源，因此 Spiderpool 对每个 sub-ENI 只发送一次释放请求，使用其 IPv4 地址（仅 IPv6 的 sub-ENI 则使用 IPv6 地址），配对的地址随之一并释放。

#### 请求

```text
POST /v1/apis/network.iaas.io/ipam/release-ip
Content-Type: application/json
X-Request-Timeout-Ms: 50000
```

请求 Header：

| Header | 是否必填 | 说明 |
| --- | --- | --- |
| `X-Request-Timeout-Ms` | 是 | 本次请求的剩余预算，单位为毫秒。Provider 应将其视为从收到请求开始可用的最大处理时间，并应在该预算耗尽前返回。 |

请求体：

```json
{
  "podName": "example-pod",
  "podNamespace": "default",
  "podUID": "9f8b7c6d-xxxx-xxxx-xxxx-xxxxxxxxxxxx",
  "nodeName": "worker-1",
  "parentNicMac": "fa:16:3e:11:22:33",
  "subnet": "10.0.0.0/24",
  "ipAddress": "10.0.0.10",
  "poolName": "example-pool-v4"
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
| `poolName` | 否 | 被释放 IP 所属的 SpiderIPPool 名称，用途同分配 API 的 `ipv4PoolName`。 |

#### 响应

Spiderpool 会忽略响应体。任意 HTTP `2xx` 状态码都会被视为成功。

## 特殊场景处理

### 分配接口必须同步成功

Spiderpool 在分配 IP 时采用同步调用方式：只有 Provider 完成 IaaS 侧 IP 绑定并正常返回网络配置后，Spiderpool 才会更新该 IP 在 SpiderIPPool 中的状态，并创建或更新对应的 SpiderEndpoint 对象。

在一些异常场景下：

* 如果 Provider 或云平台对 API 进行限流，处理时间过长导致 Spiderpool 等待 HTTP 响应超时，本次分配将被视为失败。
* 如果 Provider 侧故障无法响应，Spiderpool 会等待超时时间后将本次分配视为失败。

如果 Spiderpool-agent 在配置的 `httpRequestTimeout` 时间内（默认 `50s`）没有收到 Provider 的成功响应，那么本次分配将被视为失败，会阻止 Pod 创建，Pod 会遵循 K8s 的重试机制进行重试。

### 释放接口应该具备幂等性

释放接口应该是幂等的。如果 IP 已经释放，或者云平台侧已经不存在该 IP 绑定关系，只要可以安全地认为该 IP 已释放，Provider 就应该返回 `2xx` 状态码。

这样可以避免 CNI DEL 重复调用或 GC 重试时产生不必要的失败。

### 释放操作支持最终一致

某些云平台的 IP 释放操作较慢，受限速或异步清理机制影响，Provider 收到释放请求后，云平台侧资源不一定立即完成清理。

Spiderpool 要求 Provider 能够接收释放请求并启动云平台侧清理流程。只要释放请求已被接受，或 IP 已处于已释放状态，Provider 即可返回成功。

Spiderpool 会先调用 IaaS 释放接口，再释放 Spiderpool 内部 IP 池中的 IP。这个顺序可以避免 Spiderpool 在云平台尚未接受释放请求前重新分配同一个 IP。如果云平台在此之后异步完成最终清理，不会阻塞 Spiderpool 当前的 IP 释放流程。

### 父网卡 MAC 地址

当 Spiderpool 能够解析父网卡 MAC 地址时，会在请求中携带 `parentNicMac`。在 agent 侧的分配和释放场景下，Spiderpool 按以下顺序解析该值：子网缓存 → 节点级池由 agent 发布的 `status.parentNic.mac` → 池的 `parent-nic` 注解名字在本地 netlink 解析 → 最终回退到 SpiderMultusConfig 的 `master` 接口。

在 controller 侧 GC 场景中，Spiderpool 不一定运行在各节点的 host network namespace 中，因此可能无法获取父网卡 MAC 地址。此时，Spiderpool 发送的释放请求中 `parentNicMac` 字段可能为空，Provider 的释放接口需要能够容忍该字段缺失。

## 异常场景处理

Spiderpool 会将以下情况视为失败：

* HTTP 请求失败。
* HTTP 响应状态码不是 `2xx`。
* 分配响应 JSON 无法解析。
* 分配响应中包含 Spiderpool 未请求的 IP。

当释放失败时，Spiderpool 可能根据触发释放的路径，在后续清理流程中进行重试。因此 Provider 的释放接口应支持幂等重试。
