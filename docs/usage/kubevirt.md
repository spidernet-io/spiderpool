# KubeVirt

**English** ｜ [**简体中文**](./kubevirt-zh_CN.md)

## Introduction

*Spiderpool ensures that KubeVirt VM Pods consistently obtain the same IP addresses during restart and rebuild processes.*

## Integrate with KubeVirt Networking

The Spiderpool underlay networking solution provides the ability to integrate with KubeVirt:

1. For KubeVirt's passt network mode, it can be used in conjunction with the Spiderpool macvlan integrated solution. In this network mode, all Service Mesh functionalities are **supported**. However, only **single NIC** is supported, and live migration is not available.

2. For KubeVirt's bridge network mode, it can be used in conjunction with OVS CNI. In this network mode, Service Mesh functionalities are **not supported**, but **multiple NICs** can be used, and live migration is not available.

3. Spiderpool supports IP conflict detection for KubeVirt Pods to prevent IP conflicts. However, for KubeVirt live migration applications, enabling IP conflict detection will prevent the live migration virtual machine from starting. Therefore, in this scenario, even if the IP conflict detection feature is enabled, Spiderpool will not perform IP conflict detection for KubeVirt.

## Fix IP Address for KubeVirt VMs

KubeVirt VMs may require fixed IP addresses in the following scenarios:

1. During VM live migration, it is expected that the VM retains its previous IP address after migration.
2. When the Pod associated with the VM resource undergoes a restart.
3. When the VMI (VirtualMachineInstance) resource corresponding to the VM resource is deleted.

It is important to note that the pattern of fixed IP addresses for KubeVirt VMs differs from that of StatefulSets:

1. For VMs, the Pod name changes between restarts, but the VMI name remains unchanged regardless of restarts. Therefore, the fixed IPs will be recorded based on VMs. Specifically, SpiderEndpoint resource will be associated with the VM resource's namespace and name to record its fixed IP address.
2. For StatefulSets, the Pod name remains the same between restarts, so Spiderpool records the fixed IP addresses based on Pods.

> This feature is enabled by default. When enabled, there are no restrictions. VMs can use a limited set of IP addresses from an IP pool to assign fixed IPs. However, regardless of whether a VM uses a fixed IP pool, its Pod can consistently obtain the same IP address. If disabled, Pods associated with VMs will be treated as stateless. During installation of Spiderpool using Helm, you can disable it by using `--set ipam.enableKubevirtStaticIP=false`.

## Prerequisites

1. A ready Kubernetes cluster.

