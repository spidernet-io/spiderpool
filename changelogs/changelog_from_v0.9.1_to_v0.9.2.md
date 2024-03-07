
# v0.9.2

***

## New Feature

* IP reclaim:  differentiate stateless workload under deleting-timeout state on ready node and not-ready node : [PR 3002](https://github.com/spidernet-io/spiderpool/pull/3002)

* docs: bandwidth for ipvlan datapath : [PR 3137](https://github.com/spidernet-io/spiderpool/pull/3137)

* synchronize clusterIP CIDR from serviceCIDR to support k8s 1.29 : [PR 3132](https://github.com/spidernet-io/spiderpool/pull/3132)

* release conflicted ip of stateless workload to trigger assigning a new one : [PR 3081](https://github.com/spidernet-io/spiderpool/pull/3081)

* Rework spidercoordinator informer to update pod and service cidr : [PR 3260](https://github.com/spidernet-io/spiderpool/pull/3260)



***

## Changed Feature

* fix: the parent interface is down, set it to up before creating the vlan sub-interface : [PR 3088](https://github.com/spidernet-io/spiderpool/pull/3088)

* Spidercoordinator: It able to get CIDR from kubeadm-config : [PR 3062](https://github.com/spidernet-io/spiderpool/pull/3062)

* fix coordinator upgrade panic with CRD property empty : [PR 3118](https://github.com/spidernet-io/spiderpool/pull/3118)

* enable coordinate to support serviceCIDR according to a matched k8s version. : [PR 3168](https://github.com/spidernet-io/spiderpool/pull/3168)

* use helm charts value control coordinator components startup : [PR 3182](https://github.com/spidernet-io/spiderpool/pull/3182)

* fix the logic of obtaining kubeadm-config to avoid being unable to create Pods : [PR 3211](https://github.com/spidernet-io/spiderpool/pull/3211)

* Fix panic in spidercoordinator informer : [PR 3274](https://github.com/spidernet-io/spiderpool/pull/3274)

* spidercoordinator: Enhance the edge case : [PR 3287](https://github.com/spidernet-io/spiderpool/pull/3287)

* Spidercoordinator: sync kubeadm-config event to trigger the status update : [PR 3294](https://github.com/spidernet-io/spiderpool/pull/3294)



***

## Totoal 

Pull request number: 72

[ Commits ](https://github.com/spidernet-io/spiderpool/compare/v0.9.1...v0.9.2)
