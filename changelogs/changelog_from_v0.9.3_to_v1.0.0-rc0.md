
# v1.0.0-rc0
Welcome to the v1.0.0-rc0 release of Spiderpool!
Compared with version:v0.9.3, version:v1.0.0-rc0 has the following updates.

***

## New Feature

* subnet feature: support to turn on or off the feature of managing automatic-ippool : [PR 3241](https://github.com/spidernet-io/spiderpool/pull/3241)

* Rework spidercoordinator informer to update pod and service cidr : [PR 3249](https://github.com/spidernet-io/spiderpool/pull/3249)

* chart: Support configure ifNames for rdmaSharedDevicePlugin : [PR 3335](https://github.com/spidernet-io/spiderpool/pull/3335)

* feature: support wildcard match for IPPool : [PR 3262](https://github.com/spidernet-io/spiderpool/pull/3262)

* feature: run a clean-up job when uninstalling : [PR 3339](https://github.com/spidernet-io/spiderpool/pull/3339)

* DRA: Integrates with DRA and CDI : [PR 3329](https://github.com/spidernet-io/spiderpool/pull/3329)



***

## Changed Feature

* Support getting serviceCIDR from spec.Containers[0].Args of kube-controller-manager Pod : [PR 3243](https://github.com/spidernet-io/spiderpool/pull/3243)

* Fix panic in spidercoordinator informer : [PR 3269](https://github.com/spidernet-io/spiderpool/pull/3269)

* spidercoordinator: Enhance the edge case : [PR 3284](https://github.com/spidernet-io/spiderpool/pull/3284)

* spidermultusconfig: add missing filed for generateCoordinatorCNIConf : [PR 3283](https://github.com/spidernet-io/spiderpool/pull/3283)

* Spidercoordinator: sync kubeadm-config event to trigger the status update : [PR 3291](https://github.com/spidernet-io/spiderpool/pull/3291)

* coordinator: rework GetDefaultRouteInterface to get pod's default route nic : [PR 3302](https://github.com/spidernet-io/spiderpool/pull/3302)

* coordinator: ensure hijickRoute's gw is from hostIPRouteForPod : [PR 3358](https://github.com/spidernet-io/spiderpool/pull/3358)



***

## Totoal 

Pull request number: 47

[ Commits ](https://github.com/spidernet-io/spiderpool/compare/v0.9.3...v1.0.0-rc0)
