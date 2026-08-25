# Grafana Dashboard

[**English**](./grafana-dashboard.md) | **简体中文**

Spiderpool 内置 Grafana Dashboard，可视化展示 IPAM 和 RDMA 指标。

## 前置条件

- [Grafana Operator](https://github.com/grafana-operator/grafana-operator)（管理 Dashboard CRD）
- Prometheus
- Spiderpool Metrics 已启用

## 安装

### Helm 方式

安装 Spiderpool 时启用 Dashboard：

```bash
helm install spiderpool spidernet-io/spiderpool \
  -n kube-system \
  --set grafanaDashboard.install=true \
  --set spiderpoolAgent.prometheus.enabled=true \
  --set spiderpoolController.prometheus.enabled=true
```

指定 Dashboard 命名空间：

```bash
helm install spiderpool spidernet-io/spiderpool \
  -n kube-system \
  --set grafanaDashboard.install=true \
  --set grafanaDashboard.namespace=monitoring
```

### 手动导入

Dashboard JSON 文件在 `charts/spiderpool/files/` 目录：

| 文件 | 说明 |
|------|------|
| `grafana-ipam.json` | IPAM 指标 |
| `grafana-rdma-pod.json` | Pod 粒度 RDMA 指标 |
| `grafana-rdma-node.json` | 节点粒度 RDMA 指标 |
| `grafana-rdma-cluster.json` | 集群粒度 RDMA 指标 |
| `grafana-rdma-workload.json` | 工作负载粒度 RDMA 指标 |

## Helm 参数

| 参数 | 说明 | 默认值 |
|------|------|--------|
| `grafanaDashboard.install` | 安装 Dashboard，依赖 Grafana Operator CRDs | `false` |
| `grafanaDashboard.namespace` | Dashboard 命名空间，默认同 Helm release | `""` |
| `grafanaDashboard.annotations` | 附加 annotations | `{}` |
| `grafanaDashboard.labels` | 附加 labels | `{}` |

## Dashboard 内容

**IPAM Dashboard** 由三行面板组成：

**IPAM Overview** —— 核心统计：

| 面板 | 说明 |
|------|------|
| Total IPPools / Total Subnets | 当前 SpiderIPPool 与 SpiderSubnet 资源数量 |
| Allocation Rate / Release Rate | 当前 IPAM 分配与释放请求速率 |

**IPAM Allocation / Release** —— IPAM 核心行为：

| 面板 | 说明 |
|------|------|
| Allocation & Release Rate | spiderpool-agent 处理的 IPAM 分配与释放请求速率 |
| IPAM Failures & GC | 分配/释放失败速率与 IP GC 活动；失败曲线正常应恒为零 |
| IP Allocation Latency | 端到端分配延迟 P50/P95/P99 |
| IP Release Latency | 端到端释放延迟 P50/P95/P99 |

**IaaS Network Provider** —— 适用于使用 IaaS 托管 IP 池的集群：

| 面板 | 说明 |
|------|------|
| IaaS Allocation Rate by Path | 按模式（`node`/`global`）与路径（`cache_hit` 预热复用 / `cold_create` 实时云端创建）拆分的分配速率 |
| IaaS Cache Hit Ratio | 直接命中预热元数据（无需 provider RPC）的分配占比 |
| IaaS Allocation Latency by Path | 分配延迟 P50/P95；`cache_hit` 应为毫秒级，`cold_create` 包含云端 sub-ENI 创建 |
| Provider API Request Rate | Agent 调用 provider 的客户端请求速率，按操作（`allocate`/`release`）拆分 |
| Provider API Latency (client-side) | Agent 视角的 provider 调用延迟 P50/P95 |
| IaaS Failures & Anomalies | 全部 IaaS 失败计数（分配失败、RPC 失败、claim 回滚、元数据解码失败），健康集群只显示 0 基线 |

**RDMA Dashboard** 按不同粒度展示 RDMA 网络指标：

| Dashboard | 粒度 | 指标示例 |
|-----------|------|----------|
| RDMA Pod | Pod | 读写请求数、错误计数、CNP 包 |
| RDMA Node | 节点 | RDMA 设备状态、端口速率 |
| RDMA Cluster | 集群 | RDMA 资源总览 |
| RDMA Workload | 工作负载 | Deployment/StatefulSet 的 RDMA 用量 |

## 启用 RDMA 指标

RDMA Dashboard 依赖 RDMA 指标采集：

```bash
helm install spiderpool spidernet-io/spiderpool \
  -n kube-system \
  --set grafanaDashboard.install=true \
  --set spiderpoolAgent.prometheus.enabled=true \
  --set spiderpoolAgent.prometheus.enabledRdmaMetric=true
```

## 故障排除

### Dashboard 无数据

首先确认 Prometheus 正在采集 spiderpool-agent 和 spiderpool-controller 指标，检查 ServiceMonitor 是否创建。然后验证 Metrics 环境变量：

```bash
kubectl get pods -n kube-system -l app.kubernetes.io/component=spiderpool-agent \
  -o jsonpath='{.items[0].spec.containers[0].env[?(@.name=="SPIDERPOOL_ENABLED_METRIC")].value}'
```

### RDMA 指标无数据

确认节点有 RDMA 设备，且 `spiderpoolAgent.prometheus.enabledRdmaMetric` 设置为 `true`。
