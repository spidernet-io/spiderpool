# IPAM Performance Testing

**English** | [**简体中文**](./ipam-performance-zh_CN.md)

*[Spiderpool](https://github.com/spidernet-io/spiderpool) is an underlay networking solution that provides rich IPAM and CNI integration capabilities. This article will compare it with the mainstream IPAM CNI plug-ins (e.g., [Whereabouts](https://github.com/k8snetworkplumbingwg/whereabouts), [Kube-OVN](https://github.com/kubeovn/kube-ovn)) and the widely-used overlay IPAM CNI plugins ([calico-ipam](https://github.com/projectcalico/calico), [cilium](https://github.com/cilium/cilium)) in `1000 Pod` scenarios.*

## Background

Why do we need to do performance testing on the underlay IPAM CNI plugin?

1. The speed at which IPAM allocates IP addresses largely determines the speed of application publishing.
2. Underlay IPAM often becomes a performance bottleneck when a large-scale Kubernetes cluster recovers from failures.
3. Under underlay networks, private IPv4 addresses are limited. Within a limited range of IP addresses, concurrent creation of Pods can involve IP address preemption and conflict, and it is challenging to quickly adjust the limited IP address resources.

## ENV

- Kubernetes: `v1.26.7`
- Container runtime: `containerd v1.7.2`
- OS: `Ubuntu 22.04 LTS`
- Kernel: `5.15.0-33-generic`

| Node     | Role                  | CPU | Memory |
| -------- | --------------------- | --- | ------ |
| master1  | control-plane, worker | 3C  | 8Gi    |
| master2  | control-plane, worker | 3C  | 8Gi    |
| master3  | control-plane, worker | 3C  | 8Gi    |
| worker4  | worker                | 3C  | 8Gi    |
| worker5  | worker                | 3C  | 8Gi    |
| worker6  | worker                | 3C  | 8Gi    |
| worker7  | worker                | 3C  | 8Gi    |
| worker8  | worker                | 3C  | 8Gi    |
| worker9  | worker                | 3C  | 8Gi    |
| worker10 | worker                | 3C  | 8Gi    |

## Objects

This test is based on the `0.3.1` version of [CNI Specification](https://www.cni.dev/docs/spec/), with [macvlan](https://www.cni.dev/plugins/current/main/macvlan/) and Spiderpool as the test object, and selected several other common network solutions in the open source community as a comparison:

| Test Object                   | Version   |
| ----------------------------- | --------- |
| Spiderpool based on macvlan   | `v0.8.0`  |
| Whereabouts based on macvlan  | `v0.6.2`  |
| Kube-OVN                      | `v1.12.2` |
| Cilium                        | `v1.14.3` |
| Calico                        | `v3.26.3` |

## Plan

The test ideas are mainly:

1. Underlay IP resources are limited, IP leakage and duplication of IP allocation can easily cause interference, so the accuracy of IP allocation is very important.
2. When a large number of Pods start up and compete for IP allocation, the IPAM allocation algorithm should be efficient in order to ensure that the Pods are released quickly and successfully.

Therefore, we designed a limit test with the same number of IP resources and Pod resources, and timed the time from Pod creation to Running to test the accuracy and robustness of IPAM in disguise. The test conditions are as follows:

- IPv4 single-stack and IPv4/IPv6 dual-stack scenarios.
- Create 100 Deployments, each with 10 replicas.

## Result

The following shows the results of the IPAM performance test, which includes two scenarios, `The number of IPs is equal to the number of Pods` and `IP sufficient`, to test each CNI. Calico and Cilium, for example, are based on the IP block pre-allocation mechanism to allocate IPs, and therefore can't perform the `The number of IPs is equal to the number of Pods` test in a relatively `fair` way, and only perform the `IP sufficient` scenario. We can only test `unlimited IPs` scenarios.

| Test Object                   | Limit IP to Pod Equivalents | IP Sufficient |
| ----------------------------- | --------------------------- | ------------- |
| Spiderpool based on macvlan   | 207s                        | 182s          |
| Whereabouts based on macvlan  | Failure                     | 2529s         |
| Kube-OVN                      | 405s                        | 343s          |
| Cilium                        | NA                          | 215s          |
| Calico                        | NA                          | 322s          |

## Analysis

![performance](../images/ipam-performance.png)

Spiderpool allocates IP addresses from the same CIDR range to all Pods in the whole cluster. Consequently, IP allocation and release face intense competition, presenting larger challenges in terms of IP allocation performance. By comparison, Whereabouts, Calico, and Cilium adopt an IPAM allocation principle where each node has a small IP address pool. This reduces the competition for IP allocation and mitigates the associated performance challenges. However, experimental data shows that despite Spiderpool's "lossy" IPAM principle, its IP allocation performance is actually quite good.

- During testing, the following phenomenon was encountered:

    Whereabouts based on macvlan: We tested the combination of macvlan and Whereabouts in a scenario where the available number of IP addresses matches the number of Pods in a 1:1 ratio. Within 300 seconds, 261 Pods reached the "Running" state at a relatively steady pace. By the 1080-second mark, 768 IP addresses were allocated. Afterward, the growth rate of Pods significantly slowed down, reaching 845 Pods by 2280 seconds. Subsequently, Whereabouts essentially stopped working, resulting in a positively near-infinite amount of time needed for further allocation. In our testing scenario, where the number of IP addresses matches the number of Pods in a 1:1 ratio, if the IPAM component fails to properly reclaim IP addresses, new Pods will fail to start due to a lack of available IP resources. We observed some of the following errors in the Pod that failed to start:

    ```bash
    [default/whereabout-9-5c658db57b-xtjx7:k8s-pod-network]: error adding container to network "k8s-pod-network": error at storage engine: time limit exceeded while waiting to become leader

    name "whereabout-9-5c658db57b-tdlms_default_e1525b95-f433-4dbe-81d9-6c85fd02fa70_1" is reserved for "38e7139658f37e40fa7479c461f84ec2777e29c9c685f6add6235fd0dba6e175"
    ```

## Summary

Although Spiderpool is primarily designed for underlay networks, it provides powerful IPAM capabilities. Its IP allocation and reclamation features face more intricate challenges, including IP address contention and conflicts, compared to the popular Overlay CNI IPAM plugins. However, Spiderpool's performance is ahead of the latter.
