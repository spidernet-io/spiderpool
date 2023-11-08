# Route support

**English** ｜ [**简体中文**](./route-zh_CN.md)

## Introduction

Spiderpool supports the configuration of routing information for Pods.

### Configure Default Route with Gateway

When setting the **gateway address** (`spec.gateway`) for a SpiderIPPool resource, a default route will be generated for Pods based on that gateway address:

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

### Inherit IP Pool Routes

SpiderIPPool resources also support configuring routes (`spec.routes`),  which will be inherited by Pods during their creation process:

> - If a gateway address is configured for the SpiderIPPool resource, avoid setting default routes in the routes field.
> - Both `dst` and `gw` fields are required.

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

### Customize Routes

You can customize routes for Pods by adding the annotation `ipam.spidernet.io/routes`:

> - When a gateway address or default route is configured in the SpiderIPPool resource, avoid configuring default routes for Pods.
> - Both `dst` and `gw` fields are required.

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