2. [Helm](https://helm.sh/docs/intro/install/) has been installed.

## Steps

The following steps demonstrate how to use the passt network mode of KubeVirt with macvlan CNI to enable VMs to access the underlay network and assign fixed IPs using Spiderpool.

> Currently, macvlan and ipvlan are not suitable for KubeVirt's bridge network mode because in bridge network mode, the MAC address of the Pod interface is moved to the VM, causing the Pod to use a different address. However, macvlan and ipvlan CNI require the Pod's network interface to have the original MAC address.

### Install Spiderpool

Please refer to [Macvlan Quick Start](./install/underlay/get-started-macvlan.md) for installing Spiderpool. Make sure to set the Helm installation option `ipam.enableKubevirtStaticIP=true`.

### Create KubeVirt VM Applications

#### underlay single NIC situation

In the example YAML below, we create 1 KubeVirt VM application with KubeVirt passt network mode + macvlan:

- `v1.multus-cni.io/default-network`: select the default network CNI configuration for the application.

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

When creating a KubeVirt VM application, Spiderpool randomly selects an IP from the specified IP pool to establish a binding with the application.

```bash
~# kubectl get spiderippool
NAME          VERSION   SUBNET        ALLOCATED-IP-COUNT   TOTAL-IP-COUNT   DEFAULT
test-ippool   4         10.6.0.0/16   1                    10               false

~# kubectl get po -l vm.kubevirt.io/name=vm-cirros -o wide
NAME                            READY   STATUS      RESTARTS   AGE     IP              NODE                   NOMINATED NODE   READINESS GATES
virt-launcher-vm-cirros-rg6fs   2/2     Running     0          3m43s   10.6.168.105    node2                  <none>           1/1
```

Upon restarting a Kubevirt VM Pod, the new Pod retains its original IP address as expected.

```bash
~# kubectl delete pod virt-launcher-vm-cirros-rg6fs
pod "virt-launcher-vm-cirros-rg6fs" deleted

~# kubectl get po -l vm.kubevirt.io/name=vm-cirros -o wide
NAME                            READY   STATUS      RESTARTS   AGE     IP              NODE                   NOMINATED NODE   READINESS GATES
virt-launcher-vm-cirros-d68l2   2/2     Running     0          1m21s   10.6.168.105    node2                  <none>           1/1
```

When restarting a Kubevirt VMI, new Pods also maintain their assigned IP addresses as expected.

```bash
~# kubectl delete vmi vm-cirros
virtualmachineinstance.kubevirt.io "vm-cirros" deleted

~# kubectl get po -l vm.kubevirt.io/name=vm-cirros -o wide
NAME                            READY   STATUS    RESTARTS   AGE    IP              NODE                   NOMINATED NODE   READINESS GATES
virt-launcher-vm-cirros-jjgrl   2/2     Running   0          104s   10.6.168.105    node2                  <none>           1/1
```

The VM can communicate with other underlay Pods

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

The VM can access cluster IP addresses.

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

#### underlay multiple NICs situation

In the example YAML below, we create 1 KubeVirt VM application with KubeVirt bridge network mode + [ovs-cni](./install/underlay/get-started-ovs.md):

- `ipam.spidernet.io/ippools`: select IPPools for every NIC.(You can also use the multus resource CNI configuration level default IPPools)
- The multus resource `kube-system/ovs-vlan30` and `kube-system/ovs-vlan40` must enable the coordinator plugin to solve the multiple interfaces default route problem.
- ovs-cni doesn't support clusterIP network connectivity access.

```yaml
apiVersion: kubevirt.io/v1
kind: VirtualMachine
metadata:
  name: vm-centos
spec:
  runStrategy: Always
  template:
    metadata:
      annotations:
        ipam.spidernet.io/ippools: |-
          [{
             "ipv4": ["vlan30-v4-ippool"],
             "ipv6": ["vlan30-v6-ippool"]
           },{
             "ipv4": ["vlan40-v4-ippool"],
             "ipv6": ["vlan40-v6-ippool"]
          }]
    spec:
      architecture: amd64
      domain:
        cpu:
          cores: 1
          model: host-model
          sockets: 2
          threads: 1
        devices:
          disks:
          - disk:
              bus: virtio
            name: containerdisk
          - disk:
              bus: virtio
            name: cloudinitdisk
          interfaces:
          - bridge: {}
            name: ovs-bridge1
          - bridge: {}
            name: ovs-bridge2
        features:
          acpi:
            enabled: true
        machine:
          type: q35
        resources:
          requests:
            memory: 1Gi
      networks:
      - multus:
          default: true
          networkName: kube-system/ovs-vlan30
        name: ovs-bridge1
      - multus:
          networkName: kube-system/ovs-vlan40
        name: ovs-bridge2
      volumes:
      - name: containerdisk
        containerDisk:
          image: release-ci.daocloud.io/virtnest/system-images/centos-7.9-x86_64:v1
      - cloudInitNoCloud:
          networkData: |
            version: 2
            ethernets:
              eth0:
                dhcp4: true
              eth1:
                dhcp4: true
          userData: |
            #cloud-config
            ssh_pwauth: true
            disable_root: false
            chpasswd: {"list": "root:dangerous", expire: False}
            runcmd:
              - sed -i "/#\?PermitRootLogin/s/^.*$/PermitRootLogin yes/g" /etc/ssh/sshd_config
        name: cloudinitdisk
```

## Summary

Spiderpool guarantees that KubeVirt VM Pods consistently acquire the same IP addresses during restart and rebuild processes, meeting the fixed IP address requirements for Kubevirt VMs. It seamlessly integrates with macvlan or OVS CNI to enable VMs to access underlay networks.
