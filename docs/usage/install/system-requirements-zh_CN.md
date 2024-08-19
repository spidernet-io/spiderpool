# 安装要求

[**English**](./system-requirements.md) | **简体中文**

## 主机要求

- x86-64 或 arm64 相同
- 使用 ipvlan 做集群 CNI 时，linux 内核版本必须大于 4.2

## Kubernetes 要求

使用 [SpiderSubnet](./../spider-subnet.md) 功能要求 Kubernetes 版本不低于 `v1.21`

## Spiderpool 组件的主机网络端口占用

| 组件                               | 端口/协议 | 描述                  | 配置环境变量                      |
|----------------------------------|---------------|---------------------|-----------------------------|
| daemonset spiderpool-agent       | 5710/tcp      | pod 健康检查端口          | SPIDERPOOL_HEALTH_PORT      |
| daemonset spiderpool-agent       | 5711/tcp      | 指标端口（如果开启了 指标功能）    | SPIDERPOOL_METRIC_HTTP_PORT |
| daemonset spiderpool-agent       | 5712/tcp      | gops 端口（如果开启了debug） | SPIDERPOOL_GOPS_LISTEN_PORT |
| deployment spiderpool-controller | 5720/tcp      | pod 健康检查端口          | SPIDERPOOL_HEALTH_PORT      |
| deployment spiderpool-controller | 5711/tcp      | 指标端口（如果开启了 指标功能）    | SPIDERPOOL_METRIC_HTTP_PORT |
| deployment spiderpool-controller | 5722/tcp      | webhook 端口          | SPIDERPOOL_WEBHOOK_PORT     |
| deployment spiderpool-controller | 5724/tcp      | gops 端口（如果开启了debug） | SPIDERPOOL_GOPS_LISTEN_PORT |

## (可选安装) SR-IOV 组件的主机网络端口占用

| 组件                             | 端口/协议 | 描述           | 配置环境变量 |
|--------------------------------------|---------------|--------------|---------------|
| daemonset network-resources-injector | 5731/tcp      |  webhook 端口，该端口会占用 该组件是可选安装 | NA            |
| deployment operator-webhook          | 5732/tcp      | webhook 端口，该组件是可选安装 | NA            |
