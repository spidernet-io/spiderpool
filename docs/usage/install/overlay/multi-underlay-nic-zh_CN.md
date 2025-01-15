# Calico with multi underlay NIC

[**English**](./multi-underlay-nic.md) | **简体中文**

## 基于 Webhook 自动为 Pod 附加多张 Underlay 网卡

  本文集群节点网卡: `ens192` 所在子网为 `10.6.0.0/16`，`ens193` 所在子网为 `10.7.0.0/16`，以此创建 SpiderIPPool:

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

## 创建测试应用

1. 为应用也添加相同注解:

    ```yaml
    ...
    spec:
      template:
        metadata:
          annotations:
            cni.spidernet.io/network-resource-inject: multi-network
    ```

    > 注意：使用 webhook 自动注入网络资源功能时，不能为应用添加其他网络配置注解(如 `k8s.v1.cni.cncf.io/networks` 和 `ipam.spidernet.io ippools`等)，否则会影响资源自动注入功能。

2. 当 Pod 被创建后，可观测到 Pod 被自动注入了网卡 annotation

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
