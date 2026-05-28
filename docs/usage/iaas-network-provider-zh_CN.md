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

通过 Helm values 配置 Provider URL：

```yaml
ipam:
  enableGatewayDetection: false
  enableIPConflictDetection: false
plugins:
  installVlanCNI: true
iaasNetworkProvider:
  serverUrl: "http://iaas-network-provider.iaas-network-provider-system.svc:80"
```

- 如果 `iaasNetworkProvider.serverUrl` 为空，Spiderpool 不会调用 IaaS Network Provider。
- 必须同时启用 `plugins.installVlanCNI`。
- 必须关闭 `ipam.enableGatewayDetection` 和 `ipam.enableIPConflictDetection` 关闭网关可达性检测和 IP 冲突检测。此模式和传统先调用 CNI 后调用 IPAM 方式不同，必须先调用 IPAM 获取 Iaas IP 信息才能调用 CNI 完成 Pod 网络设置。所以网关可达性检测和 IP 冲突检测在此模式下无法工作。

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

如果 Spiderpool-agent 在指定时间内（2 min）没有收到 Provider 的成功响应，那么本次分配将被视为失败, 会阻止 Pod 创建，Pod 会遵循 K8s 的重试机制进行重试。

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
