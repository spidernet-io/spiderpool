
# v1.0.4
Welcome to the v1.0.4 release of Spiderpool!
Compared with version:v1.0.3, version:v1.0.4 has the following updates.

***

## New Feature

* ipam: add GC option for running pods with empty podIPs : [PR 4897](https://github.com/spidernet-io/spiderpool/pull/4897)



***

## Changed Feature

* chart: optimize rdma titles : [PR 4712](https://github.com/spidernet-io/spiderpool/pull/4712)

* Add a knob to initiate webhook deletion checks for ip resources (#4675) : [PR 4974](https://github.com/spidernet-io/spiderpool/pull/4974)



***

## Fix

* IPAM sends GARPS to updating arp cache table : [PR 4689](https://github.com/spidernet-io/spiderpool/pull/4689)

* Fix RDMA multicast metric in node dashboard : [PR 4711](https://github.com/spidernet-io/spiderpool/pull/4711)

* fix: retrieve the endpoint for deleted stateless pod : [PR 4733](https://github.com/spidernet-io/spiderpool/pull/4733)

* SpiderMultusConfig: Fix error json tag for min/maxTxRateMbps (#4716) : [PR 4739](https://github.com/spidernet-io/spiderpool/pull/4739)

* IPAM fix: ENV EnableGCStatelessTerminatingPod(Not)ReadyNode=false does not work : [PR 4760](https://github.com/spidernet-io/spiderpool/pull/4760)

* Fix ENV EnableGCStatelessTerminatingPod(Not)ReadyNode doesn't works : [PR 4787](https://github.com/spidernet-io/spiderpool/pull/4787)

* Add node name filter for pod owner cache to track local pods only : [PR 4894](https://github.com/spidernet-io/spiderpool/pull/4894)

* Remove outdated endpoints to prevent blocking the creation of new Pods : [PR 4960](https://github.com/spidernet-io/spiderpool/pull/4960)



***

## Total 

Pull request number: 15

[ Commits ](https://github.com/spidernet-io/spiderpool/compare/v1.0.3...v1.0.4)
