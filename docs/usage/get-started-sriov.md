# SRIOV Quick Start

**English** | [**简体中文**](./get-started-sriov-zh_CN.md)

Spiderpool provides a solution for assigning static IP addresses in underlay networks. In this page, we'll demonstrate how to build a complete underlay network solution using [Multus](https://github.com/k8snetworkplumbingwg/multus-cni), [Macvlan](https://github.com/containernetworking/plugins/tree/main/plugins/main/macvlan), [Veth](https://github.com/spidernet-io/plugins), and [Spiderpool](https://github.com/spidernet-io/spiderpool), which meets the following kinds of requirements:

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
   

## Install Veth

[`Veth`](https://github.com/spidernet-io/plugins) is a CNI plugin designed to resolve the following issues in other CNIs like Macvlan and SR-IOV:

* Enable clusterIP communication for Pods in the Sriov CNI scenario

* Address communication issues in multiple NICs for Pods by automatically coordinating policy routing between NICs

Download and install the Veth binary on all nodes:

```shell
~# wget https://github.com/spidernet-io/plugins/releases/download/v0.1.4/spider-plugins-linux-amd64-v0.1.4.tar

~# tar xvfzp ./spider-plugins-linux-amd64-v0.1.4.tar -C /opt/cni/bin

~# chmod +x /opt/cni/bin/veth
```

## Create Sriov Configmap that matches the NIC configuration

* Check if the vendor, deviceID and driver information of the NIC are in Configmap.

    ```shell
    ~# ethtool -i enp4s0f0np0 |grep -e driver -e bus-info
    driver: mlx5_core
    bus-info: 0000:04:00.0
    ~#
    ~# lspci -s 0000:04:00.0 -n
    04:00.0 0200: 15b3:1018
    ```

    > In this example, the vendor is 15b3, the deviceID is 1018, and the driver is mlx5_core.

* Create Configmap

    ```shell
    vendor="15b3"
    deviceID="1018"
    driver="mlx5_core"
    cat <<EOF | kubectl apply -f -
    apiVersion: v1
    kind: ConfigMap
    metadata:
        name: sriovdp-config
        namespace: kube-system
    data:
        config.json: |
        {
            "resourceList": [{
                    "resourceName": "mlnx_sriov",
                    "selectors": {
                        "vendors": [ "$vendor" ],
                        "devices": [ "$deviceID" ],
                        "drivers": [ "$driver" ]
                        }
                }
            ]
        }
    EOF
    ```

    > resourceName is the name of the Sriov resource, and after it is declared in Configmap, a Sriov resource named `intel.com/mlnx_sriov` will be generated on the node for Pods after the sriov-plugin takes effect. The prefix `intel.com` can be defined through the `resourcePrefix` field.
    > Refer to [Sriov Configmap](https://github.com/k8snetworkplumbingwg/sriov-network-device-plugin#configurations) for more configuration rules.


## Create Sriov VFs

1. Check the current number of VFs:

    ```shell
    ~# cat /sys/class/net/enp4s0f0np0/device/sriov_numvfs
    0
    ```
   
2. Create 8 VFs

    ```shell
    ~# echo 8 > /sys/class/net/enp4s0f0np0/device/sriov_numvfs
    ```

    > Refer to [Setting up Virtual Functions](https://github.com/k8snetworkplumbingwg/sriov-network-device-plugin/blob/master/docs/vf-setup.md) for more details.

## Install Sriov Device Plugin 

```shell
~# kubectl apply -f https://raw.githubusercontent.com/k8snetworkplumbingwg/sriov-network-device-plugin/v3.5.1/deployments/k8s-v1.16/sriovdp-daemonset.yaml
```

Wait for the plugin to take effect After installation.

* Check the Node and verify that the Sriov resource named `intel.com/mlnx_sriov` defined in Configmap has taken effect, with 8 as the number of VFs:

    ```shell
    ~# kubectl get  node  master-11 -ojson |jq '.status.allocatable'
    {
      "cpu": "24",
      "ephemeral-storage": "94580335255",
      "hugepages-1Gi": "0",
      "hugepages-2Mi": "0",
      "intel.com/mlnx_sriov": "8",
      "memory": "16247944Ki",
      "pods": "110"
    }
    ```

## Install Sriov CNI

1. Install Sriov CNI through manifest:

    ```shell
    ~# kubectl apply -f https://raw.githubusercontent.com/k8snetworkplumbingwg/sriov-cni/v2.7.0/images/k8s-v1.16/sriov-cni-daemonset.yaml
    ```

## Install Multus

1. Install Multus through manifest:

    ```shell
    ~# kubectl apply -f https://raw.githubusercontent.com/k8snetworkplumbingwg/multus-cni/v3.9/deployments/multus-daemonset.yml
    ```

2. Create a NetworkAttachmentDefinition configuration for Sriov in Multus

   To implement clusterIP communication using Veth, confirm the service CIDR of the cluster through a query command `kubectl -n kube-system get configmap kubeadm-config -oyaml | grep service` or other methods:

    ```shell
    SERVICE_CIDR="10.43.0.0/16"
    cat <<EOF | kubectl apply -f -
    apiVersion: k8s.cni.cncf.io/v1
    kind: NetworkAttachmentDefinition
    metadata:
      annotations:
        k8s.v1.cni.cncf.io/resourceName: intel.com/mlnx_sriov
      name: sriov-test
      namespace: kube-system
    spec:
      config: |-
        {
            "cniVersion": "0.3.1",
            "name": "sriov-test",
            "plugins": [
                {
                    "type": "sriov",
                    "ipam": {
                        "type": "spiderpool"
                    }
                },{
                      "type": "veth",
                      "service_cidr": ["${SERVICE_CIDR}"]
                  }
            ]
        }
   EOF
    ```

   > `k8s.v1.cni.cncf.io/resourceName: intel.com/mlnx_sriov`: the name of the Sriov resource to be used


## Install Spiderpool

1. Install Spiderpool CRD

    ```shell
    helm repo add spiderpool https://spidernet-io.github.io/spiderpool
    helm repo update spiderpool
    helm install spiderpool spiderpool/spiderpool --namespace kube-system \
        --set feature.enableIPv4=true --set feature.enableIPv6=false 
    ```
   
2. Create a SpiderSubnet instance.
   
   The Pod will obtain an IP address from this subnet for underlying network communication, so the subnet needs to correspond to the underlying subnet that is being accessed.
   Here is an example of creating a SpiderSubnet instance:：
    
   ```shell
   cat <<EOF | kubectl apply -f -
   apiVersion: spiderpool.spidernet.io/v2beta1
   kind: SpiderSubnet
   metadata:
     name: subnet-test
   spec:
     ipVersion: 4
     ips:
     - "10.20.168.190-10.20.168.199"
     subnet: 10.20.0.0/16
     gateway: 10.20.0.1
     vlan: 0
   EOF
   ```

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
           ipam.spidernet.io/subnet: |-
             {
               "ipv4": ["subnet-test"]
             }
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
               intel.com/mlnx_sriov: '1' 
             limits:
               intel.com/mlnx_sriov: '1'  
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
   > `intel.com/mlnx_sriov`: Sriov resources used.
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

3. Spiderpool has created fixed IP pools for applications, ensuring that the applications' IPs are automatically fixed within the defined ranges.

   ```shell
   ~# kubectl get spiderippool
   NAME                                     VERSION   SUBNET         ALLOCATED-IP-COUNT   TOTAL-IP-COUNT   DEFAULT   DISABLE
   auto-sriov-deploy-v4-eth0-f5488b112fd9   4         10.20.0.0/16   2                    2                false     false
   
   ~#  kubectl get spiderendpoints
   NAME                           INTERFACE   IPV4POOL                                 IPV4               IPV6POOL   IPV6   NODE
   sriov-deploy-9b4b9f6d9-mmpsm   eth0        auto-sriov-deploy-v4-eth0-f5488b112fd9   10.20.168.191/16                     worker-12
   sriov-deploy-9b4b9f6d9-xfsvj   eth0        auto-sriov-deploy-v4-eth0-f5488b112fd9   10.20.168.190/16                     master-11
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
