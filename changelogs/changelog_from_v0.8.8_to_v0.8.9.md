
# v0.8.9

***

## New Feature

* Add a flag to configure an link-local address to veth0 for istio : [PR 4261](https://github.com/spidernet-io/spiderpool/pull/4261)



***

## Fix

* Update GOMAXPROCS configuration : [PR 4040](https://github.com/spidernet-io/spiderpool/pull/4040)

* Fix panic  in validate webhook for creating spidermultusconfig : [PR 4099](https://github.com/spidernet-io/spiderpool/pull/4099)

* fix: same-name conflict check specified by multus.spidernet.io/cr-name : [PR 4202](https://github.com/spidernet-io/spiderpool/pull/4202)

* Fix:  one NIC's IP pool shortage depleted IPs of other NICs in a multi-NIC setup. : [PR 4384](https://github.com/spidernet-io/spiderpool/pull/4384)

* Fix: fail to create  statefulSet applications in multi-NIC scenarios. : [PR 4391](https://github.com/spidernet-io/spiderpool/pull/4391)

* Fix controller panic in cilium ipam is multi-pool : [PR 4459](https://github.com/spidernet-io/spiderpool/pull/4459)

* bump multus to v4 & fix multus fails to reach api server when the old service account is out of data : [PR 4460](https://github.com/spidernet-io/spiderpool/pull/4460)

* fix: fix get null podCIDR and serviceCIDR : [PR 4476](https://github.com/spidernet-io/spiderpool/pull/4476)

* Detecting IP conflicts takes place before gateway detection, avoiding communication failure : [PR 4505](https://github.com/spidernet-io/spiderpool/pull/4505)

* coodirnator: optimize the detectiong timeout for ip conflict and gateway detection : [PR 4512](https://github.com/spidernet-io/spiderpool/pull/4512)



***

## Total 

Pull request number: 24

[ Commits ](https://github.com/spidernet-io/spiderpool/compare/v0.8.8...v0.8.9)
