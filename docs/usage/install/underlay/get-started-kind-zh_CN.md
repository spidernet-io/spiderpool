# Kind Quick Start

[**English**](./get-started-kind.md) | **简体中文**

Kind 是一个使用 Docker 容器节点运行本地 Kubernetes 集群的工具。Spiderpool 提供了安装 Kind 集群的脚本，能快速搭建一套以 Macvlan 为 main CNI 搭配 Multus、Spiderpool 的 Kind 集群，您可以使用它来进行 Spiderpool 的测试与体验。

## 先决条件

* 已安装 [Go](https://go.dev/)

## Kind 上集群部署 Spiderpool

1. 克隆 Spiderpool 代码仓库到本地主机上，并进入 Spiderpool 工程的根目录。
  
    ```bash
    git clone https://github.com/spidernet-io/spiderpool.git && cd spiderpool
    ```

2. 执行 `make dev-doctor`，检查本地主机上的开发工具是否满足部署 Kind 集群与 Spiderpool 的条件，如果缺少组件会为您自动安装。

3. 通过以下方式获取 Spiderpool 的最新镜像。

    ```bash
    ~# SPIDERPOOL_LATEST_IMAGE_TAG=$(curl -s https://api.github.com/repos/spidernet-io/spiderpool/releases | jq -r '.[].tag_name | select(("^v1.[0-9]*.[0-9]*$"))' | head -n 1)
    ```

4. 执行以下命令，创建 Kind 集群，并为您安装 Multus、Macvlan、Spiderpool。

    ```bash
    ~# make e2e_init -e E2E_SPIDERPOOL_TAG=$SPIDERPOOL_LATEST_IMAGE_TAG
    ```

    注意：如果您是国内用户，您可以使用如下命令，避免拉取 Spiderpool 与 Multus 镜像失败。

    ```bash
    ~# make e2e_init -e E2E_SPIDERPOOL_TAG=$SPIDERPOOL_LATEST_IMAGE_TAG -e SPIDERPOOL_REGISTER=ghcr.m.daocloud.io -e E2E_MULTUS_IMAGE_REGISTER=ghcr.m.daocloud.io
    ```

## 验证安装

在 Spiderpool 工程的根目录下执行如下命令，为 kubectl 配置 Kind 集群的 KUBECONFIG。

```bash
~# export KUBECONFIG=$(pwd)/test/.cluster/spider/.kube/config
```

您可以看到如下的内容输出：

```bash
~# kubectl get nodes 
NAME                   STATUS   ROLES           AGE     VERSION
spider-control-plane   Ready    control-plane   2m29s   v1.26.2
spider-worker          Ready    <none>          2m58s   v1.26.2

~# kubectll get po -n kube-sysem | grep spiderpool
spiderpool-agent-fmx74                         1/1     Running   0               4m26s
spiderpool-agent-jzfh8                         1/1     Running   0               4m26s
spiderpool-controller-79fcd4d75f-n9kmd         1/1     Running   0               4m25s
spiderpool-controller-79fcd4d75f-scw2v         1/1     Running   0               4m25s
spiderpool-init                                1/1     Running   0               4m26s

~# kubectl get spidersubnet
NAME                VERSION   SUBNET                    ALLOCATED-IP-COUNT   TOTAL-IP-COUNT
default-v4-subnet   4         172.18.0.0/16             253                  253
default-v6-subnet   6         fc00:f853:ccd:e793::/64   253                  253

~# kubectl get spiderippool
NAME                VERSION   SUBNET                    ALLOCATED-IP-COUNT   TOTAL-IP-COUNT   DEFAULT   DISABLE
default-v4-ippool   4         172.18.0.0/16             5                    253              true      false
default-v6-ippool   6         fc00:f853:ccd:e793::/64   5                    253              true      false
```

Spiderpool 提供的快速安装 Kind 集群脚本会自动为您创建一个应用，以验证您的 Kind 集群是否能够正常工作，以下是应用的运行状态：

```bash
~# kubectl get po -l app=test-pod -o wide
NAME                       READY   STATUS    RESTARTS   AGE     IP             NODE            NOMINATED NODE   READINESS GATES
test-pod-856f9689d-876nm   1/1     Running   0          5m34s   172.18.40.63   spider-worker   <none>           <none>
```

您也可以手动创建应用验证 Kind 集群是否能够正常工作，以下命令会创建 1 个副本 Deployment：

```bash
cat <<EOF | kubectl create -f -
apiVersion: apps/v1
kind: Deployment
metadata:
  name: test-app
spec:
  replicas: 1
  selector:
    matchLabels:
      app: test-app
  template:
    metadata:
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

```bash
~# kubectl get po -l app=test-app -o wide
NAME                        READY   STATUS    RESTARTS   AGE     IP              NODE                   NOMINATED NODE   READINESS GATES
test-app-84d5699474-dbtl5   1/1     Running   0          6m23s   172.18.40.112   spider-control-plane   <none>           <none>
```

通过测试，Kind 集群一切正常，您可以基于它测试与体验 Spiderpool 的更多功能，请参阅 [快速启动](./docs/usage/install.md)。

## 卸载

* 卸载 Kind 集群

    执行 `make clean` 卸载 Kind 集群。

* 删除测试镜像

    ```bash
    ~# docker rmi -f $(docker images | grep spiderpool | awk '{print $3}') 
    ~# docker rmi -f $(docker images | grep multus | awk '{print $3}')
    ```
