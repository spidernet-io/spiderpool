# Ovs-cni Quick Start

**English** | [**简体中文**](./get-started-ovs-zh_CN.md)

Spiderpool can be used as a solution to provide fixed IPs in an Underlay network scenario, and this article will use [Multus](https://github.com/k8snetworkplumbingwg/multus-cni), [Ovs-cni](https://github.com/k8snetworkplumbingwg/ovs-cni), and [Spiderpool](https://github.com/spidernet-io/spiderpool) as examples to build a complete Underlay network solution that exposes the available bridges as node resources for use by the cluster.

[`ovs-cni`](https://github.com/k8snetworkplumbingwg/ovs-cni) is a Kubernetes CNI plugin that utilizes Open vSwitch (OVS) to enable network virtualization within a Kubernetes cluster.

## Prerequisites

1. [System requirements](./../system-requirements.md)

2. Make sure a multi-node Kubernetes cluster is ready.

3. [Helm](https://helm.sh/docs/intro/install/) has been already installed.

4. Open vSwitch must be installed and running on the host. It could refer to [Installation](https://docs.openvswitch.org/en/latest/intro/install/#installation-from-packages).

    The following examples are based on Ubuntu 22.04.1. installation may vary depending on the host system.

    ```bash
    ~# sudo apt-get install -y openvswitch-switch
    ~# sudo systemctl start openvswitch-switch
    ```

5. If your OS is such as Fedora and CentOS and uses NetworkManager to manage network configurations, you need to configure NetworkManager in the following scenarios:

    * If you are using Underlay mode, the `coordinator` will create veth interfaces on the host. To prevent interference from NetworkManager with the veth interface. It is strongly recommended that you configure NetworkManager.

    * If you create VLAN and Bond interfaces through Ifacer, NetworkManager may interfere with these interfaces, leading to abnormal pod access. It is strongly recommended that you configure NetworkManager.

      ```shell
      ~# IFACER_INTERFACE="<NAME>"
      ~# cat << EOF | > /etc/NetworkManager/conf.d/spidernet.conf
      > [keyfile]
      > unmanaged-devices=interface-name:^veth*;interface-name:${IFACER_INTERFACE}
      > EOF
      ~# systemctl restart NetworkManager
      ```

## Install Spiderpool

1. Install Spiderpool.

    ```bash
    helm repo add spiderpool https://spidernet-io.github.io/spiderpool
    helm repo update spiderpool
    helm install spiderpool spiderpool/spiderpool --namespace kube-system --set multus.multusCNI.defaultCniCRName="ovs-conf" --set plugins.installOvsCNI=true
    ```

    > If ovs-cni is not installed, you can install it by specifying the Helm parameter `--set plugins.installOvsCNI=true`.
    >
    > If you are a mainland user who is not available to access ghcr.io, you can specify the parameter `-set global.imageRegistryOverride=ghcr.m.daocloud.io` to avoid image pulling failures for Spiderpool.
    >
    > Specify the name of the NetworkAttachmentDefinition instance for the default CNI used by Multus via `multus.multusCNI.defaultCniCRName`. If the `multus.multusCNI.defaultCniCRName` option is provided, an empty NetworkAttachmentDefinition instance will be automatically generated upon installation. Otherwise, Multus will attempt to create a NetworkAttachmentDefinition instance based on the first CNI configuration found in the /etc/cni/net.d directory. If no suitable configuration is found, a NetworkAttachmentDefinition instance named `default` will be created to complete the installation of Multus.

2. To configure Open vSwitch bridges on each node:

    Create a bridge and configure it using `eth0`` as an example.

    ```bash
    ~# ovs-vsctl add-br br1
    ~# ovs-vsctl add-port br1 eth0
    ~# ip addr add <IP address>/<subnet mask> dev br1
    ~# ip link set br1 up
    ~# ip route add default via <default gateway IP> dev br1
    ```

    Pleade include these commands in your system startup script to ensure they take effect after host restarts.

    After creating the bridge, you will be able to view its information on each node:

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

3. Create a SpiderIPPool instance.

    The Pod will obtain an IP address from the IP pool for underlying network communication, so the subnet of the IP pool needs to correspond to the underlying subnet being accessed.

    Here is an example of creating a SpiderSubnet instance:

    ```bash
    cat <<EOF | kubectl apply -f -
    apiVersion: spiderpool.spidernet.io/v2beta1
    kind: SpiderIPPool
    metadata:
      name: ippool-test
    spec:
      ips:
      - "172.18.30.131-172.18.30.140"
      subnet: 172.18.0.0/16
      gateway: 172.18.0.1
      multusName: 
      - kube-system/ovs-conf
    EOF
    ```

4. Verify the installation：

    ```bash
    ~# kubectl get po -n kube-system |grep spiderpool
    spiderpool-agent-7hhkz                   1/1     Running     0              13m
    spiderpool-agent-kxf27                   1/1     Running     0              13m
    spiderpool-controller-76798dbb68-xnktr   1/1     Running     0              13m
    spiderpool-init                          0/1     Completed   0              13m

    ~# kubectl get sp ippool-test       
    NAME          VERSION   SUBNET          ALLOCATED-IP-COUNT   TOTAL-IP-COUNT   DEFAULT
    ippool-test   4         172.18.0.0/16   0                    10               false
    ~# 
    ```

5. To simplify writing Multus CNI configuration in JSON format, Spiderpool provides SpiderMultusConfig CR to automatically manage Multus NetworkAttachmentDefinition CR. Here is an example of creating an ovs-cni SpiderMultusConfig configuration:

    * Confirm the bridge name for ovs-cni. Take the host bridge: `br1` as an example:

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

## Create Applications

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
