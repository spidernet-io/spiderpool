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
- 若遇到报错 `failed to get IPPool candidates from Subnet: no matching auto-created IPPool candidate with matchLables`，请检查 `spiderpool-controller` 的日志。目前 Spiderpool 的 controller 组件要求使用 SpiderSubnet 功能的集群最低版本为 `v1.21`, 如遇到以下日志报错即表明当前集群版本过低:

    ```text
    W1220 05:44:16.129916       1 reflector.go:535] k8s.io/client-go/informers/factory.go:150: failed to list *v1.CronJob: the server could not find the requested resource
    E1220 05:44:16.129978       1 reflector.go:147] k8s.io/client-go/informers/factory.go:150: Failed to watch *v1.CronJob: failed to list *v1.CronJob: the server could not find the requested resource
    ```

### Spiderpool IPAM 是否依赖 spiderpool-controller 组件？

spiderpool-controller 组件针对 SpiderSubnet、 SpiderIPPool 等资源的 `Spec` 字段实现了 Webhook 功能。而 spiderpool-agent 组件是 IPAM 功能实现的核心部分，在分配 IP 的时候会对 SpiderIPPool 资源的 `Status` 字段进行修改，该字段属于 [subresource](https://kubernetes.io/docs/tasks/extend-kubernetes/custom-resources/custom-resource-definitions/#subresources)，不会被 spiderpool-controller 所注册的 Webhook 拦截到，所以 IPAM 不会依赖 spiderpool-controller 组件。
