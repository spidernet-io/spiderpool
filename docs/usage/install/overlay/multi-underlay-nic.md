# Calico with multi underlay NIC

**English** | [**简体中文**](./multi-underlay-nic-zh_CN.md)

## Auto attach multiple underlay NICs to Pod based on Webhook

    The subnet for the interface `ens192` on the cluster nodes here is `10.6.0.0/16`. The subnet for the interface `ens193` on the cluster nodes here is `10.7.0.0/16`. Create  SpiderIPPools using these subnets:

    ```shell
    $ cat <<EOF | kubectl apply -f -
    apiVersion: spiderpool.spidernet.io/v2beta1
    kind: SpiderIPPool
    metadata:
      name: macvlan-ens192
    spec:
      disable: false
      gateway: 10.6.0.1
      subnet: 10.6.0.0/16
      ips:
        - 10.6.212.100-10.6.212.200
    ---
    apiVersion: spiderpool.spidernet.io/v2beta1
    kind: SpiderIPPool
    metadata:
      name: macvlan-ens193
    spec:
      disable: false
      gateway: 10.7.0.1
      subnet: 10.7.0.0/16
      ips:
        - 10.7.212.100-10.7.212.200
    ---
    apiVersion: spiderpool.spidernet.io/v2beta1
    kind: SpiderMultusConfig
    metadata:
      name: macvlan-ens192
      namespace: spiderpool
      annotations:
        cni.spidernet.io/network-resource-inject: multi-network
    spec:
      cniType: macvlan
      macvlan:
        master:
        - ens192
        ippools:
          ipv4:
          - macvlan-ens192
        vlanID: 0
    ---
    apiVersion: spiderpool.spidernet.io/v2beta1
    kind: SpiderMultusConfig
    metadata:
      name: macvlan-ens193
      namespace: spiderpool
      annotations:
        cni.spidernet.io/network-resource-inject: multi-network
    spec:
      cniType: macvlan
      macvlan:
        master:
        - ens193
        ippools:
          ipv4:
          - macvlan-ens193
        vlanID: 0
    EOF
    ```

## Create an application

1. Add the same annotation to the application:

    ```yaml
    ...
    spec:
      template:
        metadata:
          annotations:
            cni.spidernet.io/network-resource-inject: multi-network
    ```

    > Note: When using the webhook automatic injection of network resources feature, do not add other network configuration annotations (such as `k8s.v1.cni.cncf.io/networks` and `ipam.spidernet.io/ippools`) to the application, as it will affect the automatic injection of resources.

2. Once the Pod is created, you can observe that the Pod has been automatically injected with network card annotations.

    ```yaml
    ...
    spec:
      template:
        metadata:
          annotations:
              k8s.v1.cni.cncf.io/networks: |-
                [{"name":"macvlan-ens192","namespace":"spiderpool"},
                {"name":"macvlan-ens193","namespace":"spiderpool"}]
         ....
    ```
