# Cilium provides network policy support for IPVlan

**English** | [**简体中文**](./cilium-chaining-zh_CN.md)

## Introduction

This article describes how IPVlan integrates with Cilium to provide network policy capabilities for IPVlan CNI.

## background

Currently, most Underlay type CNIs in the community, such as IPVlan, Macvlan, etc., do not support Kubernetes' native network policy capabilities. We can use the Cilium chaining-mode function to provide network policy capabilities for IPVlan.However, Cilium officially removed support for IPVlan Dataplane in version 1.12. For details, see [removed-options](https://docs.cilium.io/en/v1.12/operations/upgrade/#removed-options).

Inspired by [Terway](https://github.com/AliyunContainerService/terway), the [cilium-chaining](https://github.com/spidernet-io/cilium-chaining) project is based on Cilium
The v1.12.7 version modifies the IPVlan Dataplane part to enable Cilium to work with IPVlan in chaining-mode. Solve the problem that IPVlan does not support Kubernetes’ native network policy capabilities.

## Prerequisites

1. The node kernel version is required to be at least greater than 4.19
2. Prepare a Kubernetes cluster and be careful not to install Cilium
3. Installed [Helm](https://helm.sh/docs/intro/install/)

## Steps

### Install Spiderpool

Refer to [Installation](./readme.md) to install Spiderpool.

### Install Cilium-chaining

1. Install the cilium-chaining component using the following command:

    ```bash
    helm repo add cilium-chaining https://spidernet-io.github.io/cilium-chaining
    helm repo update cilium-chaining
    helm install cilium-chaining/cilium-chaining --namespace kube-system
    ```

2. Verify installation:

    ```bash
    ~# kubectl  get po -n kube-system
    NAME                                     READY   STATUS      RESTARTS         AGE
    cilium-chaining-4xnnm                    1/1     Running     0                5m48s
    cilium-chaining-82ptj                    1/1     Running     0                5m48s
    ```

## Configure CNI

Create Multus NetworkAttachmentDefinition CR. The following is an example of creating an IPvlan NetworkAttachmentDefinition configuration:

- In the following configuration, specify master as ens192, ens192 must exist on the node

- Embed cilium into CNI configuration, placed after ipvlan plugin

- The name of CNI must be consistent with the cniChainingMode when installing cilium-chaining, otherwise it will not work properly

```shell
IPVLAN_MASTER_INTERFACE="ens192"
CNI_CHAINING_MODE="terway-chainer"
cat <<EOF | kubectl apply -f -
apiVersion: k8s.cni.cncf.io/v1
kind: NetworkAttachmentDefinition
metadata:
  name: ipvlan-ens192
  namespace: kube-system
spec:
  config: |
   {
     "cniVersion": "0.4.0",
     "name": "${CNI_CHAINING_MODE}",
     "plugins": [
      {
         "type": "ipvlan",
         "mode": "l2",
         "master": "${IPVLAN_MASTER_INTERFACE}",
         "ipam": {
         "type": "spiderpool"
         }
      },
      {
        "type": "cilium-cni"
      },
     {
        "type": "coordinator"
     }]
   }
EOF
```

### Create a test application

In the following example Yaml, a set of DaemonSet applications will be created, using `v1.multus-cni.io/default-network`: used to specify the CNI configuration file used by the application:

```shell
APP_NAME=test
cat <<EOF | kubectl create -f -
apiVersion: apps/v1
kind: DaemonSet
metadata:
  labels:
    app: ${APP_NAME}
  name: ${APP_NAME}
  namespace: default
spec:
  selector:
    matchLabels:
      app: ${APP_NAME}
  template:
    metadata:
      labels:
        app: ${APP_NAME}
      annotations:
        v1.multus-cni.io/default-network: kube-system/ipvlan-ens192
    spec:
      containers:
      - image: docker.io/centos/tools
        imagePullPolicy: IfNotPresent
        name: ${APP_NAME}
        ports:
        - name: http
          containerPort: 80
          protocol: TCP
EOF
```

Check Pod running status:

```bash
~# kubectl get po -owide
NAME                    READY   STATUS              RESTARTS   AGE     IP             NODE          NOMINATED NODE   READINESS GATES
test-55c97ccfd8-l4h5w   1/1     Running             0          3m50s   10.6.185.217   worker1       <none>           <none>
test-55c97ccfd8-w62k7   1/1     Running             0          3m50s   10.6.185.206   controller1   <none>           <none>
```

### Verify whether the network policy is effective

- Test the communication between Pods and Pods across nodes and subnets

    ```shell
    ~# kubectl exec -it test-55c97ccfd8-l4h5w -- ping -c2 10.6.185.30
    PING 10.6.185.30 (10.6.185.30): 56 data bytes
    64 bytes from 10.6.185.30: seq=0 ttl=64 time=1.917 ms
    64 bytes from 10.6.185.30: seq=1 ttl=64 time=1.406 ms

    --- 10.6.185.30 ping statistics ---
    2 packets transmitted, 2 packets received, 0% packet loss
    round-trip min/avg/max = 1.406/1.661/1.917 ms
    ~# kubectl exec -it test-55c97ccfd8-l4h5w -- ping -c2 10.6.185.206
    PING 10.6.185.206 (10.6.185.206): 56 data bytes
    64 bytes from 10.6.185.206: seq=0 ttl=64 time=1.608 ms
    64 bytes from 10.6.185.206: seq=1 ttl=64 time=0.647 ms

    --- 10.6.185.206 ping statistics ---
    2 packets transmitted, 2 packets received, 0% packet loss
    round-trip min/avg/max = 0.647/1.127/1.608 ms
    ```

- Create a network policy that prohibits Pods from communicating with the outside world

    ```shell
    ~# cat << EOF | kubectl apply -f -
    kind: NetworkPolicy
    apiVersion: networking.k8s.io/v1
    metadata:
      name: deny-all
    spec:
      podSelector:
        matchLabels:
          app: test
      policyTypes:
      - Egress
      - Ingress
    ```

> deny-all matches all pods based on label. This policy prohibits pods from communicating with others.

- Verify Pod external communication again

    ```shell
    ~# kubectl exec -it test-55c97ccfd8-l4h5w -- ping -c2 10.6.185.206
    kubectl exec [POD] [COMMAND] is DEPRECATED and will be removed in a future version. Use kubectl exec [POD] -- [COMMAND] instead.
    PING 10.6.185.206 (10.6.185.206): 56 data bytes
    --- 10.6.185.206 ping statistics ---
    14 packets transmitted, 0 packets received, 100% packet loss
    ```

### Conclusion

From the results, it can be seen that the Pod's access to external traffic is prohibited and the network policy takes effect, proving that the Cilium-chaining project helps IPVlan achieve network policy capabilities.
