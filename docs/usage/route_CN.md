# 路由功能

## 介绍

SpiderPool作为一款IPAM插件遵循CNI协议实现了路由定义功能并将配置的路由返回给Main CNI使其生效至容器Namespace内。我们有以下 `自定义路由` 以及 `继承IPPool` 路由两种方式 **叠加一起** 实现路由功能.

### 自定义路由

可以在创建Pod时通过Annotations来指定需要配置的路由，如下所示:

```yaml
apiVersion: v1
kind: Pod
metadata:
  name: dummy
  annotations:
    ipam.spidernet.io/ippool: |-
      {
        "ipv4": ["ipv4-ippool"]
      }
    ipam.spidernet.io/routes: |-
      [{
        "dst": "192.168.0.101/24",
        "gw": "10.16.0.254"
      }]'
spec:
  containers:
    - name: dummy
      image: busybox
      imagePullPolicy: IfNotPresent
      command: ["/bin/sh", "-c", "trap : TERM INT; sleep infinity & wait"]
```

> 注意：若该Pod使用的IPPool中的gateway字段不为空，我们会根据该gateway自动为其补充一条默认路由。因此请不要在Annotations里额外指定一条默认路由。

### 继承IPPool路由

[SpiderIPPool CRD](./../reference/crd-spiderippool.md) 中有一个名为 [route](./../reference/crd-spiderippool.md#Route) 的属性。Pod会继承某个为其分配IP地址的IPPool里的route内容作为它的路由条目。

```yaml
apiVersion: spiderpool.spidernet.io/v2beta1
kind: SpiderIPPool
metadata:
  name: ipv4-ippool
spec:
  subnet: 172.18.41.0/24
  ips:
    - 172.18.41.51-172.18.41.60
  gateway: 172.18.41.0
  routes:
    - dst: 172.18.42.0/24
      gw: 172.18.41.1
```

> 注意：在使用 [自动池](./spider-subnet.md) 功能时, 自动创建的IPPool会继承其父亲SpiderSubnet中的route属性。

## 注意

以上两种方式的路由会叠加一起生效，请不要指定 **重复的** 路由条目。
