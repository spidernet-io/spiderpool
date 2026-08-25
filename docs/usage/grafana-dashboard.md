# Grafana Dashboard

**English** | [**简体中文**](./grafana-dashboard-zh_CN.md)

Spiderpool ships with built-in Grafana Dashboards for visualizing IPAM and RDMA metrics.

## Prerequisites

- [Grafana Operator](https://github.com/grafana-operator/grafana-operator) (manages Dashboard CRDs)
- Prometheus
- Spiderpool Metrics enabled

## Installation

### Via Helm

Enable Dashboard when installing Spiderpool:

```bash
helm install spiderpool spidernet-io/spiderpool \
  -n kube-system \
  --set grafanaDashboard.install=true \
  --set spiderpoolAgent.prometheus.enabled=true \
  --set spiderpoolController.prometheus.enabled=true
```

Specify Dashboard namespace:

```bash
helm install spiderpool spidernet-io/spiderpool \
  -n kube-system \
  --set grafanaDashboard.install=true \
  --set grafanaDashboard.namespace=monitoring
```

### Manual Import

Dashboard JSON files are located in `charts/spiderpool/files/`:

| File | Description |
|------|-------------|
| `grafana-ipam.json` | IPAM metrics |
| `grafana-rdma-pod.json` | Pod-level RDMA metrics |
| `grafana-rdma-node.json` | Node-level RDMA metrics |
| `grafana-rdma-cluster.json` | Cluster-level RDMA metrics |
| `grafana-rdma-workload.json` | Workload-level RDMA metrics |

## Helm Parameters

| Parameter | Description | Default |
|-----------|-------------|---------|
| `grafanaDashboard.install` | Install Dashboard, requires Grafana Operator CRDs | `false` |
| `grafanaDashboard.namespace` | Dashboard namespace, defaults to Helm release namespace | `""` |
| `grafanaDashboard.annotations` | Additional annotations | `{}` |
| `grafanaDashboard.labels` | Additional labels | `{}` |

## Dashboard Overview

**IPAM Dashboard** is organized in three rows:

**IPAM Overview** — headline stats:

| Panel | Description |
|-------|-------------|
| Total IPPools / Total Subnets | Current number of SpiderIPPool and SpiderSubnet resources |
| Allocation Rate / Release Rate | Current IPAM allocation and release request rate |

**IPAM Allocation / Release** — core IPAM behavior:

| Panel | Description |
|-------|-------------|
| Allocation & Release Rate | IPAM allocation and release request rate served by spiderpool-agent |
| IPAM Failures & GC | Allocation/release failure rate and IP GC activity; failure lines should stay at zero |
| IP Allocation Latency | End-to-end allocation latency P50/P95/P99 |
| IP Release Latency | End-to-end release latency P50/P95/P99 |

**IaaS Network Provider** — for clusters using IaaS-managed pools:

| Panel | Description |
|-------|-------------|
| IaaS Allocation Rate by Path | Allocation rate split by mode (`node`/`global`) and path (`cache_hit` = prewarm reuse, `cold_create` = realtime cloud call) |
| IaaS Cache Hit Ratio | Share of allocations served from prewarmed metadata without a provider RPC |
| IaaS Allocation Latency by Path | Allocation latency P50/P95; `cache_hit` should be milliseconds, `cold_create` includes the cloud sub-ENI creation |
| Provider API Request Rate | Client-side request rate from spiderpool-agent to the provider, by operation (`allocate`/`release`) |
| Provider API Latency (client-side) | Latency P50/P95 of agent-to-provider calls, measured at the client |
| IaaS Failures & Anomalies | All IaaS failure counters by reason (allocation failures, RPC failures, claim rollbacks, metadata decode failures); a healthy cluster only shows the zero baseline |

**RDMA Dashboard** presents RDMA network metrics at different granularities:

| Dashboard | Granularity | Example Metrics |
|-----------|-------------|-----------------|
| RDMA Pod | Pod | Read/write requests, error counts, CNP packets |
| RDMA Node | Node | RDMA device status, port speed |
| RDMA Cluster | Cluster | RDMA resource overview |
| RDMA Workload | Workload | Deployment/StatefulSet RDMA usage |

## Enable RDMA Metrics

RDMA Dashboard requires RDMA metrics collection:

```bash
helm install spiderpool spidernet-io/spiderpool \
  -n kube-system \
  --set grafanaDashboard.install=true \
  --set spiderpoolAgent.prometheus.enabled=true \
  --set spiderpoolAgent.prometheus.enabledRdmaMetric=true
```

## Troubleshooting

### Dashboard shows no data

Verify Prometheus is scraping spiderpool-agent and spiderpool-controller metrics. Check if ServiceMonitor is created. Then verify the Metrics environment variable:

```bash
kubectl get pods -n kube-system -l app.kubernetes.io/component=spiderpool-agent \
  -o jsonpath='{.items[0].spec.containers[0].env[?(@.name=="SPIDERPOOL_ENABLED_METRIC")].value}'
```

### RDMA metrics show no data

Confirm nodes have RDMA devices and `spiderpoolAgent.prometheus.enabledRdmaMetric` is set to `true`.
