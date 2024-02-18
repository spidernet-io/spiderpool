# Weave Quick Start

**English** | [**简体中文**](./get-started-weave-zh_CN.md)

`Weave`, an open-source network solution, provides network connectivity and policies for containers by creating a virtual network, automatically discovering and connecting containers. Also known as a Kubernetes Container Network Interface (CNI) solution, `Weave` utilizes the built-in `IPAM` to allocate IP addresses for Pods by default, with limited visibility and IPAM capabilities for Pods. This page demonstrates how `Weave` and `Spiderpool` can be integrated to extend `Weave`'s IPAM capabilities while preserving its original functions.

## Prerequisites

- [System requirements](./../system-requirements.md)
- A ready Kubernetes cluster without any CNI installed
- Helm, Kubectl and Jq (optional)

## Install

1. Install Weave:

    ```shell
    kubectl apply -f  https://github.com/weaveworks/weave/releases/download/v2.8.1/weave-daemonset-k8s.yaml
    ```

    Wait for Pod Running:

    ```shell
    [root@node1 ~]# kubectl get po -n kube-system  | grep weave
    weave-net-ck849                         2/2     Running     4     0   1m
    weave-net-vhmqx                         2/2     Running     4     0   1m
    ```

2. Install Spiderpool

    ```shell
    helm repo add spiderpool https://spidernet-io.github.io/spiderpool
    helm repo update spiderpool
    helm install spiderpool spiderpool/spiderpool --namespace kube-system --set multus.multusCNI.install=false
    ```

    > If you are a mainland user who is not available to access ghcr.io, you can specify the parameter `-set global.imageRegistryOverride=ghcr.m.daocloud.io` to avoid image pulling failures for Spiderpool.
    >
    > Specify the name of the NetworkAttachmentDefinition instance for the default CNI used by Multus via `multus.multusCNI.defaultCniCRName`. If the `multus.multusCNI.defaultCniCRName` option is provided, an empty NetworkAttachmentDefinition instance will be automatically generated upon installation. Otherwise, Multus will attempt to create a NetworkAttachmentDefinition instance based on the first CNI configuration found in the /etc/cni/net.d directory. If no suitable configuration is found, a NetworkAttachmentDefinition instance named `default` will be created to complete the installation of Multus.

    Wait for Pod Running and create the IPPool used by Pod:

    ```shell
    cat << EOF | kubectl apply -f -
    apiVersion: spiderpool.spidernet.io/v2beta1
    kind: SpiderIPPool
    metadata:
      name: weave-ippool-v4
      labels:  
        ipam.spidernet.io/subnet-cidr: 10-32-0-0-12
    spec:
      ips:
      - 10.32.0.100-10.32.50.200
      subnet: 10.32.0.0/12
    EOF
    ```

    > `Weave` uses `10.32.0.0/12` as the cluster's default subnet, and thus a SpiderIPPool with ，/the same subnet needs to be created in this case.

3. Verify installation

    ```shell
    [root@node1 ~]# kubectl get po -n kube-system | grep spiderpool
    spiderpool-agent-7hhkz                   1/1     Running     0              13m
    spiderpool-agent-kxf27                   1/1     Running     0              13m
    spiderpool-controller-76798dbb68-xnktr   1/1     Running     0              13m
    spiderpool-init                          0/1     Completed   0              13m
    [root@node1 ~]# kubectl get sp
    NAME               VERSION   SUBNET         ALLOCATED-IP-COUNT   TOTAL-IP-COUNT   DISABLE
    weave-ippool-v4    4         10.32.0.0/12   0                    12901            false
   ```

## Switch `Weave`'s `IPAM` to Spiderpool

Change the `ipam` field of `/etc/cni/net.d/10-weave.conflist` on each node:

Change the following:

```shell
[root@node1 ~]# cat /etc/cni/net.d/10-weave.conflist
{
    "cniVersion": "0.3.0",
    "name": "weave",
    "plugins": [
        {
            "name": "weave",
            "type": "weave-net",
            "hairpinMode": true
        },
        {
            "type": "portmap",
            "capabilities": {"portMappings": true},
            "snat": true
        }
    ]
}
```

To:

```json
{
    "cniVersion": "0.3.0",
    "name": "weave",
    "plugins": [
        {
            "name": "weave",
            "type": "weave-net",
            "ipam": {
              "type": "spiderpool"
            },
            "hairpinMode": true
        },
        {
            "type": "portmap",
            "capabilities": {"portMappings": true},
            "snat": true
        }
    ]
}
```

Alternatively, it can be changed with `jq` in one step. If `jq` is not installed, you can use the following command to install it:

```shell
# Take centos7 as an example
yum -y install jq
```

Change the CNI configuration file:

```shell
cat <<< $(jq '.plugins[0].ipam.type = "spiderpool" ' /etc/cni/net.d/10-weave.conflist) > /etc/cni/net.d/10-weave.conflist
```

> Make sure to run this command at each node

## Create applications

Specify that the Pods will be allocated IPs from that SpiderSubnet via the annotation `ipam.spidernet.io/ippool`:

```shell
[root@node1 ~]# cat << EOF | kubectl apply -f -
apiVersion: apps/v1
kind: Deployment
metadata:
  name: nginx
spec:
  replicas: 2
  selector:
    matchLabels:
      app: nginx
  template:
    metadata:
      annotations:
        ipam.spidernet.io/ippool: '{"ipv4":["weave-ippool-v4"]}'
      labels:
        app: nginx
    spec:
      containers:
      - image: nginx
        imagePullPolicy: IfNotPresent
        lifecycle: {}
        name: container-1
EOF
```

> _spec.template.metadata.annotations.ipam.spidernet.io/subnet_: specifies that the Pods will be assigned IPs from SpiderSubnet: `weave-ippool-v4`.

The Pods have been created and allocated IP addresses from Spiderpool Subnets:

```shell
[root@node1 ~]# kubectl get po  -o wide
NAME                     READY   STATUS    RESTARTS   AGE   IP             NODE    NOMINATED NODE   READINESS GATES
nginx-5745d9b5d7-2rvn7   1/1     Running   0          8s    10.32.22.190   node1   <none>           <none>
nginx-5745d9b5d7-5ssck   1/1     Running   0          8s    10.32.35.87    node2   <none>           <none>

[root@node1 ~]# kubectl get sp
NAME              VERSION   SUBNET          ALLOCATED-IP-COUNT   TOTAL-IP-COUNT   DISABLE
weave-ippool-v4   4         10.32.0.0/12    2                    2                false
```

To test connectivity, let's use inter-node communication between Pods as an example:

```shell
[root@node1 ~]# kubectl exec  nginx-5745d9b5d7-2rvn7 -- ping 10.32.35.87 -c 2
PING 10.32.35.87 (10.32.35.87): 56 data bytes
64 bytes from 10.32.35.87: seq=0 ttl=64 time=4.561 ms
64 bytes from 10.32.35.87: seq=1 ttl=64 time=0.632 ms

--- 10.32.35.87 ping statistics ---
2 packets transmitted, 2 packets received, 0% packet loss
round-trip min/avg/max = 0.632/2.596/4.561 ms
```

The test results indicate that IP allocation and network connectivity are normal. `Spiderpool` has extended the capabilities of Weave's IPAM. Next, you can go to [Spiderpool](https://spidernet-io.github.io/spiderpool/) to explore other features of `Spiderpool`.
