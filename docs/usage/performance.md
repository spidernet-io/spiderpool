# Spiderpool Performance Testing

*[Spiderpool](https://github.com/spidernet-io/spiderpool) is a high-performance IPAM CNI plugin for underlay networks. This report will compare its performance with the mainstream underlay IPAM CNI plugins (such as  [Whereabouts](https://github.com/k8snetworkplumbingwg/whereabouts), [Kube-OVN](https://github.com/kubeovn/kube-ovn)) and the widely used overlay IPAM CNI plugin [calico-ipam](https://github.com/projectcalico/calico) under the "1000 Pod" scenario.*

## Background

Why do we need to do performance testing on the underlay IPAM CNI plugin?

1. The speed at which IPAM allocates IP addresses largely determines the speed of application publishing.
2. Underlay IPAM often becomes a performance bottleneck when a large-scale Kubernetes cluster recovers from failures.
3. Under underlay networks, private IPv4 addresses are limited. Within a limited range of IP addresses, concurrent creation of Pods can involve IP address preemption and conflict, and it is challenging to quickly adjust the limited IP address resources.

## ENV

- Kubernetes: `v1.25.4`
- container runtime: `containerd 1.6.12`
- OS: `CentOS Linux 8`
- kernel: `4.18.0-348.7.1.el8_5.x86_64`

| Node     | Role          | CPU  | Memory |
| -------- | ------------- | ---- | ------ |
| master1  | control-plane | 4C   | 8Gi    |
| master2  | control-plane | 4C   | 8Gi    |
| master3  | control-plane | 4C   | 8Gi    |
| worker4  |               | 3C   | 8Gi    |
| worker5  |               | 3C   | 8Gi    |
| worker6  |               | 3C   | 8Gi    |
| worker7  |               | 3C   | 8Gi    |
| worker8  |               | 3C   | 8Gi    |
| worker9  |               | 3C   | 8Gi    |
| worker10 |               | 3C   | 8Gi    |

## Objects

This test is based on the [CNI Specification](https://www.cni.dev/docs/spec/) `0.3.1`, using [macvlan](https://www.cni.dev/plugins/current/main/macvlan/)with Spiderpool, and selecting several other underlay network solutions in the open source community for comparison:

| Main CNI            | Main CNI Version | IPAM CNI                  | IPAM CNI Version | Features                                                     |
| ------------------- | ---------------- | ------------------------- | ---------------- | ------------------------------------------------------------ |
| macvlan             | `v1.1.1`         | Spiderpool                | `v0.4.0`         | There are multiple IP pools in a cluster, and the IP addresses in each pool can be used by Pods on any Node in the cluster. Competition occurs when multiple Pods in a cluster allocate IP addresses from the same pool. Support hosting the entire lifecycle of an IP pool, synchronizing it with workload creation, scaling, deletion, and reducing concurrency or storage issues caused by overly large shared pools. |
| macvlan             | `v1.1.1`         | Whereabouts (CRD backend) | `v0.6.1`         | Each Node can define its own available IP pool ranges. If there are duplicate IP addresses defined between Nodes, these IP addresses are promoted as a shared resource. |
| Kube-OVN (underlay) | `v1.11.3`        | Kube-OVN                  | `v1.11.3`        | IP addresses are organized by Subnet. Each Namespace can belong to a specific Subnet. Pods under the Namespace will automatically obtain IP addresses from the Subnet they belong to. Subnets are also a cluster resource, and the IP addresses of the same Subnet can be distributed on any Node. |
| Calico              | `v3.23.3`        | calico-ipam (CRD backend) | `v3.23.3`        | Each Node has one or more IP blocks exclusively, and the Pods on each Node only use the IP addresses in the local IP block. There is no competition or conflict between Nodes, and the allocation efficiency is very high. |

## Plan

During the testing, we will follow the following agreement:

- IPv4/IPv6 dual-stack.
- When testing the underlay IPAM CNI plugins, ensure that the number of available IP addresses is **1:1** to the number of Pods as much as possible. For example, if we plan to create 1000 Pods in the next step, we should limit the number of available IPv4/IPv6 addresses to 1000.

Specifically, we will attempt to create a total of 1000 Pods on the Kubernetes cluster in the following two ways, and record the time taken for all Pods to become `Running`:

- Create only one Deployment with 1000 replicas.
- Create 100 Deployments with 10 replicas per Deployment.

Next, we will use the following command to delete these 1000 Pods at once, and record the time it took for all Pods to become `Running` again:

```bash
kubectl get pod | grep "prefix" | awk '{print $1}' | xargs kubectl delete pod
```

Finally, we delete all Deployments and record the time taken for all Pods to completely disappear.

## Result

### 1 Deployment with 1000 replicas

| CNI                   | Creation | Re-creation | Deletion |
| --------------------- | -------- | ----------- | -------- |
| macvlan + Spiderpool  | 2m35s    | 9m50s       | 1m50s    |
| macvlan + Whereabouts | 25m18s   | failure     | 3m5s     |
| Kube-OVN              | 3m55s    | 7m20s       | 2m13s    |
| Calico + calico-ipam  | 1m56s    | 4m6s        | 1m36s    |

> During the testing of macvlan + Whereabouts, in the creation scenario, 922 Pods became `Running` at a relatively uniform rate within 14m25s. After that, the growth rate of Pods significantly decreased, and ultimately it took 25m18s for 1000 Pods to become `Running`. As for the re-creation scenario, after 55 Pods became `Running`, Whereabouts basically stopped working, and the time consumption was close to infinity.

### 100 Deployments with 10 replicas

| CNI                   | Creation | Re-creation | Deletion |
| --------------------- | -------- | ----------- | -------- |
| macvlan + Spiderpool  | 1m37s    | 3m27s       | 1m22s    |
| macvlan + Whereabouts | 21m49s   | failure     | 2m9s     |
| Kube-OVN              | 4m6s     | 7m46s       | 2m8s     |
| Calico + calico-ipam  | 1m57s    | 3m58s       | 1m35s    |

## Summary

Although Spiderpool is an IPAM CNI plugin suitable for underlay networks, it faces more complex IP address preemption and conflict issues than mainstream overlay IPAM CNI plugins, but its performance in most scenarios is not inferior to the latter.
