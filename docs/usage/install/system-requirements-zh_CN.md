# 安装要求

[**English**](./system-requirements.md) | **简体中文**

## 主机要求

- x86-64, arm64
- 使用 ipvlan 做集群 CNI 时，系统内核版本必须大于 4.2

## Kubernetes 要求

我们已在以下版本的 Kubernetes 中测试使用了 Spiderpool:

- v1.22.7
- v1.23.5
- v1.24.4
- v1.25.3
- v1.26.2
- v1.27.1
- v1.28.0

使用 [SpiderSubnet](./../spider-subnet.md) 功能要求 Kubernetes 版本不低于 `v1.21`

## 网络端口要求

| 环境变量配置                      | 端口/协议    | 解释                                                 | 是否必填     |
|-----------------------------|----------|----------------------------------------------------|----------|
| SPIDERPOOL_HEALTH_PORT      | 5710/tcp | `spiderpool-agent` pod 健康检查端口号,服务于 Kubernetes      | 必填       |
| SPIDERPOOL_METRIC_HTTP_PORT | 5711/tcp | `spiderpool-agent` 指标端口号                           | 可选(默认关闭) |
| SPIDERPOOL_GOPS_LISTEN_PORT | 5712/tcp | `spiderpool-agent` gops 端口号用于 debug                | 可选(默认启动) |
| SPIDERPOOL_HEALTH_PORT      | 5720/tcp | `spiderpool-controller` pod 健康检查端口号,服务于 Kubernetes | 必填       |
| SPIDERPOOL_METRIC_HTTP_PORT | 5711/tcp | `spiderpool-controller` 指标端口号                      | 可选(默认关闭) |
| SPIDERPOOL_WEBHOOK_PORT     | 5722/tcp | `spiderpool-controller` webhook 端口号,服务于 Kubernetes | 必填       |
| SPIDERPOOL_GOPS_LISTEN_PORT | 5724/tcp | `spiderpool-controller` gops 端口号用于 debug           | 可选(默认启动) |
