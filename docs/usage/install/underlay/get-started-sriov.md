# SR-IOV Quick Start

**English** | [**简体中文**](./get-started-sriov-zh_CN.md)

Spiderpool provides a solution for assigning static IP addresses in underlay networks. In this page, we'll demonstrate how to build a complete underlay network solution using [Multus](https://github.com/k8snetworkplumbingwg/multus-cni), [SR-IOV](https://github.com/k8snetworkplumbingwg/sriov-cni), [Veth](https://github.com/spidernet-io/plugins), and [Spiderpool](https://github.com/spidernet-io/spiderpool), which meets the following kinds of requirements:

* Applications can be assigned static Underlay IP addresses through simple operations.

* Pods with multiple Underlay NICs connect to multiple Underlay subnets.

* Pods can communicate in various ways, such as Pod IP, clusterIP, and nodePort.

## Prerequisites

1. Make sure a Kubernetes cluster is ready
2. [Helm](https://helm.sh/docs/intro/install/) has already been installed
3. [A SR-IOV-enabled NIC](https://github.com/k8snetworkplumbingwg/sriov-network-device-plugin#supported-sr-iov-nics)

    * Check the NIC's bus-info:

        ```shell
        ~# ethtool -i enp4s0f0np0 |grep bus-info
        bus-info: 0000:04:00.0
        ```

    * Check whether the NIC supports SR-IOV via bus-info. If the `Single Root I/O Virtualization (SR-IOV)` field appears, it means that SR-IOV is supported:

        ```shell
        ~# lspci -s 0000:04:00.0 -v |grep SR-IOV
        Capabilities: [180] Single Root I/O Virtualization (SR-IOV)      
        ```

4. If your OS is such as Fedora and CentOS and uses NetworkManager to manage network configurations, you need to configure NetworkManager in the following scenarios:

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

    ```shell
    helm repo add spiderpool https://spidernet-io.github.io/spiderpool
    helm repo update spiderpool
    helm install spiderpool spiderpool/spiderpool --namespace kube-system --set sriov.install=true --set multus.multusCNI.defaultCniCRName="sriov-test"
    ```

    > When using the helm option `--set sriov.install=true`, it will install the [sriov-network-operator](https://github.com/k8snetworkplumbingwg/sriov-network-operator). The default value for resourcePrefix is "spidernet.io" which can be modified via the helm option `--set sriov.resourcePrefix`.
    >
    > For users in the Chinese mainland, it is recommended to specify the spec `--set global.imageRegistryOverride=ghcr.m.daocloud.io` to avoid image pull failures from Spiderpool.
    >
    > Specify the name of the NetworkAttachmentDefinition instance for the default CNI used by Multus via `multus.multusCNI.defaultCniCRName`. If the `multus.multusCNI.defaultCniCRName` option is provided, an empty NetworkAttachmentDefinition instance will be automatically generated upon installation. Otherwise, Multus will attempt to create a NetworkAttachmentDefinition instance based on the first CNI configuration found in the /etc/cni/net.d directory. If no suitable configuration is found, a NetworkAttachmentDefinition instance named `default` will be created to complete the installation of Multus.

2. To enable the SR-IOV CNI on specific nodes, you need to apply the following command to label those nodes. This will allow the sriov-network-operator to install the components on the designated nodes.

    ```shell
    kubectl label node $NodeName node-role.kubernetes.io/worker=""
    ```

3. Create VFs on the node

    Use the following command to view the available network interfaces on the node:

    ```shell
    $ kubectl get sriovnetworknodestates -n kube-system
    NAME                   SYNC STATUS   AGE
    node-1                 Succeeded     24s
    ...

    $ kubectl get sriovnetworknodestates -n kube-system node-1 -o yaml
    apiVersion: sriovnetwork.openshift.io/v1
    kind: SriovNetworkNodeState
    spec: ...
    status:
      interfaces:
      - deviceID: "1017"
        driver: mlx5_core
        linkSpeed: 10000 Mb/s
        linkType: ETH
        mac: 04:3f:72:d0:d2:86
        mtu: 1500
        name: enp4s0f0np0
        pciAddress: "0000:04:00.0"
        totalvfs: 8
        vendor: 15b3
      syncStatus: Succeeded
    ```

    > If the status of SriovNetworkNodeState CRs is `InProgress`,  it indicates that the sriov-operator is currently synchronizing the node state. Wait for the status to become `Succeeded` to confirm that the synchronization is complete. Check the CR to ensure that the sriov-network-operator has discovered the network interfaces on the node that support SR-IOV.

    Based on the given information, it is known that the network interface's `enp4s0f0np0` on the node `node-1` supports SR-IOV capability with a maximum of 8 VFs. Now, let's create SriovNetworkNodePolicy CRs and specify PF (Physical function, physical network interface) through `nicSelector.pfNames` to generate VFs(Virtual Function) on these network interfaces of the respective nodes:

    ```shell
    $ cat << EOF | kubectl apply -f -
    apiVersion: sriovnetwork.openshift.io/v1
    kind: SriovNetworkNodePolicy
    metadata:
      name: policy1
      namespace: sriov-network-operator
    spec:
      deviceType: netdevice
      nodeSelector:
        kubernetes.io/os: "linux"
      nicSelector:
        pfNames:
          - enp4s0f0np0
      numVfs: 8 # desired number of VFs
      resourceName: sriov_netdevice
    EOF
    ```

    > After executing the above command, please note that configuring nodes to enable SR-IOV functionality may require a node restart. If needed, specify worker nodes instead of master nodes for this configuration.
    > The resourceName should not contain special characters and is limited to [0-9], [a-zA-Z], and "_".

    After applying the SriovNetworkNodePolicy CRs, you can check the status of the SriovNetworkNodeState CRs again to verify that the VFs have been successfully configured:

    ```shell
    $ kubectl get sriovnetworknodestates -n sriov-network-operator node-1 -o yaml
    ...
    - Vfs:
        - deviceID: 1018
          driver: mlx5_core
          pciAddress: 0000:04:00.4
          vendor: "15b3"
        - deviceID: 1018
          driver: mlx5_core
          pciAddress: 0000:04:00.5
          vendor: "15b3"
        - deviceID: 1018
          driver: mlx5_core
          pciAddress: 0000:04:00.6
          vendor: "15b3"
        deviceID: "1017"
        driver: mlx5_core
        mtu: 1500
        numVfs: 8
        pciAddress: 0000:04:00.0
        totalvfs: 8
        vendor: "8086"
    ...
    ```

    To confirm that the SR-IOV resources named `spidernet.io/sriov_netdevice` have been successfully enabled on a specific node and that the number of VFs is set to 8, you can use the following command:

    ```shell
    ~# kubectl get  node  node-1 -o json |jq '.status.allocatable'
    {
      "cpu": "24",
      "ephemeral-storage": "94580335255",
      "hugepages-1Gi": "0",
      "hugepages-2Mi": "0",
      "spidernet.io/sriov_netdevice": "8",
      "memory": "16247944Ki",
      "pods": "110"
    }
    ```

    > The sriov-network-config-daemon Pod is responsible for configuring VF on nodes, and it will sequentially complete the work on each node. When configuring VF on each node, the SR-IOV network configuration daemon will evict all Pods on the node, configure VF, and possibly restart the node. When SR-IOV network configuration daemon fails to evict a Pod, it will cause all processes to stop, resulting in the vf number of nodes remaining at 0. In this case, the SR-IOV network configuration daemon Pod will see logs similar to the following:
    >
    > `error when evicting pods/calico-kube-controllers-865d498fd9-245c4 -n kube-system (will retry after 5s) ...`
    >
    > This issue can be referred to similar topics in the sriov-network-operator community [issue](https://github.com/k8snetworkplumbingwg/sriov-network-operator/issues/463)
    >
    > The reason why the designated Pod cannot be expelled can be investigated, which may include the following:
    >
    > (1) The Pod that failed the eviction may have been configured with a PodDisruptionBudget, resulting in a
    > shortage of available replicas. Please adjust the PodDisruptionBudget
    >
    > (2) Insufficient available nodes in the cluster, resulting in no nodes available for scheduling

4. Create a SpiderIPPool instance.

    The Pod will obtain an IP address from this subnet for underlying network communication, so the subnet needs to correspond to the underlying subnet that is being accessed.
    Here is an example of creating a SpiderSubnet instance:：

    ```shell
    cat <<EOF | kubectl apply -f -
    apiVersion: spiderpool.spidernet.io/v2beta1
    kind: SpiderIPPool
    metadata:
      name: ippool-test
    spec:
      default: true
      ips:
      - "10.20.168.190-10.20.168.199"
      subnet: 10.20.0.0/16
      gateway: 10.20.0.1
      multusName: kube-system/sriov-test
    EOF
    ```

5. Create a SpiderMultusConfig instance.

    ```shell
    $ cat <<EOF | kubectl apply -f -
    apiVersion: spiderpool.spidernet.io/v2beta1
    kind: SpiderMultusConfig
    metadata:
      name: sriov-test
      namespace: kube-system
    spec:
      cniType: sriov
      sriov:
        resourceName: spidernet.io/sriov_netdevice
      ```

    > SpiderIPPool.Spec.multusName: 'kube-system/sriov-test' must be to match the Name and Namespace of the SpiderMultusConfig instance created.

## Create applications

1. Create test Pods and Services via the command below：

    ```shell
    cat <<EOF | kubectl create -f -
    apiVersion: apps/v1
    kind: Deployment
    metadata:
      name: sriov-deploy
    spec:
      replicas: 2
      selector:
        matchLabels:
          app: sriov-deploy
      template:
        metadata:
          annotations:
            v1.multus-cni.io/default-network: kube-system/sriov-test
          labels:
            app: sriov-deploy
        spec:
          containers:
          - name: sriov-deploy
            image: nginx
            imagePullPolicy: IfNotPresent
            ports:
            - name: http
              containerPort: 80
              protocol: TCP
            resources:
              requests:
                spidernet/sriov_netdevice: '1' 
              limits:
                spidernet/sriov_netdevice: '1'  
    ---
    apiVersion: v1
    kind: Service
    metadata:
      name: sriov-deploy-svc
      labels:
        app: sriov-deploy
    spec:
      type: ClusterIP
      ports:
        - port: 80
          protocol: TCP
          targetPort: 80
      selector:
        app: sriov-deploy 
    EOF
    ```

    Spec descriptions:

    > `spidernet/sriov_netdevice`: Sriov resources used.
    >
    >`v1.multus-cni.io/default-network`: specifies the CNI configuration for Multus.
    >
    > For more information on Multus annotations, refer to [Multus Quickstart](https://github.com/k8snetworkplumbingwg/multus-cni/blob/master/docs/quickstart.md).

2. Check the status of Pods:

    ```shell
    ~# kubectl get pod -l app=sriov-deploy -owide
    NAME                           READY   STATUS    RESTARTS   AGE     IP              NODE        NOMINATED NODE   READINESS GATES
    sriov-deploy-9b4b9f6d9-mmpsm   1/1     Running   0          6m54s   10.20.168.191   worker-12   <none>           <none>
    sriov-deploy-9b4b9f6d9-xfsvj   1/1     Running   0          6m54s   10.20.168.190   master-11   <none>           <none>
    ```

3. Spiderpool ensuring that the applications' IPs are automatically fixed within the defined ranges.

    ```shell
    ~# kubectl get spiderippool
    NAME         VERSION   SUBNET         ALLOCATED-IP-COUNT   TOTAL-IP-COUNT   DEFAULT   DISABLE
    ippool-test  4         10.20.0.0/16   2                    10               true      false
   
    ~#  kubectl get spiderendpoints
    NAME                           INTERFACE   IPV4POOL      IPV4               IPV6POOL   IPV6   NODE
    sriov-deploy-9b4b9f6d9-mmpsm   eth0        ippool-test   10.20.168.191/16                     worker-12
    sriov-deploy-9b4b9f6d9-xfsvj   eth0        ippool-test   10.20.168.190/16                     master-11
    ```

4. Test the communication between Pods:

    ```shell
    ~# kubectl exec -it sriov-deploy-9b4b9f6d9-mmpsm -- ping 10.20.168.190 -c 3
    PING 10.20.168.190 (10.20.168.190) 56(84) bytes of data.
    64 bytes from 10.20.168.190: icmp_seq=1 ttl=64 time=0.162 ms
    64 bytes from 10.20.168.190: icmp_seq=2 ttl=64 time=0.138 ms
    64 bytes from 10.20.168.190: icmp_seq=3 ttl=64 time=0.191 ms
   
    --- 10.20.168.190 ping statistics ---
    3 packets transmitted, 3 received, 0% packet loss, time 2051ms
    rtt min/avg/max/mdev = 0.138/0.163/0.191/0.021 ms
    ```

5. Test the communication between Pods and Services:

    * Check Services' IPs：

        ```shell
        ~# kubectl get svc
        NAME               TYPE        CLUSTER-IP     EXTERNAL-IP   PORT(S)              AGE
        kubernetes         ClusterIP   10.43.0.1      <none>        443/TCP              23d
        sriov-deploy-svc   ClusterIP   10.43.54.100   <none>        80/TCP               20m
        ```

    * Access its own service within the Pod:

        ```shell
        ~# kubectl exec -it sriov-deploy-9b4b9f6d9-mmpsm -- curl 10.43.54.100 -I
        HTTP/1.1 200 OK
        Server: nginx/1.23.3
        Date: Mon, 27 Mar 2023 08:22:39 GMT
        Content-Type: text/html
        Content-Length: 615
        Last-Modified: Tue, 13 Dec 2022 15:53:53 GMT
        Connection: keep-alive
        ETag: "6398a011-267"
        Accept-Ranges: bytes
        ```
