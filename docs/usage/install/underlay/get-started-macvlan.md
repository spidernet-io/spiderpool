# Macvlan Quick Start

**English** | [**简体中文**](./get-started-macvlan-zh_CN.md)

Spiderpool provides a solution for assigning static IP addresses in underlay networks. In this page, we'll demonstrate how to build a complete underlay network solution using [Multus](https://github.com/k8snetworkplumbingwg/multus-cni), [Macvlan](https://github.com/containernetworking/plugins/tree/main/plugins/main/macvlan) and [Spiderpool](https://github.com/spidernet-io/spiderpool), which meets the following kinds of requirements:

* Applications can be assigned static Underlay IP addresses through simple operations.

* Pods with multiple Underlay NICs connect to multiple Underlay subnets.

* Pods can communicate in various ways, such as Pod IP, clusterIP, and nodePort.

## Prerequisites

1. [System requirements](./../system-requirements.md)

2. Make sure a Kubernetes cluster is ready.

3. [Helm](https://helm.sh/docs/intro/install/) has been already installed.

4. If your OS is such as Fedora and CentOS and uses NetworkManager to manage network configurations, you need to configure NetworkManager in the following scenarios:

    * If you are using Underlay mode, the plugin `coordinator` will create veth interfaces on the host. To prevent interference from NetworkManager with the veth interface. It is strongly recommended that you configure NetworkManager.

    * If you want to create VLAN and Bond interfaces through [Ifacer plugin](./../../../reference/plugin-ifacer.md), NetworkManager may interfere with these interfaces, leading to abnormal pod access. It is strongly recommended that you configure NetworkManager.

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
    helm install spiderpool spiderpool/spiderpool --namespace kube-system --set multus.multusCNI.defaultCniCRName="macvlan-conf" 
    ```

    > If Macvlan is not installed in your cluster, you can specify the Helm parameter `--set plugins.installCNI=true` to install Macvlan in your cluster.
    >
    > If you are mainland user who is not available to access ghcr.io，You can specify the parameter `-set global.imageRegistryOverride=ghcr.m.daocloud.io` to avoid image pulling failures for Spiderpool.
    >
    > Specify the name of the NetworkAttachmentDefinition instance for the default CNI used by Multus via `multus.multusCNI.defaultCniCRName`. If the `multus.multusCNI.defaultCniCRName` option is provided, an empty NetworkAttachmentDefinition instance will be automatically generated upon installation. Otherwise, Multus will attempt to create a NetworkAttachmentDefinition instance based on the first CNI configuration found in the /etc/cni/net.d directory. If no suitable configuration is found, a NetworkAttachmentDefinition instance named `default` will be created to complete the installation of Multus.

2. Create a SpiderIPPool instance.

    Create an IP Pool in the same subnet as the network interface `eth0` for Pods to use, the following is an example of creating a related SpiderIPPool:

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
      - kube-system/macvlan-conf
    EOF
    ```

3. Verify installation

   ```shell
    ~# kubectl get po -n kube-system | grep spiderpool
    spiderpool-agent-7hhkz                   1/1     Running     0              13m
    spiderpool-agent-kxf27                   1/1     Running     0              13m
    spiderpool-controller-76798dbb68-xnktr   1/1     Running     0              13m
    spiderpool-init                          0/1     Completed   0              13m
    ~# kubectl get sp
    NAME            VERSION   SUBNET          ALLOCATED-IP-COUNT   TOTAL-IP-COUNT   DISABLE
    ippool-test     4         172.18.0.0/16   0                    10               false
   ```

## Create CNI configuration

To simplify writing Multus CNI configuration in JSON format, Spiderpool provides SpiderMultusConfig CR to automatically manage **Multus NetworkAttachmentDefinition CR**. Here is an example of creating a Macvlan SpiderMultusConfig configuration:

* Verify the required host parent interface for Macvlan. In this case, a Macvlan sub-interface will be created for Pods from the host parent interface --eth0.

    > * If there is a VLAN requirement, you can specify the VLAN ID in the `spec.vlanID` field. We will create the corresponding VLAN sub-interface for the network card.
    > * We also provide support for network card bonding. Just specify the name of the bond network card and its mode in the `spec.bond.name` and `spec.bond.mode` respectively. We will automatically combine multiple network cards into one bonded network card for you.

    ```shell
    MACVLAN_MASTER_INTERFACE="eth0"
    cat <<EOF | kubectl apply -f -
    apiVersion: spiderpool.spidernet.io/v2beta1
    kind: SpiderMultusConfig
    metadata:
      name: macvlan-conf
      namespace: kube-system
    spec:
      cniType: macvlan
      macvlan:
        master:
        - ${MACVLAN_MASTER_INTERFACE}
    EOF
    ```

In the example of this article, use the above configuration to create the following Macvlan SpiderMultusConfig, which will automatically generate **Multus NetworkAttachmentDefinition CR** based on it, which corresponds to the eth0 network card of the host.

```bash
~# kubectl get spidermultusconfigs.spiderpool.spidernet.io -n kube-system
NAME           AGE
macvlan-conf   10m

~# kubectl get network-attachment-definitions.k8s.cni.cncf.io -n kube-system
NAME           AGE
macvlan-conf   10m
```

## Create applications

1. Create test Pods and service via the command below：

    ```bash
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
            v1.multus-cni.io/default-network: kube-system/macvlan-conf
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
    ---
    apiVersion: v1
    kind: Service
    metadata:
      name: test-app-svc
      labels:
        app: test-app
    spec:
      type: ClusterIP
      ports:
        - port: 80
          protocol: TCP
          targetPort: 80
      selector:
        app: test-app 
    EOF
    ```

2. Check the status of Pods:

    ```bash
    ~# kubectl get po -l app=test-app -o wide
    NAME                      READY   STATUS    RESTARTS   AGE     IP              NODE                 NOMINATED NODE   READINESS GATES
    test-app-f9f94688-2srj7   1/1     Running   0          2m13s   172.18.30.139   ipv4-worker          <none>           <none>
    test-app-f9f94688-8982v   1/1     Running   0          2m13s   172.18.30.138   ipv4-control-plane   <none>           <none>
    ```

3. Spiderpool has created fixed IP pools for applications, ensuring that the applications' IPs are automatically fixed within the defined ranges.

    ```bash
    ~# kubectl get spiderippool
    NAME          VERSION   SUBNET           ALLOCATED-IP-COUNT   TOTAL-IP-COUNT   DEFAULT    
    ippool-test   4         172.18.0.0/16    2                    10                false
    
    ~#  kubectl get spiderendpoints
    NAME                      INTERFACE   IPV4POOL      IPV4               IPV6POOL   IPV6   NODE                 CREATETION TIME
    test-app-f9f94688-2srj7   eth0        ippool-test   172.18.30.139/16                     ipv4-worker          3m5s
    test-app-f9f94688-8982v   eth0        ippool-test   172.18.30.138/16                     ipv4-control-plane   3m5s
    ```

4. Test the communication between Pods:

    ```shell
    ~# kubectl exec -ti test-app-f9f94688-2srj7 -- ping 172.18.30.138 -c 2
    
    PING 172.18.30.138 (172.18.30.138): 56 data bytes
    64 bytes from 172.18.30.138: seq=0 ttl=64 time=1.524 ms
    64 bytes from 172.18.30.138: seq=1 ttl=64 time=0.194 ms

    --- 172.18.30.138 ping statistics ---
    2 packets transmitted, 2 packets received, 0% packet loss
    round-trip min/avg/max = 0.194/0.859/1.524 ms
    ```

5. Test the communication between Pods and service IP:

    ```shell
    ~# kubectl get service
    
    NAME           TYPE        CLUSTER-IP    EXTERNAL-IP   PORT(S)   AGE
    kubernetes     ClusterIP   10.96.0.1     <none>        443/TCP   20h
    test-app-svc   ClusterIP   10.96.190.4   <none>        80/TCP    109m
    
    ~# kubectl exec -ti  test-app-85cf87dc9c-7dm7m -- curl 10.96.190.4:80 -I

    HTTP/1.1 200 OK
    Server: nginx/1.23.1
    Date: Thu, 23 Mar 2023 05:01:04 GMT
    Content-Type: text/html
    Content-Length: 4055
    Last-Modified: Fri, 23 Sep 2022 02:53:30 GMT
    Connection: keep-alive
    ETag: "632d1faa-fd7"
    Accept-Ranges: bytes
    ```
