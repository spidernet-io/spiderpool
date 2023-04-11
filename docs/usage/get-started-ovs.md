# Ovs-cni Quick Start

**English** | [**简体中文**](./get-started-ovs-zh_CN.md)

Spiderpool can be used as a solution to provide fixed IPs in an Underlay network scenario, and this article will use [Multus](https://github.com/k8snetworkplumbingwg/multus-cni), [Ovs-cni](https://github.com/k8snetworkplumbingwg/ovs-cni), and [Spiderpool](https://github.com/spidernet-io/spiderpool) as examples to build a complete Underlay network solution that exposes the available bridges as node resources for use by the cluster.

## Prerequisites

1. Make sure a multi-node Kubernetes cluster is ready.

2. [Helm](https://helm.sh/docs/intro/install/) has been already installed.

3. [Open vSwitch](https://docs.openvswitch.org/en/latest/intro/install/#installation-from-packages) must be installed and running on the host.

    The following examples are based on Ubuntu 22.04.1. installation may vary depending on the host system.

    ```bash
    ~# sudo apt-get install -y openvswitch-switch
    ~# sudo systemctl start openvswitch-switch
    ```

## Install Ovs-cni 

[`ovs-cni`](https://github.com/k8snetworkplumbingwg/ovs-cni) is a Kubernetes CNI plugin based on Open vSwitch (OVS) that provides a way to use OVS for network virtualization in a Kubernetes cluster in a Kubernetes cluster.

Verify that the binary /opt/cni/bin/ovs exists on the node. if the binary is missing, you can download it and install it on all nodes using the following command:

```bash
~# wget https://github.com/k8snetworkplumbingwg/ovs-cni/releases/download/v0.31.1/plugin

~# mv ./plugin /opt/cni/bin/ovs

~# chmod +x /opt/cni/bin/ovs
```

Note: Ovs-cni does not configure bridges, it is up to the user to create them and connect them to L2, L3, The following is an example of creating a bridge, to be executed on each node:


1. Create an Open vSwitch bridge.

    ```bash
    ~# ovs-vsctl add-br br1
    ```

2. Network interface connected to the bridge：

    This procedure depends on your platform, the following commands are only example instructions and it may break your system. First use `ip link show` to query the host for available interfaces, the example uses the interface on the host: `eth0` as an example. 

    ```bash
    ~# ovs-vsctl add-port br1 eth0
    ~# ip addr add <IP地址>/<子网掩码> dev br1
    ~# ip link set br1 up
    ~# ip route add default via <默认网关IP> dev br1
    ```

Once created, the following bridge information can be viewed on each node:

```bash
~# ovs-vsctl show
ec16d9e1-6187-4b21-9c2f-8b6cb75434b9
    Bridge br1
        Port eth0
            Interface eth0
        Port br1
            Interface br1
                type: internal
        Port veth97fb4795
            Interface veth97fb4795
    ovs_version: "2.17.3"
```

## Install Multus

[`Multus`](https://github.com/k8snetworkplumbingwg/multus-cni) is a CNI plugin that allows Pods to have multiple NICs by scheduling third-party CNIs. The management of the ovs-cni CNI configuration is simplified through the CRD-based approach provided by Multus, with nothing for manual editing of CNI configuration files on each host.

1. Install Multus via the manifest:

    ```bash
    kubectl apply -f https://raw.githubusercontent.com/k8snetworkplumbingwg/multus-cni/v3.9/deployments/multus-daemonset.yml
    ```

2. Create a NetworkAttachmentDefinition for ovs-cni.

    The following parameters need to be confirmed:

     * Confirm the required host bridge for ovs-cni, for example based on the command `ovs-vsctl show`, this example takes the host bridge: `br1` as an example.

```bash
cat <<EOF | kubectl apply -f -
apiVersion: k8s.cni.cncf.io/v1
kind: NetworkAttachmentDefinition
metadata:
  name: ovs-conf
  namespace: kube-system
spec:
  config: |-
    {
        "cniVersion": "0.3.1",
        "name": "ovs-conf",
        "plugins": [
            {
                "type": "ovs",
                "bridge": "br1",
                "ipam": {
                    "type": "spiderpool"
                }
            }
        ]
    }
EOF
```

## Install Spiderpool

1. Install Spiderpool

    ```bash
    helm repo add spiderpool https://spidernet-io.github.io/spiderpool
    helm repo update spiderpool
    helm install spiderpool spiderpool/spiderpool --namespace kube-system
    ```
    
    > If you are mainland user who is not available to access ghcr.io，You can specify the parameter `-set global.imageRegistryOverride=ghcr.m.daocloud.io` to avoid image pulling failures for Spiderpool.

2. Create a SpiderSubnet instance.

    The Pod will obtain an IP address from this subnet for underlying network communication, so the subnet needs to correspond to the underlying subnet that is being accessed.

    Here is an example of creating a SpiderSubnet instance:

    ```bash
    cat <<EOF | kubectl apply -f -
    apiVersion: spiderpool.spidernet.io/v2beta1
    kind: SpiderSubnet
    metadata:
      name: subnet-test
    spec:
      ipVersion: 4
      ips:
      - "172.18.30.131-172.18.30.140"
      subnet: 172.18.0.0/16
      gateway: 172.18.0.1
      vlan: 0
    EOF
    ```

3. Verify the installation：

```bash
~# kubectl get po -n kube-system | grep spiderpool
spiderpool-agent-f899f                       1/1     Running   0             2m
spiderpool-agent-w69z6                       1/1     Running   0             2m
spiderpool-controller-5bf7b5ddd9-6vd2w       1/1     Running   0             2m
~# kubectl get spidersubnet
NAME          VERSION   SUBNET          ALLOCATED-IP-COUNT   TOTAL-IP-COUNT
subnet-test   4         172.18.0.0/16   0                    10
```

## Create applications

In the following example Yaml, 2 copies of the Deployment are created, of which:

* `ipam.spidernet.io/subnet`: used to specify the subnet of Spiderpool, Spiderpool will automatically select some random IPs in this subnet to create a fixed IP pool to bind with this application, which can achieve the effect of IP fixing.

* `v1.multus-cni.io/default-network`: used to specify Multus' NetworkAttachmentDefinition configuration, which will create a default NIC for the application.

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
        ipam.spidernet.io/subnet: |-
          {
            "ipv4": ["subnet-test"]
          }
        v1.multus-cni.io/default-network: kube-system/ovs-conf
      labels:
        app: test-app
    spec:
      affinity:
        podAntiAffinity:
          requiredDuringSchedulingIgnoredDuringExecution:
            - labelSelector:
                matchLabels:
                  app: test-app
              topologyKey: kubernetes.io/hostname
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

Spiderpool automatically creates a pool of IP fixes for the application and the application's IP will be automatically fixed to that IP range:

```bash
~# kubectl get po -l app=test-app -o wide
NAME                        READY   STATUS    RESTARTS   AGE     IP              NODE                 NOMINATED NODE   READINESS GATES
test-app-6f8dddd88d-hstg7   1/1     Running   0          3m37s   172.18.30.131   ipv4-worker          <none>           <none>
test-app-6f8dddd88d-rj7sm   1/1     Running   0          3m37s   172.18.30.132   ipv4-control-plane   <none>           <none>

~# kubectl get spiderippool
NAME                                 VERSION   SUBNET          ALLOCATED-IP-COUNT   TOTAL-IP-COUNT   DEFAULT   DISABLE
auto-test-app-v4-eth0-9b208a961acd   4         172.18.0.0/16   2                    2                false     false

~#  kubectl get spiderendpoints
NAME                        INTERFACE   IPV4POOL                             IPV4               IPV6POOL   IPV6   NODE
test-app-6f8dddd88d-hstg7   eth0        auto-test-app-v4-eth0-9b208a961acd   172.18.30.131/16                     ipv4-worker
test-app-6f8dddd88d-rj7sm   eth0        auto-test-app-v4-eth0-9b208a961acd   172.18.30.132/16                     ipv4-control-plane
```

Testing Pod communication with cross-node Pods:

```shell
~#kubectl exec -ti test-app-6f8dddd88d-hstg7 -- ping 172.18.30.132 -c 2

PING 172.18.30.132 (172.18.30.132): 56 data bytes
64 bytes from 172.18.30.132: seq=0 ttl=64 time=1.882 ms
64 bytes from 172.18.30.132: seq=1 ttl=64 time=0.195 ms

--- 172.18.30.132 ping statistics ---
2 packets transmitted, 2 packets received, 0% packet loss
round-trip min/avg/max = 0.195/1.038/1.882 ms
```
