# Pod annotation of multi-NIC

When assigning multiple NICs to a Pod with [Multus CNI](https://github.com/k8snetworkplumbingwg/multus-cni), Spiderpool supports to specify the IP pools for each interface.

This feature supports to implement by annotation `ipam.spidernet.io/subnets` and `ipam.spidernet.io/ippools`

## Get Started

The example will create two Multus CNI Configuration object and create two underlay subnets.
Then run a Pod with two NICs with IP in different subnets.

### Set up Spiderpool

Follow the guide [installation](./install/get-started-kind.md) to install Spiderpool.

### Set up Multus Configuration

In this example, Macvlan will be used as the main CNI, Create two network-attachment-definitions，The following parameters need to be confirmed:

* Confirm the host machine parent interface required for Macvlan. This example takes the ens192 and ens224 network cards of the host machine as examples to create a Macvlan sub interface for Pod to use.

* In order to use the Veth plugin for clusterIP communication, you need to confirm the serviceIP CIDR of the cluster service, e.g. by using the command `kubectl -n kube-system get configmap kubeadm-config -oyaml | grep service`.

```shell
~# kubectl apply -f https://raw.githubusercontent.com/spidernet-io/spiderpool/main/docs/example/multi-interfaces-annotation/multus-conf.yaml

~# kubectl get network-attachment-definitions.k8s.cni.cncf.io -n kube-system
NAME                  AGE
macvlan-conf-ens192   20s
macvlan-conf-ens224   22s
```

## multiple NICs by subnet

Create two Subnets to provide IP addresses for different interfaces.

```shell
~# kubectl apply -f https://raw.githubusercontent.com/spidernet-io/spiderpool/main/docs/example/multi-interfaces-annotation/different-segment-ipv4-subnets.yaml

~# kubectl get spidersubnet
NAME                 VERSION   SUBNET        ALLOCATED-IP-COUNT   TOTAL-IP-COUNT
subnet-test-ens192   4         10.6.0.1/16   0                    10
subnet-test-ens224   4         10.7.0.1/16   0                    10
```

In the following example Yaml, 2 copies of the Deployment are created：

```shell
~# kubectl apply -f https://raw.githubusercontent.com/spidernet-io/spiderpool/main/docs/example/multi-interfaces-annotation/subnet-test-deploy.yaml
```

Eventually, when the Deployment is created, Spiderpool will select random IPs from the specified subnet to create two fixed IP pools to bind to each of the Deployment Pod's two NICs.

```bash
~# kubectl get spiderippool
NAME                                 VERSION   SUBNET        ALLOCATED-IP-COUNT   TOTAL-IP-COUNT   DEFAULT   DISABLE
auto-test-app-v4-eth0-b1a361c7e9df   4         10.6.0.1/16   2                    3                false     false
auto-test-app-v4-net1-b1a361c7e9df   4         10.7.0.1/16   2                    3                false     false

~# kubectl get spiderippool auto-test-app-v4-eth0-b1a361c7e9df -o jsonpath='{.spec.ips}'
["10.6.168.171-10.6.168.173"]

~# kubectl get spiderippool auto-test-app-v4-net1-b1a361c7e9df -o jsonpath='{.spec.ips}'
["10.7.168.171-10.7.168.173"]

~# kubectl get po -l app=test-app -o wide
NAME                        READY   STATUS    RESTARTS   AGE   IP             NODE    NOMINATED NODE   READINESS GATES
test-app-6f4594ff67-fkqbw   1/1     Running   0          40s   10.6.168.172   node2   <none>           <none>
test-app-6f4594ff67-gwlx8   1/1     Running   0          40s   10.6.168.173   node1   <none>           <none>

~# kubectl exec -ti test-app-6f4594ff67-fkqbw -- ip a
3: eth0@if2: <BROADCAST,MULTICAST,UP,LOWER_UP> mtu 1500 qdisc noqueue state UP group default
    link/ether ae:fa:5e:d9:79:11 brd ff:ff:ff:ff:ff:ff link-netnsid 0
    inet 10.6.168.172/16 brd 10.6.255.255 scope global eth0
       valid_lft forever preferred_lft forever
4: veth0@if13: <BROADCAST,MULTICAST,UP,LOWER_UP> mtu 1500 qdisc noqueue state UP group default
    link/ether 26:6f:22:91:22:f9 brd ff:ff:ff:ff:ff:ff link-netnsid 0
5: net1@if3: <BROADCAST,MULTICAST,UP,LOWER_UP> mtu 1500 qdisc noqueue state UP group default
    link/ether d6:4b:c2:6a:62:0f brd ff:ff:ff:ff:ff:ff link-netnsid 0
    inet 10.7.168.173/16 brd 10.7.255.255 scope global net1
       valid_lft forever preferred_lft forever
```

The following command shows the multi-NIC routing information in the Pod. The Veth plug-in can automatically coordinate the policy routing between multiple NICs and solve the communication problems between multiple NICs.

```bash
~# kubectl exec -ti test-app-6f4594ff67-fkqbw -- ip rule show
0:  from all lookup local
32764:  from 10.7.168.173 lookup 100
32765:  from all to 10.7.168.173/16 lookup 100
32766:  from all lookup main
32767:  from all lookup default

~# kubectl exec -ti test-app-6f4594ff67-fkqbw -- ip r show main
default via 10.6.0.1 dev eth0

~# kubectl exec -ti test-app-6f4594ff67-fkqbw -- ip route show table 100
default via 10.7.0.1 dev net1
10.6.168.123 dev veth0 scope link
10.7.0.0/16 dev net1 proto kernel scope link src 10.7.168.173
10.96.0.0/12 via 10.6.168.123 dev veth0
```

## multiple NICs by IPPool

Create two IPPools to provide IP addresses for different interfaces.

```shell
~# kubectl apply -f https://raw.githubusercontent.com/spidernet-io/spiderpool/main/docs/example/multi-interfaces-annotation/different-segment-ipv4-ippools.yaml

~# kubectl get spiderippool
NAME                   VERSION   SUBNET        ALLOCATED-IP-COUNT   TOTAL-IP-COUNT   DEFAULT   DISABLE
ippool-test-ens192     4         10.6.0.1/16   0                    5                false     false
ippool-test-ens224     4         10.7.0.1/16   0                    5                false     false
```

In the following example Yaml, 1 copies of the Deployment are created：

```shell
~# kubectl apply -f https://raw.githubusercontent.com/spidernet-io/spiderpool/main/docs/example/multi-interfaces-annotation/ippool-test-deploy.yaml
```

Eventually, when the Deployment is created, Spiderpool randomly selects IPs from the two specified IPPools to form bindings to each of the two NICs.

```bash
~# kubectl get spiderippool
NAME                   VERSION   SUBNET        ALLOCATED-IP-COUNT   TOTAL-IP-COUNT   DEFAULT   DISABLE
ippool-test-ens192     4         10.6.0.1/16   1                    5                false     false
ippool-test-ens224     4         10.7.0.1/16   1                    5                false     false

~# kubectl get po -l app=ippool-test-app -o wide
NAME                               READY   STATUS    RESTARTS   AGE     IP             NODE    NOMINATED NODE   READINESS GATES
ippool-test-app-65f646574c-mpr47   1/1     Running   0          6m18s   10.6.168.175   node2   <none>           <none>

~#  kubectl exec -ti ippool-test-app-65f646574c-mpr47 -- ip a
...
3: eth0@tunl0: <BROADCAST,MULTICAST,UP,LOWER_UP,M-DOWN> mtu 1500 qdisc noqueue
    link/ether 2a:ca:ce:06:1e:91 brd ff:ff:ff:ff:ff:ff
    inet 10.6.168.175/16 brd 10.6.255.255 scope global eth0
       valid_lft forever preferred_lft forever
4: veth0@if15: <BROADCAST,MULTICAST,UP,LOWER_UP,M-DOWN> mtu 1500 qdisc noqueue
    link/ether 86:ba:6f:97:ae:1b brd ff:ff:ff:ff:ff:ff
5: net1@eth0: <BROADCAST,MULTICAST,UP,LOWER_UP> mtu 1500 qdisc noqueue
    link/ether f2:12:b5:8c:ff:4f brd ff:ff:ff:ff:ff:ff
    inet 10.7.168.177/16 brd 10.7.255.255 scope global net1
       valid_lft forever preferred_lft forever
```

The following command shows the multi-NIC routing information in the Pod. The Veth plug-in can automatically coordinate the policy routing between multiple NICs and solve the communication problems between multiple NICs.

```bash
~# kubectl exec -ti ippool-test-app-65f646574c-mpr47 -- ip rule show
0: from all lookup local
32764: from 10.7.168.177 lookup 100
32765: from all to 10.7.168.177/16 lookup 100
32766: from all lookup main
32767: from all lookup default

~# kubectl exec -ti ippool-test-app-65f646574c-mpr47 -- ip r show main
default via 10.6.0.1 dev eth0

~# kubectl exec -ti ippool-test-app-65f646574c-mpr47 -- ip r show table 100
default via 10.7.0.1 dev net1
10.6.168.123 dev veth0 scope link
10.7.0.0/16 dev net1 scope link  src 10.7.168.177
10.96.0.0/12 via 10.6.168.123 dev veth0
```

## Clean up

Clean the relevant resources so that you can run this tutorial again.

```bash
kubectl delete \
-f https://raw.githubusercontent.com/spidernet-io/spiderpool/main/docs/example/multi-interfaces-annotation/multus-conf.yaml \
-f https://raw.githubusercontent.com/spidernet-io/spiderpool/main/docs/example/multi-interfaces-annotation/different-segment-ipv4-subnets.yaml \
-f https://raw.githubusercontent.com/spidernet-io/spiderpool/main/docs/example/multi-interfaces-annotation/subnet-test-deploy.yaml \
-f https://raw.githubusercontent.com/spidernet-io/spiderpool/main/docs/example/multi-interfaces-annotation/different-segment-ipv4-ippools.yaml \
-f https://raw.githubusercontent.com/spidernet-io/spiderpool/main/docs/example/basic/custom-dual-ippool-deploy.yaml \
-f https://raw.githubusercontent.com/spidernet-io/spiderpool/main/docs/example/multi-interfaces-annotation/ippool-test-deploy.yaml \
--ignore-not-found=true
```
