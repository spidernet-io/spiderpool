# SpiderIPPool

**简体中文** | [**English**](./spider-ippool.md)

## 介绍

SpiderIPPool 资源代表 Spiderpool 为 Pod 分配 IP 的 IP 地址范围。 请参照 [SpiderIPPool CRD](./../reference/crd-spiderippool.md) 为你的集群创建 SpiderIPPool 资源。

## SpiderIPPool 功能

- 单双栈以及 IPv6 支持
- IP 地址范围控制
- 网关路由控制
- 仅用以及全局缺省池控制
- 搭配各种资源亲和性使用控制

## 使用介绍

### 单双栈控制

Spiderpool 支持 IPv4-only, IPv6-only, 双栈这三种 IP 地址分配方式，可通过 [configmap](./../reference/configmap.md) 配置来控制。

> 通过 Helm 安装时可配置参数来指定： `--set ipam.enableIPv4=true --set ipam.enableIPv6=true`。

当我们 Spiderpool 环境开启双栈配置后，我们可以手动指定使用哪些 IPv4 和 IPv6 池来分配 IP 地址：

> 在双栈环境下，你也可为pod只分配IPv4/IPv6的IP，如: `ipam.spidernet.io/ippool: '{"ipv4": ["custom-ipv4-ippool"]}'`

```yaml
apiVersion: apps/v1
kind: Deployment
metadata:
  name: custom-dual-ippool-deploy
spec:
  replicas: 3
  selector:
    matchLabels:
      app: custom-dual-ippool-deploy
  template:
    metadata:
      annotations:
        ipam.spidernet.io/ippool: |-
          {
            "ipv4": ["custom-ipv4-ippool"],"ipv6": ["custom-ipv6-ippool"]
          }
      labels:
        app: custom-dual-ippool-deploy
    spec:
      containers:
        - name: custom-dual-ippool-deploy
          image: busybox
          imagePullPolicy: IfNotPresent
          command: ["/bin/sh", "-c", "trap : TERM INT; sleep infinity & wait"]
```


### 指定 IPPool 为应用分配 IP 地址

> 对于以下指定使用 SpiderIPPool 规则的优先级，请参考 [IP 候选池规则](./../concepts/ipam-zh_CN.md#获取候选池)

#### 使用 Pod Annotation 指定使用IP池

我们可借助注解 `ipam.spidernet.io/ippool` 或 `ipam.spidernet.io/ippools` 标记在 Pod 的 Annotation上来指定 Pod 使用哪些 IP 池, 注解 `ipam.spidernet.io/ippools` 多用于多网卡指定。此外我们可以指定多个 IP 池以供备选，当某个池的 IP 被用完后，可继续从你指定的其他池中分配地址。

```yaml
ipam.spidernet.io/ippool: |-
  {
    "ipv4": ["demo-v4-ippool1", "backup-ipv4-ippool"],
    "ipv6": ["demo-v6-ippool1", "backup-ipv6-ippool"]
  }
```

```yaml
ipam.spidernet.io/ippools: |-
  [{
      "interface": "eth0",
      "ipv4": ["demo-v4-ippool1"],
      "ipv6": ["demo-v6-ippool1"],
      "cleangateway": true
   },{
      "interface": "net1",
      "ipv4": ["demo-v4-ippool2"],
      "ipv6": ["demo-v6-ippool2"],
      "cleangateway": false
  }]
```

#### 使用 Namespace 注解指定池

我们可以为 Namespace 打上注解 `ipam.spidernet.io/default-ipv4-ippool` 和 `ipam.spidernet.io/default-ipv6-ippool`, 当应用部署的时，可从应用所在 Namespace 的注解中选择 IP 池使用：

> 注意：未使用 Pod Annotation 指定使用IP池时，优先使用此处 Namespace 注解规则。

```yaml

apiVersion: v1
kind: Namespace
metadata:
  annotations:
    ipam.spidernet.io/default-ipv4-ippool: '["ns-v4-ippool1","ns-v4-ippool2"]'
    ipam.spidernet.io/default-ipv6-ippool: '["ns-v6-ippool1","ns-v6-ippool2"]'
  name: kube-system
...
```

#### 使用 CNI 配置文件指定池

我们可以在 CNI 配置文件中，指定缺省的 IPv4 和 IPv6 池以供应用选择该 CNI 配置时使用，具体可参照 [CNI配置](./../reference/plugin-ipam.md)

> 注意：未使用 Pod Annotation 指定使用IP池，且没有通过 Namespace 注解指定 IP 池时，将优先使用此处 CNI 配置文件指定池规则。

```yaml
{
  "name": "macvlan-vlan0",
  "type": "macvlan",
  "master": "eth0",
  "ipam": {
    "type": "spiderpool",
    "default_ipv4_ippool":["default-v4-ippool","backup-ipv4-ippool"],
    "default_ipv6_ippool":["default-v6-ippool","backup-ipv6-ippool"]
    }
}
```

#### 为 SpiderIPPool 设置集群默认级别

在 [SpiderIPPool CRD](./../reference/crd-spiderippool.md) 中我们可以看到 `spec.default` 字段是一个 bool 类型，当我们没有通过 Annotation 或 CNI 配置文件指定 IPPool 时，系统会根据该字段挑选出集群默认池使用:

> 注意：
>
> - 未使用 Pod Annotation 指定使用IP池，没有通过 Namespace 注解指定 IP 池时，且未在 CNI 配置文件中指定 IP 池时，此处会生效。
> - 可为多个 IPPool 资源设置为集群默认级别。 

```yaml
apiVersion: spiderpool.spidernet.io/v2beta1
kind: SpiderIPPool
metadata:
  name: master-172
spec:
  default: true
...
```

### SpiderIPPool 搭配亲和性使用

具体请参考 [IP 池亲和性搭配](./spider-affinity-zh_CN.md)

### SpiderIPPool 网关与路由配置

具体请参考 [路由功能](./route-zh_CN.md)

因此 Pod 会拿到基于网关的默认路由，以及此 IP 池上的自定义路由。(若 IP 池不设置网关，则不会生效默认路由)

### 命令行工作(kubectl)查看扩展字段

为了更简单方便的查看 SpiderIPPool 资源的相关属性，我们补充了一些扩展字段可让用户通过 `kubectl get sp -o wide` 查看:

- `ALLOCATED-IP-COUNT` 字段表示该池已分配的 IP 数量
- `TOTAL-IP-COUNT` 字段表示该池的总 IP 数量
- `DEFAULT` 字段表示该池是否为集群默认级别
- `DISABLE` 字段表示该池是否被禁用
- `NODENAME` 字段表示与该池亲和的节点
- `MULTUSNAME` 字段表示与该池亲和的 multus 实例
- `APP-NAMESPACE` 字段属于 [SpiderSubnet](./spider-subnet-zh_CN.md) 功能独有，表明该池是一个系统自动创建的池，同时该字段表明其对应应用的命名空间。

```shell
~# kubectl get sp -o wide  
NAME                                  VERSION   SUBNET                    ALLOCATED-IP-COUNT   TOTAL-IP-COUNT   DEFAULT   DISABLE   NODENAME               MULTUSNAME                      APP-NAMESPACE
auto4-demo-deploy-subnet-eth0-fcca4   4         172.100.0.0/16            1                    2                false     false                                                            kube-system
test-pod-ippool                       4         10.6.0.0/16               0                    10               false     false     ["master","worker1"]   ["kube-system/macvlan-vlan0"]   
```

### 指标(metric)

我们也为 SpiderIPPool 资源补充了相关的指标信息，详情请看 [metric](./../reference/metrics.md)
