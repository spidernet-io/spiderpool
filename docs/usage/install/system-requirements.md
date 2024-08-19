# System requirements

**English** | [**简体中文**](./system-requirements-zh_CN.md)

## Node requirements

- x86-64, arm64
- The system kernel version must be greater than 4.2 when using `ipvlan` as the cluster's CNI

## Kubernetes requirements

The [SpiderSubnet](./../spider-subnet.md) feature requires a minimum version of `v1.21`.

## Spiderpool Requirements Of Host Ports

| Component                        | Port/Protocol | Description                        | ENV Configuration           |
|----------------------------------|---------------|------------------------------------|-----------------------------|
| daemonset spiderpool-agent       | 5710/tcp      | health-check port                  | SPIDERPOOL_HEALTH_PORT      |
| daemonset spiderpool-agent       | 5711/tcp      | metrics port if metrics is enabled | SPIDERPOOL_METRIC_HTTP_PORT |
| daemonset spiderpool-agent       | 5712/tcp      | gops port if debugging is expected | SPIDERPOOL_GOPS_LISTEN_PORT |
| deployment spiderpool-controller | 5720/tcp      | health-check port                  | SPIDERPOOL_HEALTH_PORT      |
| deployment spiderpool-controller | 5711/tcp      | metrics port if metrics is enabled | SPIDERPOOL_METRIC_HTTP_PORT |
| deployment spiderpool-controller | 5722/tcp      | webhook port                       | SPIDERPOOL_WEBHOOK_PORT     |
| deployment spiderpool-controller | 5724/tcp      | gops port if debugging is expected | SPIDERPOOL_GOPS_LISTEN_PORT |

## (optional components) SR-IOV Requirements Of Host Ports

| Component                             | Port/Protocol | Description  | ENV Configuration |
|--------------------------------------|---------------|--------------|-------------------|
| daemonset network-resources-injector | 5731/tcp      | webhook port | NA                |
| deployment operator-webhook          | 5732/tcp      | webhook port | NA                |
