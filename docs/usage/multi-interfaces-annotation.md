# Pod annotation of multi-NIC

When assigning multiple NICs to a pod with [Multus CNI](https://github.com/k8snetworkplumbingwg/multus-cni), Spiderpool supports to specify the IP pools for each interface.

This feature supports to implement by annotation `ipam.spidernet.io/subnet` and `ipam.spidernet.io/ippool` 

## Get Started

The example will create two Multus CNI Configuration object and create two underlay subnets.
Then run a pod with two NICs with IP in different subnets.

### Set up Spiderpool

Follow the guide [installation](https://github.com/spidernet-io/spiderpool/blob/main/docs/usage/install.md) to install Spiderpool.

### Set up Multus Configuration

TODO, create two network-attachment-definitions

## multiple NICs by subnet

TODO

## multiple NICs by IPPool

Create two IPPools to provide IP addresses for different interfaces.

```bash
kubectl apply -f https://raw.githubusercontent.com/spidernet-io/spiderpool/main/docs/example/multi-interfaces-annotation/different-segment-ipv4-ippools.yaml
```

```bash
kubectl get sp
NAME               VERSION   SUBNET           ALLOCATED-IP-COUNT   TOTAL-IP-COUNT   DISABLE
eth0-ipv4-ippool   4         172.18.41.0/24   0                    2                false
net1-ipv4-ippool   4         172.18.42.0/24   0                    2                false
```

Then, create a Deployment whose Pod is [attached an additional interface](https://github.com/k8snetworkplumbingwg/multus-cni/blob/master/docs/quickstart.md#creating-a-pod-that-attaches-an-additional-interface) (macvlan) through the Multus annotation `k8s.v1.cni.cncf.io/networks`.

```bash
kubectl apply -f https://raw.githubusercontent.com/spidernet-io/spiderpool/main/docs/example/multi-interfaces-annotation/multi-macvlan-interfaces-deploy.yaml
```

```yaml
apiVersion: apps/v1
kind: Deployment
metadata:
  name: multi-macvlan-interfaces-deploy
spec:
  replicas: 1
  selector:
    matchLabels:
      app: multi-macvlan-interfaces-deploy
  template:
    metadata:
      annotations:
        v1.multus-cni.io/default-network: kube-system/macvlan-pod-network
        k8s.v1.cni.cncf.io/networks: kube-system/macvlan-cni-default
        ipam.spidernet.io/ippools: |-
          [{
            "interface": "eth0",
            "ipv4": ["eth0-ipv4-ippool"]
          },{
            "interface": "net1",
            "ipv4": ["net1-ipv4-ippool"]
          }]
      labels:
        app: multi-macvlan-interfaces-deploy
    spec:
      containers:
      - name: multi-macvlan-interfaces-deploy
        image: busybox
        imagePullPolicy: IfNotPresent
        command: ["/bin/sh", "-c", "trap : TERM INT; sleep infinity & wait"]
```

The Pod annotation `ipam.spidernet.io/ippools` specifies the [pool selection rules](TODO) for each interfaces in the form of an array, which means that executing the [CNI ADD command](https://www.cni.dev/docs/spec/#cni-operations) with the environment parameter `CNI_IFNAME` as `eth0` will get an IP allocation result from IPPool `eth0-ipv4-ippool`. The interface `net1` works in a similar way.

> As for the reason why the two interfaces are named `eth0` and `net1` respectively, it is because that is the convention of Multus CNI. Generally, the first interface (default interface) of a Pod will be named `eth0`, and the additional interfaces attached will be named `net1`, `net2`...

Finally, you can check the details of the IP allocation result.

```bash
kubectl get se multi-macvlan-interfaces-deploy-b99b55bd7-gvvqt -o jsonpath='{.status.current}' | jq
{
  "containerID": "57e7a0a713bc16bfeb2390969a43daef99d1625c8bebc841646a90fa854900f3",
  "creationTime": "2022-11-24T05:22:19Z",
  "ips": [
    {
      "interface": "eth0",
      "ipv4": "172.18.41.41/24",
      "ipv4Pool": "eth0-ipv4-ippool",
      "vlan": 0
    },
    {
      "interface": "net1",
      "ipv4": "172.18.42.40/24",
      "ipv4Pool": "net1-ipv4-ippool",
      "vlan": 0
    }
  ],
  "node": "spider-worker"
}
```

Inspect the container.

```bash
kubectl exec -it multi-macvlan-interfaces-deploy-b99b55bd7-gvvqt -- ip a
...
4: eth0@if13: <BROADCAST,MULTICAST,UP,LOWER_UP,M-DOWN> mtu 1500 qdisc noqueue 
    link/ether 46:34:cc:2e:70:2c brd ff:ff:ff:ff:ff:ff
    inet 172.18.41.41/24 brd 172.18.41.255 scope global eth0
       valid_lft forever preferred_lft forever
    inet6 fe80::4434:ccff:fe2e:702c/64 scope link 
       valid_lft forever preferred_lft forever
5: net1@if13: <BROADCAST,MULTICAST,UP,LOWER_UP,M-DOWN> mtu 1500 qdisc noqueue 
    link/ether aa:e3:32:27:75:01 brd ff:ff:ff:ff:ff:ff
    inet 172.18.42.40/24 brd 172.18.42.255 scope global net1
       valid_lft forever preferred_lft forever
    inet6 fe80::a8e3:32ff:fe27:7501/64 scope link 
       valid_lft forever preferred_lft forever
```

## Clean up

Clean the relevant resources so that you can run this tutorial again.

```bash
kubectl delete \
-f https://raw.githubusercontent.com/spidernet-io/spiderpool/main/docs/example/multi-interfaces-annotation/different-segment-ipv4-ippools.yaml \
-f https://raw.githubusercontent.com/spidernet-io/spiderpool/main/docs/example/multi-interfaces-annotation/macvlan-cni-default.yaml \
--ignore-not-found=true
```
