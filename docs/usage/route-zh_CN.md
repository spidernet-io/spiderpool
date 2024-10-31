# 路由支持

**简体中文** ｜ [**English**](./route.md)

## 介绍

Spiderpool 提供了为 Pod 配置路由信息的功能。

### 搭配网关配置默认路由

为 SpiderIPPool 资源设置**网关地址**(`spec.gateway`)后，我们会根据该网关地址为 Pod 生成一条默认路由：

```yaml
apiVersion: spiderpool.spidernet.io/v2beta1
kind: SpiderIPPool
metadata:
  name: ipv4-ippool-route
spec:
  subnet: 172.18.41.0/24
  ips:
    - 172.18.41.51-172.18.41.60
  gateway: 172.18.41.0
```

### 继承 IP 池路由

我们也可为 SpiderIPPool 资源配置路由(`spec.routes`)，创建 Pod 时会继承该路由：

> - 当 SpiderIPPool 资源配置了网关地址后，请勿为路由字段配置默认路由。
> - `dst` 和 `gw` 字段都为必填

```yaml
apiVersion: spiderpool.spidernet.io/v2beta1
kind: SpiderIPPool
metadata:
  name: ipv4-ippool-route
spec:
  subnet: 172.18.41.0/24
  ips:
    - 172.18.41.51-172.18.41.60
  gateway: 172.18.41.0
  routes:
    - dst: 172.18.42.0/24
      gw: 172.18.41.1
```

### 自定义路由

我们也支持为应用配置自定义路由的功能，只需为 Pod 打上注解 `ipam.spidernet.io/routes`:

> - 当 SpiderIPPool 资源中配置了网关地址、或配置了默认路由后，请勿为 Pod 配置默认路由。
> - `dst` 和 `gw` 字段都为必填

```yaml
ipam.spidernet.io/routes: |-
  [{
      "dst": "10.0.0.0/16",
      "gw": "192.168.1.1"
  },{
      "dst": "172.10.40.0/24",
      "gw": "172.18.40.1"
  }]
```
