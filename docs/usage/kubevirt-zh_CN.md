# Kubevirt

**简体中文** | [**English**](./kubevirt.md)

## 介绍

*Spiderpool 能保证 kubevirt vm 的 Pod 在重启、重建场景下，持续获取到相同的 IP 地址。*

## Kubevirt 网络搭配

Spiderpool underlay 网络解决方案可给 Kubevirt 赋予介入 underlay 的能力:

1. 对于 Kubevirt 的 Passt 网络模式，可搭配 Spiderpool macvlan 集成方案使用。在该网络模式下，**支持** Service Mesh 的所有功能，不过只能使用**单网卡**，且不支持热迁移。

2. 对于 Kubevirt 的 Bridge 网络模式，可搭配 OVS CNI 使用。在该网络模式下，**不支持** Service Mesh 功能，可使用**多网卡**，不支持热迁移。

## Kubevirt VM 固定地址

Kubevirt VM 会在以下一些场景中会出现固定地址的使用：

1. VM 的热迁移，期望迁移过后的 VM 仍能继承之前的 IP 地址。

2. VM 资源对应的 Pod 出现了重启的情况。

3. VM 资源对应的 VMI(VirtualMachineInstance) 资源被删除的情景。 

此外，Kubevirt VM 固定 IP 地址与 StatefulSet 的表现形式是不一样的：

1. 对于 VM ，Pod 重启前后，其 Pod 的名字是会发生变化的，但是其对应的 VMI 不论重启与否，其名字都不会发生变化。因此，我们将会以 VM 为单位来记录其固定的 IP 地址(我们的 SpiderEndpoint 资源将会继承使用 VM 资源的命名空间以及名字)。

2. 对于 StatefulSet，Pod 副本重启前后，其 Pod 名保持不变，我们 Spiderpool 会因此以 Pod 为单位来记录其固定的 IP 地址。

> Notice: 该功能默认开启。若开启，无任何限制， VM 可通过有限 IP 地址集合的 IP 池来固化 IP 的范围，但是，无论 VM 是否使用固定的 IP 池，它的 Pod 都可以持续分到相同 IP。 若关闭，VM 对应的 Pod 将被当作无状态对待，使用 Helm 安装 Spiderpool 时，可通过`--set ipam.enableKubevirtStaticIP=false` 关闭。

## 实施要求

1. 一套 Kubernetes 集群。

