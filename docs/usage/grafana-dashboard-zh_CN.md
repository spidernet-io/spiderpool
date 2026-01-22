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

**IPAM Dashboard** 展示 IP 分配和释放的请求数、延迟分布、IPPool 可用 IP 统计，以及分配失败、重试耗尽等错误计数。

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
