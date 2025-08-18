
# v1.0.5
Welcome to the v1.0.5 release of Spiderpool!
Compared with version:v1.0.4, version:v1.0.5 has the following updates.

***

## New Feature

* SpiderMultusConfig: Add mtu size support for macvlan / ipvlan / sriov : [PR 4636](https://github.com/spidernet-io/spiderpool/pull/4636)

* Add pause and discards stats for RDMA metrics : [PR 4812](https://github.com/spidernet-io/spiderpool/pull/4812)

* ipam: add GC option for running pods with empty podIPs : [PR 4889](https://github.com/spidernet-io/spiderpool/pull/4889)

* Add node rdma device traffic class to metrics : [PR 4900](https://github.com/spidernet-io/spiderpool/pull/4900)

* feat: add rdma ippool zone filter for sriov cni : [PR 4999](https://github.com/spidernet-io/spiderpool/pull/4999)



***

## Changed Feature

* Add a knob to initiate webhook deletion checks for ip resources : [PR 4675](https://github.com/spidernet-io/spiderpool/pull/4675)

* chart: optimize rdma titles : [PR 4710](https://github.com/spidernet-io/spiderpool/pull/4710)



***

## Fix

* Ensure kernel sends GARPs to avoid communication failures : [PR 4649](https://github.com/spidernet-io/spiderpool/pull/4649)

* Update charts for ippool/subnet ValidatingWebhook : [PR 4663](https://github.com/spidernet-io/spiderpool/pull/4663)

* Fix RDMA multicast metric in node dashboard : [PR 4706](https://github.com/spidernet-io/spiderpool/pull/4706)

* SpiderMultusConfig: Fix error json tag for min/maxTxRateMbps : [PR 4716](https://github.com/spidernet-io/spiderpool/pull/4716)

* fix: retrieve the endpoint for deleted stateless pod : [PR 4729](https://github.com/spidernet-io/spiderpool/pull/4729)

* IPAM fix: ENV EnableGCStatelessTerminatingPod(Not)ReadyNode=false does not work : [PR 4752](https://github.com/spidernet-io/spiderpool/pull/4752)

* Fix ENV EnableGCStatelessTerminatingPod(Not)ReadyNode doesn't works : [PR 4784](https://github.com/spidernet-io/spiderpool/pull/4784)

* Add node name filter for pod owner cache to track local pods only : [PR 4881](https://github.com/spidernet-io/spiderpool/pull/4881)

* Fix metrics miss vport data from ethtool : [PR 4898](https://github.com/spidernet-io/spiderpool/pull/4898)

* fix: set policy route with marks in case hijacking calico packets : [PR 4905](https://github.com/spidernet-io/spiderpool/pull/4905)

* Fix label key pod_namespace using pod namespace instead of name : [PR 4936](https://github.com/spidernet-io/spiderpool/pull/4936)

* Remove outdated endpoints to prevent blocking the creation of new Pods : [PR 4948](https://github.com/spidernet-io/spiderpool/pull/4948)

* Update setNicAddr.sh : [PR 5001](https://github.com/spidernet-io/spiderpool/pull/5001)

* Optimize IPConflictDetection to fix syscall fails to receive ARP responses : [PR 5029](https://github.com/spidernet-io/spiderpool/pull/5029)



***

## Total 

Pull request number: 100

[ Commits ](https://github.com/spidernet-io/spiderpool/compare/v1.0.4...v1.0.5)
