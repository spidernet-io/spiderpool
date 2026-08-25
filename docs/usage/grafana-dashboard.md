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

**IPAM Dashboard** displays IP allocation and release request counts, latency distribution, IPPool available IP statistics, and error counts for allocation failures and retry exhaustions.

The IPAM Dashboard also contains an **IaaS Pool (prewarm / global)** row for clusters using IaaS-managed pools:

| Panel | Description |
|-------|-------------|
| IaaS allocation rate by path | Allocation rate split by `cache_hit` / `cold_create` / `cold_rebind` / `cold_steal` |
| IaaS allocation cache hit ratio | Share of allocations served from prewarmed metadata without a provider RPC |
| IaaS allocation duration P95 by path | End-to-end allocation latency; cold paths include the provider Allocate RPC |
| IaaS allocation failures by reason | Failure rate by `no_ready_ip` / `pair_conflict` / `metadata_not_ready` / `rpc_error` / `internal` |
| IaaS provider RPC (client-side) | Provider allocate/release RPC latency P95 and failure rate as seen by the agent |
| IaaS claim rollbacks & metadata decode failures | Cold-path claim rollbacks and pool metadata decode failures; both should stay at zero |

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
