
# v0.7.0

***

## Feature

* spidercoordinator: support setting podCIDRType to none  : [PR 2141](https://github.com/spidernet-io/spiderpool/pull/2141)

* add SpiderMultusConfig annotation validation in webhook : [PR 2170](https://github.com/spidernet-io/spiderpool/pull/2170)

* coordinator: auto mode is supported : [PR 2178](https://github.com/spidernet-io/spiderpool/pull/2178)

* single IP support in dual stack with SpiderSubnet feature : [PR 2205](https://github.com/spidernet-io/spiderpool/pull/2205)

* feature : SpiderSubnet could take over orphan IPPool : [PR 2185](https://github.com/spidernet-io/spiderpool/pull/2185)

* optimize IPAM pool selections : [PR 2207](https://github.com/spidernet-io/spiderpool/pull/2207)

* spidermultusconfig: ovs cni config supported : [PR 2199](https://github.com/spidernet-io/spiderpool/pull/2199)



***

## Fix

* fix SpiderSubnet CRD vlanID : [PR 2117](https://github.com/spidernet-io/spiderpool/pull/2117)

* fix:Missing resource type in cleanCRD scripts : [PR 2131](https://github.com/spidernet-io/spiderpool/pull/2131)

* coordinator: fix failed to setupHijackRoutes  : [PR 2148](https://github.com/spidernet-io/spiderpool/pull/2148)

* clean up spidercoordinator.status if phase is NotReady : [PR 2152](https://github.com/spidernet-io/spiderpool/pull/2152)

* spidermultusconfig: remove redundant field "resourceName" : [PR 2140](https://github.com/spidernet-io/spiderpool/pull/2140)

* spidermultusconfig: validate customcniconfig if is valid : [PR 2153](https://github.com/spidernet-io/spiderpool/pull/2153)

* fix potential panic with CRD string method : [PR 2172](https://github.com/spidernet-io/spiderpool/pull/2172)

* fix wrong SpiderEndpoint vlan ID number  : [PR 2160](https://github.com/spidernet-io/spiderpool/pull/2160)

* orphan pod skip SpiderSubnet feature : [PR 2214](https://github.com/spidernet-io/spiderpool/pull/2214)

* fix SpiderMultusConfig potential panic : [PR 2225](https://github.com/spidernet-io/spiderpool/pull/2225)

* coordinator: make all host rule tables exists in table 500 : [PR 2206](https://github.com/spidernet-io/spiderpool/pull/2206)



***

## Totoal PR

[ 74 PR](https://github.com/spidernet-io/spiderpool/compare/v0.6.0...v0.7.0)
