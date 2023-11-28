# FAQ

经常被问到的问题

[**English**](./faq.md) | **简体中文**

## 整体

### 什么是 Spiderpool ？

Spiderpool 项目由多个子插件项目组成，包括有：`spiderpool`, `coordinator`, `ifacer`. 这里的 `spiderpool` 插件是一款服务于 main CNI 的 IPAM 插件，可为您的集群管理 IP。`coordinator` 插件能为你协同路由。`ifacer` 插件可为你创建 vlan 子接口以及创建 bond 网卡。其中，`coordinator` 和 `ifacer` 插件以 CNI 协议中的链式调用方式来使用，且可选并非强制使用。

## 配置

### 为什么更改 configmap 的配置后却无法生效？

修改 configmap 资源 `spiderpool-conf` 配置后，需要重启 `spiderpool-agent` 和 `spiderpool-controller` 组件。

## 使用

### SpiderSubnet功能使用不正常

- 如果遇到报错 `Internal error occurred: failed calling webhook "spidersubnet.spiderpool.spidernet.io": the server could not find the requested resource`，请检查 configmap `spiderpool-conf` 确保 SpiderSubnet 功能已启动。
- 若遇到报错 `failed to get IPPool candidates from Subnet: no matching auto-created IPPool candidate with matchLables`，请检查 `spiderpool-controller` 的日志。
