# Kind Quick Start

**English** | [**简体中文**](./get-started-kind-zh_CN.md)

Kind is a tool for running local Kubernetes clusters using Docker container "nodes". Spiderpool provides a script to install the Kind cluster, you can use it to deploy a cluster that meets your needs, and test and experience Spiderpool.

## Prerequisites

* [Go](https://go.dev/) has already been installed.

* Clone the Spiderpool code repository to the local host and go to the root directory of the Spiderpool project.

    ```bash
    ~# git clone https://github.com/spidernet-io/spiderpool.git && cd spiderpool
    ```

* Get the latest image tag of Spiderpool.

    ```bash
    ~# SPIDERPOOL_LATEST_IMAGE_TAG=$(curl -s https://api.github.com/repos/spidernet-io/spiderpool/releases | jq -r '.[].tag_name | select(("^v1.[0-9]*.[0-9]*$"))' | head -n 1)
    ```

* Execute `make dev-doctor` to check that the development tools on the local host meet the conditions for deploying a Kind cluster with Spiderpool, and that the components are automatically installed for you if they are missing.

## Various installation modes supported by Spiderpool script

If you are mainland user who is not available to access ghcr.io, Additional parameter `-e E2E_CHINA_IMAGE_REGISTRY=true` can be specified during installation to help you pull images faster.

### Install Spiderpool in Underlay CNI (Macvlan) cluster

  ```bash
  ~# make e2e_init_underlay -e E2E_SPIDERPOOL_TAG=$SPIDERPOOL_LATEST_IMAGE_TAG
  ```

### Install Spiderpool on Calico Overlay CNI cluster

  ```bash
  ~# make e2e_init_overlay_calico -e E2E_SPIDERPOOL_TAG=$SPIDERPOOL_LATEST_IMAGE_TAG
  ```

## Install Spiderpool in Cilium cluster with kube-proxy enabled

* Confirm whether the operating system Kernel version number is >= 4.9.17. If the kernel is too low, the installation will fail. Kernel 5.10+ is recommended.

  ```bash
  ~# make e2e_init_overlay_cilium -e E2E_SPIDERPOOL_TAG=$SPIDERPOOL_LATEST_IMAGE_TAG
  ```

## Installing Spiderpool in Cilium cluster with ebpf enabled

* Confirm whether the operating system Kernel version number is >= 4.9.17. If the kernel is too low, the installation will fail. Kernel 5.10+ is recommended.

  ```bash
  ~# make e2e_init_cilium_with_ebpf -e E2E_SPIDERPOOL_TAG=$SPIDERPOOL_LATEST_IMAGE_TAG
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
NAME                                           READY   STATUS      RESTARTS   AGE                                
spiderpool-agent-4dr97                         1/1     Running     0          3m
spiderpool-agent-4fkm4                         1/1     Running     0          3m
spiderpool-controller-7864477fc7-c5dk4         1/1     Running     0          3m
spiderpool-controller-7864477fc7-wpgjn         1/1     Running     0          3m
spiderpool-init                                0/1     Completed   0          3m

~# kubectl get spiderippool
NAME                VERSION   SUBNET                    ALLOCATED-IP-COUNT   TOTAL-IP-COUNT   DEFAULT
default-v4-ippool   4         172.18.0.0/16             5                    253              true      
default-v6-ippool   6         fc00:f853:ccd:e793::/64   5                    253              true      
vlan100-v4          4         172.100.0.0/16            0                    2559             false
vlan100-v6          6         fd00:172:100::/64         0                    65009            false
vlan100-v4          4         172.200.0.0/16            0                    2559             false
vlan200-v6          6         fd00:172:200::/64         0                    65009            false
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

As tested, everything works fine with the Kind cluster. You can test and experience more features of Spiderpool based on kind clusters.

## Uninstall

* Uninstall a Kind cluster

    Execute `make clean` to uninstall the Kind cluster.

* Delete test's images

    ```bash
    ~# docker rmi -f $(docker images | grep spiderpool | awk '{print $3}')
    ~# docker rmi -f $(docker images | grep multus | awk '{print $3}')
    ```
