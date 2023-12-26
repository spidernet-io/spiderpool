# System requirements

**English** | [**简体中文**](./system-requirements-zh_CN.md)

## Node requirements

- x86-64, arm64
- The system kernel version must be greater than 4.2 when using `ipvlan` as the cluster's CNI

## Kubernetes requirements

We test Spiderpool against the following Kubernetes versions：

- v1.22.7
- v1.23.5
- v1.24.4
- v1.25.3
- v1.26.2
- v1.27.1
- v1.28.0

The [SpiderSubnet](./../spider-subnet.md) feature requires a minimum version of `v1.21`.

## Network Ports requirements

| ENV Configuration           | Port/Protocol | Description                                                  | Is Optional               |
|-----------------------------|---------------|--------------------------------------------------------------|---------------------------|
| SPIDERPOOL_HEALTH_PORT      | 5710/tcp      | `spiderpool-agent` pod health check port for kubernetes      | must                      |
| SPIDERPOOL_METRIC_HTTP_PORT | 5711/tcp      | `spiderpool-agent` metrics port                              | optional(default disable) |
| SPIDERPOOL_GOPS_LISTEN_PORT | 5712/tcp      | `spiderpool-agent` gops port for debug                       | optional(default enable)  |
| SPIDERPOOL_HEALTH_PORT      | 5720/tcp      | `spiderpool-controller` pod health check port for kubernetes | must                      |
| SPIDERPOOL_METRIC_HTTP_PORT | 5711/tcp      | `spiderpool-controller` metrics port for openTelemetry       | optional(default disable) |
| SPIDERPOOL_WEBHOOK_PORT     | 5722/tcp      | `spiderpool-controller` webhook port for kubernetes          | must                      |
| SPIDERPOOL_GOPS_LISTEN_PORT | 5724/tcp      | `spiderpool-controller` gops port for debug                  | optional(default enable)  |
