# Macvlan Quick Start

**English** | [**简体中文**](./get-started-macvlan-zh_CN.md)

Spiderpool provides a solution for assigning static IP addresses in underlay networks. In this page, we'll demonstrate how to build a complete underlay network solution using [Multus](https://github.com/k8snetworkplumbingwg/multus-cni), [Macvlan](https://github.com/containernetworking/plugins/tree/main/plugins/main/macvlan), [Veth](https://github.com/spidernet-io/plugins), and [Spiderpool](https://github.com/spidernet-io/spiderpool), which meets the following kinds of requirements:

* Applications can be assigned static Underlay IP addresses through simple operations.

* Pods with multiple Underlay NICs connect to multiple Underlay subnets.

* Pods can communicate in various ways, such as Pod IP, clusterIP, and nodePort.

## Prerequisites

1. Make sure a Kubernetes cluster is ready.

2. [Helm](https://helm.sh/docs/intro/install/) has been already installed.

## Install Macvlan

[`Macvlan`](https://github.com/containernetworking/plugins/tree/main/plugins/main/macvlan) is a CNI plugin that allows pods to be assigned Macvlan virtual NICs for connecting to Underlay networks.

Some Kubernetes installers include the Macvlan binary file by default that can be found at "/opt/cni/bin/macvlan" on your nodes. If the binary file is missing, you can download and install it on all nodes using the following command:

```bash
wget https://github.com/containernetworking/plugins/releases/download/v1.2.0/cni-plugins-linux-amd64-v1.2.0.tgz 
tar xvfzp ./cni-plugins-linux-amd64-v1.2.0.tgz -C /opt/cni/bin
chmod +x /opt/cni/bin/macvlan
```

## Install Veth

[`Veth`](https://github.com/spidernet-io/plugins) is a CNI plugin designed to resolve the following issues in other CNIs like Macvlan and SR-IOV:

* Enable clusterIP communication for Pods in Macvlan CNI environments

* Facilitate connection between Pods and the host during health checks where Pods' Macvlan IPs are unable to communicate with the host

* Address communication issues in multiple NICs for Pods by automatically coordinating policy routing between NICs

Download and install the Veth binary on all nodes:

```bash
wget https://github.com/spidernet-io/plugins/releases/download/v0.1.4/spider-plugins-linux-amd64-v0.1.4.tar
tar xvfzp ./spider-plugins-linux-amd64-v0.1.4.tar -C /opt/cni/bin
chmod +x /opt/cni/bin/veth
```

## Install Multus

[`Multus`](https://github.com/k8snetworkplumbingwg/multus-cni) is a CNI plugin that allows Pods to have multiple NICs by scheduling third-party CNIs. The management of the Macvlan CNI configuration is simplified through the CRD-based approach provided by Multus, with nothing for manual editing of CNI configuration files on each host.

1. Install Multus via the manifest:

    ```bash
    kubectl apply -f https://raw.githubusercontent.com/k8snetworkplumbingwg/multus-cni/v3.9/deployments/multus-daemonset.yml
    ```

2. Confirm the operational status of Multus:

    ```bash
    ~# kubectl get pods -A | grep -i multus
    kube-system          kube-multus-ds-hfzpl                         1/1     Running   0   5m
    kube-system          kube-multus-ds-qm8j7                         1/1     Running   0   5m
    ```

    Verify the existence of the Multus configuration files `ls /etc/cni/net.d/00-multus.conf` on the node.

3. Create a NetworkAttachmentDefinition for Macvlan.

    The following parameters need to be confirmed:

    * Verify the required host parent interface for Macvlan. In this case, a Macvlan sub-interface will be created for Pods from the host parent interface --eth0.

    * To implement clusterIP communication using Veth, confirm the service CIDR of the cluster through a query command `kubectl -n kube-system get configmap kubeadm-config -oyaml | grep service` or other methods.

    The following is the configuration for creating a NetworkAttachmentDefinition:

    ```bash
    MACLVAN_MASTER_INTERFACE="eth0"
    SERVICE_CIDR="10.96.0.0/16"

    cat <<EOF | kubectl apply -f -
    apiVersion: k8s.cni.cncf.io/v1
    kind: NetworkAttachmentDefinition
    metadata:
      name: macvlan-conf
      namespace: kube-system
    spec:
      config: |-
        {
            "cniVersion": "0.3.1",
            "name": "macvlan-conf",
            "plugins": [
                {
                    "type": "macvlan",
                    "master": "${MACLVAN_MASTER_INTERFACE}",
                    "mode": "bridge",
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

## Install Spiderpool

1. Install Spiderpool.

    ```bash
    helm repo add spiderpool https://spidernet-io.github.io/spiderpool
    helm repo update spiderpool
    helm install spiderpool spiderpool/spiderpool --namespace kube-system
    ```

    > If you are mainland user who is not available to access ghcr.io，You can specify the parameter `-set global.imageRegistryOverride=ghcr.m.daocloud.io` to avoid image pulling failures for Spiderpool.

2. Create a SpiderSubnet instance.

    An Underlay subnet for eth0 needs to be created for Pods as Macvlan uses eth0 as the parent interface.
    Here is an example of creating a SpiderSubnet instance:：

    ```bash
    cat <<EOF | kubectl apply -f -
    apiVersion: spiderpool.spidernet.io/v2beta1
    kind: SpiderSubnet
    metadata:
      name: subnet-test
    spec:
      ips:
      - "172.18.30.131-172.18.30.140"
      subnet: 172.18.0.0/16
      gateway: 172.18.0.1
    EOF
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
            ipam.spidernet.io/subnet: |-
              {
                "ipv4": ["subnet-test"]
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

    Spec descriptions：

    * `ipam.spidernet.io/subnet`: defines which subnets to be used to assign IP addresses to Pods.

        > For more information on Spiderpool annotations, refer to [Spiderpool Annotations](../reference/annotation.md).

    * `v1.multus-cni.io/default-network`：specifies the CNI configuration for Multus.

        > For more information on Multus annotations, refer to [Multus Quickstart](https://github.com/k8snetworkplumbingwg/multus-cni/blob/master/docs/quickstart.md).

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
    NAME                                               VERSION   SUBNET          ALLOCATED-IP-COUNT   TOTAL-IP-COUNT   DISABLE
    auto-deployment-default-test-app-v4-a0ae75eb5d47   4         172.18.0.0/16   2                    2                false
    
    ~#  kubectl get spiderendpoints
    NAME                      INTERFACE   IPV4POOL                                           IPV4               IPV6POOL   IPV6   NODE                 CREATETION TIME
    test-app-f9f94688-2srj7   eth0        auto-deployment-default-test-app-v4-a0ae75eb5d47   172.18.30.139/16                     ipv4-worker          3m5s
    test-app-f9f94688-8982v   eth0        auto-deployment-default-test-app-v4-a0ae75eb5d47   172.18.30.138/16                     ipv4-control-plane   3m5s
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
