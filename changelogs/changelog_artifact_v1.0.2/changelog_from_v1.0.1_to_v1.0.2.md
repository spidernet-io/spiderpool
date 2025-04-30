
# v1.0.2
Welcome to the v1.0.2 release of Spiderpool!
Compared with version:v1.0.1, version:v1.0.2 has the following updates.

***

## New Feature

* SpiderMultusConfig: Add mtu size support for macvlan / ipvlan / sriov : [PR 4636](https://github.com/spidernet-io/spiderpool/pull/4636)

* Add pause and discards stats for RDMA metrics : [PR 4812](https://github.com/spidernet-io/spiderpool/pull/4812)



***

## Changed Feature

* Add a knob to initiate webhook deletion checks for ip resources : [PR 4675](https://github.com/spidernet-io/spiderpool/pull/4675)

* chart: optimize rdma titles : [PR 4710](https://github.com/spidernet-io/spiderpool/pull/4710)



***

## Fix

* Fix RDMA multicast metric in node dashboard : [PR 4706](https://github.com/spidernet-io/spiderpool/pull/4706)

* SpiderMultusConfig: Fix error json tag for min/maxTxRateMbps : [PR 4716](https://github.com/spidernet-io/spiderpool/pull/4716)

* fix: retrieve the endpoint for deleted stateless pod : [PR 4729](https://github.com/spidernet-io/spiderpool/pull/4729)

* IPAM fix: ENV EnableGCStatelessTerminatingPod(Not)ReadyNode=false does not work : [PR 4752](https://github.com/spidernet-io/spiderpool/pull/4752)

* Fix ENV EnableGCStatelessTerminatingPod(Not)ReadyNode doesn't works : [PR 4784](https://github.com/spidernet-io/spiderpool/pull/4784)



***

## Total 

Pull request number: 35

[ Commits ](https://github.com/spidernet-io/spiderpool/compare/v1.0.1...v1.0.2)
