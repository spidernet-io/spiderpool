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
      ~# cat > /etc/NetworkManager/conf.d/spidernet.conf <<EOF
      [keyfile]
      unmanaged-devices=interface-name:^veth*;interface-name:${IFACER_INTERFACE}
      EOF
      ~# systemctl restart NetworkManager
      ```

## Configure Open vSwitch bridge on the node

The following is an example of creating and configuring a persistent OVS Bridge. This article takes the `eth0` network card as an example and needs to be executed on each node.

### Ubuntu system uses netplan persistence OVS Bridge

If you are using an Ubuntu system, you can refer to this chapter to configure OVS Bridge through `netplan`.

1. Create OVS Bridge

    ```bash
    ~# ovs-vsctl add-br br1
    ~# ovs-vsctl add-port br1 eth0
    ~# ip link set br1 up
    ```

2. After creating 12-br1.yaml in the /etc/netplan directory, run `netplan apply` to take effect. To ensure that br1 is still available in scenarios such as restarting the host, please check whether the eth0 network card is also managed by netplan.

    ```yaml: 12-br1.yaml
    network:
    version: 2
    renderer: networkd
    ethernets:
      br1:
      addresses:
        - "<IP address>/<Subnet mask>" # 172.18.10.10/16
    ```

3. After creation, you can view the following bridge information on each node:

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

    ~# ip a show br1
    208: br1: <BROADCAST,MULTICAST,UP,LOWER_UP> mtu 1500 qdisc noqueue state UNKNOWN group default qlen 1000
        link/ether 00:50:56:b4:5f:fd brd ff:ff:ff:ff:ff:ff
        inet 172.18.10.10/16 brd 172.18.255.255 scope global noprefixroute br1
          valid_lft forever preferred_lft forever
        inet6 fe80::4f28:8ef1:6b82:a9e4/64 scope link noprefixroute 
          valid_lft forever preferred_lft forever
    ```

### Fedora, Centos, etc. use NetworkManager to persist OVS Bridge

If you use OS such as Fedora, Centos, etc., it is recommended to use NetworkManager persistent OVS Bridge. Persisting OVS Bridge through NetworkManager is a more general method that is not limited to operating systems.

1. To use NetworkManager to persist OVS Bridge, you need to install the OVS NetworkManager plug-in. The example is as follows:

    ```bash
    ~# sudo dnf install -y NetworkManager-ovs
    ~# sudo systemctl restart NetworkManager
    ```

2. Create ovs bridges, ports and interfaces.

    ```bash
    ~# sudo nmcli con add type ovs-bridge conn.interface br1 con-name br1
    ~# sudo nmcli con add type ovs-port conn.interface br1-port master br1 con-name br1-port
    ~# sudo nmcli con add type ovs-interface slave-type ovs-port conn.interface br1 master br1-port con-name br1-int
    ```

3. Create another port on the bridge and select the eth0 NIC in the physical device as its Ethernet interface so that real traffic can flow on the network.

    ```bash
    ~# sudo nmcli con add type ovs-port conn.interface ovs-port-eth0 master br1 con-name ovs-port-eth0
    ~# sudo nmcli con add type ethernet conn.interface eth0 master ovs-port-eth0 con-name ovs-port-eth0-int
    ```

4. Configure and activate the ovs bridge.

     Configure the bridge by setting a static IP

    ```bash
    ~# sudo nmcli con modify br1-int ipv4.method static ipv4.address "<IP地址>/<子网掩码>" # 172.18.10.10/16
    ```

    Activate bridge

    ```bash
    ~# sudo nmcli con down "eth0"
    ~# sudo nmcli con up ovs-port-eth0-int
    ~# sudo nmcli con up br1-int
    ```

5. After creation, you can view information similar to the following on each node.

    ```bash
    ~# nmcli c
    br1-int             dbb1c9be-e1ab-4659-8d4b-564e3f8858fa  ovs-interface  br1             
    br1                 a85626c1-2392-443b-a767-f86a57a1cff5  ovs-bridge     br1             
    br1-port            fe30170f-32d2-489e-9ca3-62c1f5371c6c  ovs-port       br1-port        
    ovs-port-eth0       a43771a9-d840-4d2d-b1c3-c501a6da80ed  ovs-port       ovs-port-eth0   
    ovs-port-eth0-int   1334f49b-dae4-4225-830b-4d101ab6fad6  ethernet       eth0         

    ~# ovs-vsctl show
    203dd6d0-45f4-4137-955e-c4c36b9709e6
        Bridge br1
            Port ovs-port-eth0
                Interface eth0
                    type: system
            Port br1-port
                Interface br1
                    type: internal
        ovs_version: "3.2.1"

    ~# ip a show br1
    208: br1: <BROADCAST,MULTICAST,UP,LOWER_UP> mtu 1500 qdisc noqueue state UNKNOWN group default qlen 1000
        link/ether 00:50:56:b4:5f:fd brd ff:ff:ff:ff:ff:ff
        inet 172.18.10.10/16 brd 172.18.255.255 scope global noprefixroute br1
          valid_lft forever preferred_lft forever
        inet6 fe80::4f28:8ef1:6b82:a9e4/64 scope link noprefixroute 
          valid_lft forever preferred_lft forever
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

2. Please check if `Spidercoordinator.status.phase` is `Synced`:

    ```shell
    ~# kubectl  get spidercoordinators.spiderpool.spidernet.io default -o yaml
    apiVersion: spiderpool.spidernet.io/v2beta1
    kind: SpiderCoordinator
    metadata:
      finalizers:
      - spiderpool.spidernet.io
      name: default
    spec:
      detectGateway: false
      detectIPConflict: false
      hijackCIDR:
      - 169.254.0.0/16
      podRPFilter: 0
      hostRPFilter: 0
      hostRuleTable: 500
      mode: auto
      podCIDRType: calico
      podDefaultRouteNIC: ""
      podMACPrefix: ""
      tunePodRoutes: true
    status:
      overlayPodCIDR:
      - 10.244.64.0/18
      phase: Synced
      serviceCIDR:
      - 10.233.0.0/18
    ```

    At present:

    * Spiderpool prioritizes obtaining the cluster's Pod and Service subnets by querying the kube-system/kubeadm-config ConfigMap. 
    * If the kubeadm-config does not exist, causing the failure to obtain the cluster subnet, Spiderpool will attempt to retrieve the cluster Pod and Service subnets from the kube-controller-manager Pod. 

    If the kube-controller-manager component in your cluster runs in systemd mode instead of as a static Pod, Spiderpool still cannot retrieve the cluster's subnet information.

    If both of the above methods fail, Spiderpool will synchronize the status.phase as NotReady, preventing Pod creation. To address such abnormal situations, we can manually create the kubeadm-config ConfigMap and correctly configure the cluster's subnet information:

    ```shell
    export POD_SUBNET=<YOUR_POD_SUBNET>
    export SERVICE_SUBNET=<YOUR_SERVICE_SUBNET>
    cat << EOF | kubectl apply -f -
    apiVersion: v1
    kind: ConfigMap
    metadata:
      name: kubeadm-config
      namespace: kube-system
    data:
      ClusterConfiguration: |
        networking:
          podSubnet: ${POD_SUBNET}
          serviceSubnet: ${SERVICE_SUBNET}
    EOF
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