2. 已安装 [Helm](https://helm.sh/docs/intro/install/)。

## 步骤

以下流程将会演示 Kubevirt 的 Passt 网络模式搭配 macvlan CNI 以使得 VM 获得 underlay 接入能力，并通过 spiderpool 实现分配固定 IP 的功能。

> Notice：当前 macvlan 和 ipvlan 并不适用于 Kubevirt 的 bridge 网络模式，因为对于 bridge 网络模式会将 pod 网卡的 MAC 地址移动到VM，使得 Pod 使用另一个不同的地址。而 macvlan 和 ipvlan CNI 要求 Pod 的网卡接口具有原始 MAC 地址。

### 安装 Spiderpool

请参考 [Macvlan Quick Start](./install/underlay/get-started-macvlan-zh_CN.md)

### 创建 Kubevirt VM 应用

以下的示例 Yaml 中， 会创建 1 个 Kubevirt VM 应用 ，其中：

- `v1.multus-cni.io/default-network`：为应用选择一张默认网卡的 CNI 配置。

```bash
apiVersion: kubevirt.io/v1
kind: VirtualMachine
metadata:
  name: vm-cirros
  labels:
    kubevirt.io/vm: vm-cirros
spec:
  runStrategy: Always
  template:
    metadata:
      annotations:
        v1.multus-cni.io/default-network: kube-system/macvlan-ens192
      labels:
        kubevirt.io/vm: vm-cirros
    spec:
      domain:
        devices:
          disks:
            - name: containerdisk
              disk:
                bus: virtio
            - name: cloudinitdisk
              disk:
                bus: virtio
          interfaces:
            - name: default
              passt: {}
        resources:
          requests:
            memory: 64M
      networks:
        - name: default
          pod: {}
      volumes:
        - name: containerdisk
          containerDisk:
            image: quay.io/kubevirt/cirros-container-disk-demo
        - name: cloudinitdisk
          cloudInitNoCloud:
            userData: |
              #!/bin/sh
              echo 'printed from cloud-init userdata'
```

最终，在 Kubevirt VM 应用被创建时，Spiderpool 会从指定 IPPool 中随机选择一个 IP 来与应用形成绑定关系。

```bash
~# kubectl get spiderippool
NAME          VERSION   SUBNET        ALLOCATED-IP-COUNT   TOTAL-IP-COUNT   DEFAULT
test-ippool   4         10.6.0.0/16   1                    10               false

~# kubectl get po -l vm.kubevirt.io/name=vm-cirros -o wide
NAME                            READY   STATUS      RESTARTS   AGE     IP              NODE                   NOMINATED NODE   READINESS GATES
virt-launcher-vm-cirros-rg6fs   2/2     Running     0          3m43s   10.6.168.105    node2                  <none>           1/1
```

重启 Kubevirt VM Pod, 观察到新的 Pod 的 IP 不会变化，符合预期。

```bash
~# kubectl delete pod virt-launcher-vm-cirros-rg6fs
pod "virt-launcher-vm-cirros-rg6fs" deleted

~# kubectl get po -l vm.kubevirt.io/name=vm-cirros -o wide
NAME                            READY   STATUS      RESTARTS   AGE     IP              NODE                   NOMINATED NODE   READINESS GATES
virt-launcher-vm-cirros-d68l2   2/2     Running     0          1m21s   10.6.168.105    node2                  <none>           1/1
```

重启 Kubevirt VMI，观察到后续新的 Pod 的IP 也不会变化，符合预期。

```bash
~# kubectl delete vmi vm-cirros
virtualmachineinstance.kubevirt.io "vm-cirros" deleted

~# kubectl get po -l vm.kubevirt.io/name=vm-cirros -o wide
NAME                            READY   STATUS    RESTARTS   AGE    IP              NODE                   NOMINATED NODE   READINESS GATES
virt-launcher-vm-cirros-jjgrl   2/2     Running   0          104s   10.6.168.105    node2                  <none>           1/1
```

VM 也可与其他 underlay Pod 的通信。

```bash
~# kubectl get po -o wide
NAME                             READY   STATUS    RESTARTS   AGE     IP              NODE                   NOMINATED NODE   READINESS GATES
daocloud-2048-5855b45f44-bvmdr   1/1     Running   0          5m55s   10.6.168.108    spider-worker          <none>           <none>

~# kubectl virtctl console vm-cirros
$ ping -c 1 10.6.168.108
PING 10.6.168.108 (10.6.168.108): 56 data bytes
64 bytes from 10.6.168.108: seq=0 ttl=255 time=70.554 ms

--- 10.6.168.108 ping statistics ---
1 packets transmitted, 1 packets received, 0% packet loss
round-trip min/avg/max = 70.554/70.554/70.554 ms
```

VM 也可访问 cluster IP。

```bash
~# kubectl get svc -o wide
NAME                TYPE        CLUSTER-IP     EXTERNAL-IP   PORT(S)   AGE     SELECTOR
daocloud-2048-svc   ClusterIP   10.233.36.38   <none>        80/TCP    3m50s   app=daocloud-2048

~# curl -I 10.233.36.38:80
HTTP/1.1 200 OK
Server: nginx/1.10.1
Date: Tue, 17 Oct 2023 06:50:04 GMT
Content-Type: text/html
Content-Length: 4090
Last-Modified: Tue, 17 Oct 2023 06:40:53 GMT
Connection: keep-alive
ETag: "652e2c75-ffa"
Accept-Ranges: bytes
```

## 总结

Spiderpool 能保证 Kubevirt VM Pod 在重启、重建场景下，持续获取到相同的 IP 地址，能很好的满足 Kubevirt 虚拟机的固定 IP 需求。并可配合 macvlan 或 OVS CNI 与 Kubevirt 的多种网络模式实现 VM underlay 接入能力。
