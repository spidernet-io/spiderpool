# SRIOV Quick Start

**English** | [**简体中文**](./get-started-sriov-zh_CN.md)

Spiderpool provides a solution for assigning static IP addresses in underlay networks. In this page, we'll demonstrate how to build a complete underlay network solution using [Multus](https://github.com/k8snetworkplumbingwg/multus-cni), [Sriov](https://github.com/k8snetworkplumbingwg/sriov-cni), [Veth](https://github.com/spidernet-io/plugins), and [Spiderpool](https://github.com/spidernet-io/spiderpool), which meets the following kinds of requirements:

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

## Install Sriov-network-operator

SriovNetwork helps us install sriov-cni and sriov-device-plugin components, making it easier to use sriov-cni:

1. Install sriov-network-operator

    ```shell
    git clone https://github.com/k8snetworkplumbingwg/sriov-network-operator.git && cd sriov-network-operator/deployment
    helm install -n sriov-network-operator --create-namespace --set operator.resourcePrefix=spidernet.io  --wait sriov-network-operator ./
    ```

   > You may need to label SR-IOV worker nodes using node-role.kubernetes.io/worker="" label, if not already.
   >
   > By default, SR-IOV Operator will be deployed in namespace 'openshift-sriov-network-operator'.
   >
   > After installation, the node may reboot automatically. If necessary, install sriov-network-operator to the designated worker nodes.

2. Configure sriov-network-operator

    Firstly, check the status of SriovNetworkNodeState CRs to confirm that sriov-network-operator has found a network card on the node that supports the SR-IOV feature.

    ```shell
    $ kubectl get sriovnetworknodestates.sriovnetwork.openshift.io -n sriov-network-operator node-1 -o yaml
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
      - deviceID: "1017"
        driver: mlx5_core
        linkSpeed: 10000 Mb/s
        linkType: ETH
        mac: 04:3f:72:d0:d2:87
        mtu: 1500
        name: enp4s0f1np1
        pciAddress: "0000:04:00.1"
        totalvfs: 8
        vendor: 15b3
      syncStatus: Succeeded
    ```

    As can be seen from the above, interfaces 'enp4s0f0np0' and 'enp4s0f1np1' on node 'node-1' both have SR-IOV capability and support a maximum number of VFs of 8. Below we will configure the VFs by creating SriovNetworkNodePolicy CRs and install sriov-device-plugin:

    ```shell
    $ cat << EOF | kubectl apply -f -
    apiVersion: sriovnetwork.openshift.io/v1
    kind: SriovNetworkNodePolicy
    metadata:
      name: policy1
      namespace: sriov-network-operator
    spec:
      deviceType: netdevice
      nicSelector:
      pfNames:
      - enp4s0f0np0
      nodeSelector:
        kubernetes.io/hostname: node-1 
      numVfs: 8 # The number of VFs desired
      resourceName: sriov_netdevice
    ```

    After creating the SriovNetworkNodePolicy CRs, check the status of the SriovNetworkNodeState CRs again to see that the VFs in status have been configured:

    ```shell
    $ kubectl get sriovnetworknodestates.sriovnetwork.openshift.io -n sriov-network-operator node-1 -o yaml
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

    You can see the sriov resource named 'spidernet/sriov_netdevice' has been configured in Node object, where the number of VFs is 8:

    ```shell
    ~# kubectl get  node  node-1 -o json |jq '.status.allocatable'
    {
      "cpu": "24",
      "ephemeral-storage": "94580335255",
      "hugepages-1Gi": "0",
      "hugepages-2Mi": "0",
      "spidernet/sriov_netdevice": "8",
      "memory": "16247944Ki",
      "pods": "110"
    }
    ```
   
    > The sriov-network-config-daemon pod is responsible for configuring VF on nodes, and it will sequentially complete the work on each node. When configuring VF on each node, the sriov network configuration daemon will evict all PODs on the node, configure VF, and possibly restart the node. When sriov network configuration daemon fails to evict a POD, it will cause all processes to stop, resulting in the vf number of nodes remaining at 0. In this case, the sriov network configuration daemon POD will see logs similar to the following:
    > 
    > `error when evicting pods/calico-kube-controllers-865d498fd9-245c4 -n kube-system (will retry after 5s) ...`
    >
    > This issue can be referred to similar topics in the sriov-network-operator community [issue](https://github.com/k8snetworkplumbingwg/sriov-network-operator/issues/463)
    >
    > The reason why the designated POD cannot be expelled can be investigated, which may include the following:
    >
    > (1) The POD that failed the eviction may have been configured with a PodDisruptionBudget, resulting in a 
    > shortage of available replicas. Please adjust the PodDisruptionBudget
    >
    > (2) Insufficient available nodes in the cluster, resulting in no nodes available for scheduling

## Install Spiderpool

1. Install Spiderpool.

    ```shell
    helm repo add spiderpool https://spidernet-io.github.io/spiderpool
    helm repo update spiderpool
    helm install spiderpool spiderpool/spiderpool --namespace kube-system
    ```

    > If you are mainland user who is not available to access ghcr.io，You can specify the parameter `-set global.imageRegistryOverride=ghcr.m.daocloud.io` to avoid image pulling failures for Spiderpool.

2. Create a SpiderIPPool instance.

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

3. Create a SpiderMultusConfig instance.

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

    > Note: SpiderIPPool.Spec.multusName: 'kube-system/sriov-test' must be to match the Name and Namespace of the SpiderMultusConfig instance created.

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
