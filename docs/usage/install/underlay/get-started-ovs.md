# Ovs-cni Quick Start

**English** | [**简体中文**](./get-started-ovs-zh_CN.md)

Spiderpool can be used as a solution to provide fixed IPs in an Underlay network scenario, and this article will use [Multus](https://github.com/k8snetworkplumbingwg/multus-cni), [Ovs-cni](https://github.com/k8snetworkplumbingwg/ovs-cni), and [Spiderpool](https://github.com/spidernet-io/spiderpool) as examples to build a complete Underlay network solution that exposes the available bridges as node resources for use by the cluster.

## Prerequisites

1. Make sure a multi-node Kubernetes cluster is ready.

2. [Helm](https://helm.sh/docs/intro/install/) has been already installed.

3. Open vSwitch must be installed and running on the host. It could refer to [Installation](https://docs.openvswitch.org/en/latest/intro/install/#installation-from-packages).

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

3. Once created, the following bridge information can be viewed on each node:

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

## Install Spiderpool

1. Install Spiderpool

    ```bash
    helm repo add spiderpool https://spidernet-io.github.io/spiderpool
    helm repo update spiderpool
    helm install spiderpool spiderpool/spiderpool --namespace kube-system --set multus.multusCNI.defaultCniCRName="ovs-conf"
    ```

    > If you are mainland user who is not available to access ghcr.io，You can specify the parameter `-set global.imageRegistryOverride=ghcr.m.daocloud.io` to avoid image pulling failures for Spiderpool.
    >
    > Specify the Multus clusterNetwork of the cluster through `multus.multusCNI.defaultCniCRName`, clusterNetwork is a specific field of the Multus plugin, which is used to specify the default network interface of the Pod.

2. Create a SpiderIPPool instance.

    The Pod will obtain an IP address from the IPPool for underlying network communication, so the subnet of the IPPool needs to correspond to the underlying subnet being accessed.

    Here is an example of creating a SpiderSubnet instance:

    ```bash
    cat <<EOF | kubectl apply -f -
    apiVersion: spiderpool.spidernet.io/v2beta1
    kind: SpiderIPPool
    metadata:
      name: ippool-test
    spec:
      ipVersion: 4
      ips:
      - "172.18.30.131-172.18.30.140"
      subnet: 172.18.0.0/16
      gateway: 172.18.0.1
      multusName: 
      - ovs-conf
    EOF
    ```

3. Verify the installation：

    ```bash
    ~# kubectl get po -n kube-system |grep spiderpool
    spiderpool-agent-7hhkz                   1/1     Running     0              13m
    spiderpool-agent-kxf27                   1/1     Running     0              13m
    spiderpool-controller-76798dbb68-xnktr   1/1     Running     0              13m
    spiderpool-init                          0/1     Completed   0              13m
    spiderpool-multus-7vkm2                  1/1     Running     0              13m
    spiderpool-multus-rwzjn                  1/1     Running     0              13m

    ~# kubectl get sp ippool-test       
    NAME          VERSION   SUBNET          ALLOCATED-IP-COUNT   TOTAL-IP-COUNT   DEFAULT
    ippool-test   4         172.18.0.0/16   0                    10               false
    ~# 
    ```

4. To simplify writing Multus CNI configuration in JSON format, Spiderpool provides SpiderMultusConfig CR to automatically manage Multus NetworkAttachmentDefinition CR. Here is an example of creating an ovs-cni SpiderMultusConfig configuration:

    * Confirm the required host bridge for ovs-cni, for example based on the command `ovs-vsctl show`, this example takes the host bridge: `br1` as an example.

    ```shell
    BRIDGE_NAME="br1"
    cat <<EOF | kubectl apply -f -
    apiVersion: spiderpool.spidernet.io/v2beta1
    kind: SpiderMultusConfig
    metadata:
      name: ovs-conf
      namespace: kube-system
    spec:
      cniType: ovs
      ovs:
        bridge: "${BRIDGE_NAME}"
    EOF
    ```

## Create applications

In the following example Yaml, 2 copies of the Deployment are created, of which:

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
        ipam.spidernet.io/ippool: |-
          {
            "ipv4": ["ippool-test"]
          }
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

SpiderIPPool assigns an IP to the application, and the application's IP will be automatically fixed within this IP range:

```bash
~# kubectl get po -l app=test-app -o wide
NAME                        READY   STATUS    RESTARTS   AGE     IP              NODE                 NOMINATED NODE   READINESS GATES
test-app-6f8dddd88d-hstg7   1/1     Running   0          3m37s   172.18.30.131   ipv4-worker          <none>           <none>
test-app-6f8dddd88d-rj7sm   1/1     Running   0          3m37s   172.18.30.132   ipv4-control-plane   <none>           <none>

~# kubectl get spiderippool
NAME          VERSION   SUBNET          ALLOCATED-IP-COUNT   TOTAL-IP-COUNT   DEFAULT   DISABLE
ippool-test   4         172.18.0.0/16   2                    2                false     false

~# kubectl get spiderendpoints
NAME                        INTERFACE   IPV4POOL      IPV4               IPV6POOL   IPV6   NODE
test-app-6f8dddd88d-hstg7   eth0        ippool-test   172.18.30.131/16                     ipv4-worker
test-app-6f8dddd88d-rj7sm   eth0        ippool-test   172.18.30.132/16                     ipv4-control-plane
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
