
# v0.8.8

***

## New Feature

* spiderpool-agent: support to configure the sysctl config : [PR 3840](https://github.com/spidernet-io/spiderpool/pull/3840)

* agent can be set rp_filter to 0 for the each node : [PR 3913](https://github.com/spidernet-io/spiderpool/pull/3913)



***

## Fix

* Fix: Statefulset pod should change IP when recreating with a changed pool in annotation : [PR 3674](https://github.com/spidernet-io/spiderpool/pull/3674)

* fix: fail to access NodePort when pod owning multiple network cards : [PR 3815](https://github.com/spidernet-io/spiderpool/pull/3815)

* pod launched by unexpected CNI when the health checking of the agent fails and multus.conf is lost  : [PR 3813](https://github.com/spidernet-io/spiderpool/pull/3813)

* init-pod: fix installtion block for agent pods can't running : [PR 3810](https://github.com/spidernet-io/spiderpool/pull/3810)

* [cherry-pick] fix: Spiderpool GC incorrect IP address during statefulset Pod scale up/down, causing IP conflict : [PR 3914](https://github.com/spidernet-io/spiderpool/pull/3914)

* coordinator: Fix error policy routing table when pod has multi nics : [PR 3986](https://github.com/spidernet-io/spiderpool/pull/3986)

* coordinator should only set rp_filter for pod not the node : [PR 3970](https://github.com/spidernet-io/spiderpool/pull/3970)

* Fix：the chart value tuneSysctlConfig does not work : [PR 3991](https://github.com/spidernet-io/spiderpool/pull/3991)



***

## Total 

Pull request number: 18

[ Commits ](https://github.com/spidernet-io/spiderpool/compare/v0.8.7...v0.8.8)
