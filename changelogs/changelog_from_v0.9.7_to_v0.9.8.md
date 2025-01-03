
# v0.9.8

***

## New Feature

* Add a flag to configure an link-local address to veth0 for istio : [PR 4251](https://github.com/spidernet-io/spiderpool/pull/4251)



***

## Fix

* fix: same-name conflict check specified by multus.spidernet.io/cr-name : [PR 4200](https://github.com/spidernet-io/spiderpool/pull/4200)

* Fix:  one NIC's IP pool shortage depleted IPs of other NICs in a multi-NIC setup. : [PR 4383](https://github.com/spidernet-io/spiderpool/pull/4383)

* Fix: statefulSet applications failed to create in multi-NIC scenarios. : [PR 4390](https://github.com/spidernet-io/spiderpool/pull/4390)

* Fix controller panic in cilium ipam is multi-pool : [PR 4461](https://github.com/spidernet-io/spiderpool/pull/4461)

* fix: fix get null podCIDR and serviceCIDR : [PR 4477](https://github.com/spidernet-io/spiderpool/pull/4477)

* Detecting IP conflicts takes place before gateway detection, avoiding communication failure : [PR 4506](https://github.com/spidernet-io/spiderpool/pull/4506)

* coodirnator: optimize the detectiong timeout for ip conflict and gateway detection : [PR 4511](https://github.com/spidernet-io/spiderpool/pull/4511)



***

## Total 

Pull request number: 26

[ Commits ](https://github.com/spidernet-io/spiderpool/compare/v0.9.7...v0.9.8)
