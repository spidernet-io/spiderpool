# Reserved IP

**简体中文** | [**English**](./reserved-ip.md)

## 介绍

*Spiderpool 通过 ReservedIP CR 为整个 Kubernetes 集群保留一些 IP 地址，这些 IP 地址将不会被 IPAM 分配。*

## Reserved IP 功能

当明确某个 IP 地址已经被集群外部使用时，为了避免 IP 冲突，从存量的 IPPool 实例找到该 IP 地址并剔除，也许是一件耗时耗力的工作。并且，网络管理员希望存量或者未来的所有 IPPool 资源中，都不会分配出该 IP 地址。因此，可在 ReservedIP CR 中设置希望不被集群所使用的 IP 地址，设置后，即使是在 IPPool 对象中包含了该 IP 地址，IPAM 插件也不会把这些 IP 地址分配给 Pod 使用。

ReservedIP 中的 IP 地址可以是：

- 明确该 IP 地址被集群外部主机使用

- 明确该 IP 地址不能被使用于网络通信，例如子网 IP、广播 IP 等

## 实施要求

1. 一套 Kubernetes 集群。

2. 已安装 [Helm](https://helm.sh/docs/intro/install/)。

## 步骤

### 安装 Spiderpool

可参考 [安装](./readme-zh_CN.md) 安装 Spiderpool.

### 安装 CNI 配置

Spiderpool 为简化书写 JSON 格式的 Multus CNI 配置，它提供了 SpiderMultusConfig CR 来自动管理 Multus NetworkAttachmentDefinition CR。如下是创建 Macvlan SpiderMultusConfig 配置的示例：

- master：在此示例用接口 `ens192` 作为 master 的参数。

```bash
MACVLAN_MASTER_INTERFACE="ens192"
MACVLAN_MULTUS_NAME="macvlan-$MACVLAN_MASTER_INTERFACE"

cat <<EOF | kubectl apply -f -
apiVersion: spiderpool.spidernet.io/v2beta1
kind: SpiderMultusConfig
metadata:
  name: ${MACVLAN_MULTUS_NAME}
  namespace: kube-system
spec:
  cniType: macvlan
  enableCoordinator: true
  macvlan:
    master:
    - ${MACVLAN_MASTER_INTERFACE}
EOF
```

在本文示例中，使用如上配置，创建如下的 Macvlan SpiderMultusConfig，将基于它自动生成的 Multus NetworkAttachmentDefinition CR。

```bash
~# kubectl get spidermultusconfigs.spiderpool.spidernet.io -n kube-system
NAME             AGE
macvlan-ens192   26m

~# kubectl get network-attachment-definitions.k8s.cni.cncf.io -n kube-system
NAME             AGE
macvlan-ens192   27m
```

### 创建 ReservedIP

使用如下的 Yaml，指定 `spec.ips` 为 `10.6.168.131-10.6.168.132` ，并创建 ReservedIP。

```bash
cat <<EOF | kubectl apply -f -
apiVersion: spiderpool.spidernet.io/v2beta1
kind: SpiderReservedIP
metadata:
  name: test-reservedip
spec:
  ips:
  - 10.6.168.131-10.6.168.132
EOF
```

### 创建 IPPool

创建一个 `spec.ips` 为 `10.6.168.131-10.6.168.133` ，共计 3 个 IP 地址的 IPPool 。通过与上述的 ReservedIP 对比可知，该 IP 池只有 1 个 IP 可用。

```bash
cat <<EOF | kubectl apply -f -
apiVersion: spiderpool.spidernet.io/v2beta1
kind: SpiderIPPool
metadata:
  name: test-ippool
spec:
  subnet: 10.6.0.0/16
  ips:
  - 10.6.168.131-10.6.168.133
EOF
```

使用如下的 Yaml ，创建一个具有 2 个副本的 Deployment，并从上面的 IPPool 中分配 IP 地址。

- `ipam.spidernet.io/ippool`：用于指定为应用分配 IP 地址的 IP 池。

```shell
cat <<EOF | kubectl create -f -
apiVersion: apps/v1
kind: Deployment
metadata:
  name: test-app
spec:
  replicas: 2
  selector:
    matchLabels:
      app: test-app
  template:
    metadata:
      annotations:
        ipam.spidernet.io/ippool: |-
            {      
              "ipv4": ["test-ippool"]
            }
        v1.multus-cni.io/default-network: kube-system/macvlan-ens192
      labels:
        app: test-app
    spec:
      containers:
      - name: test-app
        image: nginx
        imagePullPolicy: IfNotPresent
        ports:
        - name: http
          containerPort: 80
          protocol: TCP
EOF
```

因为 IP 池中的两个 IP 被 ReservedIP CR 所保留，IP 池中只有一个 IP 可用。应用只会有一个 Pod 可以成功运行，另一个 Pod 由于 "所有 IP 都已用完" 而创建失败。

```bash
~# kubectl get po -owide
NAME                       READY   STATUS              RESTARTS   AGE   IP             NODE    NOMINATED NODE   READINESS GATES
test-app-67dd9f645-dv8xz   1/1     Running             0          17s   10.6.168.133   node2   <none>           <none>
test-app-67dd9f645-lpjgs   0/1     ContainerCreating   0          17s   <none>         node1   <none>           <none>
```

如果应用的 Pod 已经分配了要保留的 IP 地址，将该 IP 地址添加到 ReservedIP CR 中，当应用副本重启后，副本将无法运行。通过下面的命令，将 Pod 所分配的 IP 地址加入到 ReservedIP CR 中，然后重启 Pod ，Pod 因"所有 IP 都已用完"而启动失败，符合预期。

```bash
~# kubectl patch spiderreservedip test-reservedip --patch '{"spec":{"ips":["10.6.168.131-10.6.168.133"]}}' --type=merge

～# kubectl delete po test-app-67dd9f645-dv8xz 
pod "test-app-67dd9f645-dv8xz" deleted

~# kubectl get po -owide
NAME                       READY   STATUS              RESTARTS   AGE     IP       NODE    NOMINATED NODE   READINESS GATES
test-app-67dd9f645-fvx4m   0/1     ContainerCreating   0          9s      <none>   node2   <none>           <none>
test-app-67dd9f645-lpjgs   0/1     ContainerCreating   0          2m18s   <none>   node1   <none>           <none>
```

保留 IP 被移除后，Pod 可以获得 IP 地址并运行。

```bash
~# kubectl delete sr test-reservedip
spiderreservedip.spiderpool.spidernet.io "test-reservedip" deleted

~# kubectl get po -owide
NAME                       READY   STATUS    RESTARTS   AGE     IP             NODE    NOMINATED NODE   READINESS GATES
test-app-67dd9f645-fvx4m   1/1     Running   0          4m23s   10.6.168.133   node2   <none>           <none>
test-app-67dd9f645-lpjgs   1/1     Running   0          6m14s   10.6.168.131   node1   <none>           <none>
```

## 总结

SpiderReservedIP 功能可以帮助基础设施管理员更加容易的进行网络规划。
