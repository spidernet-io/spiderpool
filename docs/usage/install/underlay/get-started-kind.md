# Kind Quick Start

**English** | [**简体中文**](./get-started-kind-zh_CN.md)

kind is a tool for running local Kubernetes clusters using Docker container “nodes”. Spiderpool provides a script for installing a Kind cluster, which allows you to quickly build a set of Kind clusters with Macvlan as the main CNI and Multus and Spiderpool, which you can use to test and experience Spiderpool.

## Prerequisites

* [Go](https://go.dev/) has already been installed.

## Deploying Spiderpool on a Kind cluster

1. Clone the Spiderpool code repository to the local host and go to the root directory of the Spiderpool project.

    ```bash
    ~# git clone https://github.com/spidernet-io/spiderpool.git && cd spiderpool
    ```

2. Execute `make dev-doctor` to check that the development tools on the local host meet the conditions for deploying a Kind cluster with Spiderpool, and that the components are automatically installed for you if they are missing.

3. Get the latest image of Spiderpool.

    ```bash
    ~# SPIDERPOOL_LATEST_IMAGE_TAG=$(curl -s https://api.github.com/repos/spidernet-io/spiderpool/releases | jq -r '.[].tag_name | select(("^v1.[0-9]*.[0-9]*$"))' | head -n 1)
    ```

4. Execute the following command to create a Kind cluster and install Multus, Macvlan, Spiderpool for you.

    ```bash
    ~# make e2e_init -e E2E_SPIDERPOOL_TAG=$SPIDERPOOL_LATEST_IMAGE_TAG
    ```

    Note: If you are mainland user who is not available to access ghcr.io, you can use the following command to avoid failures in pulling Spiderpool and Multus images.

    ```bash
    ~# make e2e_init -e E2E_SPIDERPOOL_TAG=$SPIDERPOOL_LATEST_IMAGE_TAG -e SPIDERPOOL_REGISTER=ghcr.m.daocloud.io -e E2E_MULTUS_IMAGE_REGISTER=ghcr.m.daocloud.io
    ```

## Check that everything is working

Execute the following command in the root directory of the Spiderpool project to configure KUBECONFIG for the Kind cluster for kubectl.

```bash
~# export KUBECONFIG=$(pwd)/test/.cluster/spider/.kube/config
```

It should be possible to observe the following:

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

The Quick Install Kind Cluster script provided by Spiderpool will automatically create an application for you to verify that your Kind cluster is working properly and the following is the running state of the application:

```bash
~# kubectl get po -l app=test-pod -o wide
NAME                       READY   STATUS    RESTARTS   AGE     IP             NODE            NOMINATED NODE   READINESS GATES
test-pod-856f9689d-876nm   1/1     Running   0          5m34s   172.18.40.63   spider-worker   <none>           <none>
```

You can also manually create an application to verify that the cluster is available, the following command will create 1 copy of Deployment:

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

As tested, everything works fine with the Kind cluster. You can test and experience more features of Spiderpool based on kind clusters, see [Quick start](./docs/usage/install.md).

## Uninstall

* Uninstall a Kind cluster

    Execute `make clean` to uninstall the Kind cluster.

* Delete test's images

    ```bash
    ~# docker rmi -f $(docker images | grep spiderpool | awk '{print $3}') 
    ~# docker rmi -f $(docker images | grep multus | awk '{print $3}')
    ```
