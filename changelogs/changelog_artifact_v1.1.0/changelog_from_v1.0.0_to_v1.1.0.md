
# v1.1.0
Welcome to the v1.1.0 release of Spiderpool!
Compared with version:v1.0.0, version:v1.1.0 has the following updates.

***

## New Feature

* ipam: add GC option for running pods with empty podIPs : [PR 4897](https://github.com/spidernet-io/spiderpool/pull/4897)

* Add custom runtime netns path : [PR 5175](https://github.com/spidernet-io/spiderpool/pull/5175)

* feat: add rdma ippool zone filter for sriov cni  : [PR 5182](https://github.com/spidernet-io/spiderpool/pull/5182)

* bump version to v1.1.0-rc1 : [PR 5191](https://github.com/spidernet-io/spiderpool/pull/5191)



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

* Optimize IPConflictDetection to fix syscall fails to receive ARP resp… : [PR 5066](https://github.com/spidernet-io/spiderpool/pull/5066)

* Fix error annotations injected by rdma webhook : [PR 5145](https://github.com/spidernet-io/spiderpool/pull/5145)

* Using jq to parse the default CNI for improved script robustness (#5183) : [PR 5186](https://github.com/spidernet-io/spiderpool/pull/5186)

* Fix dual port nic parent label not right : [PR 5327](https://github.com/spidernet-io/spiderpool/pull/5327)



***

## Total 

Pull request number: 25

[ Commits ](https://github.com/spidernet-io/spiderpool/compare/v1.0.0...v1.1.0)
