
# v0.9.5

***

## New Feature

* spiderpool-agent: support to configure the sysctl config : [PR 3842](https://github.com/spidernet-io/spiderpool/pull/3842)

* agent can be set rp_filter to 0 for the each node : [PR 3907](https://github.com/spidernet-io/spiderpool/pull/3907)

* Add chainCNI support for spidermultusconfig : [PR 3973](https://github.com/spidernet-io/spiderpool/pull/3973)



***

## Fix

* Fix: Statefulset pod should change IP when recreating with a changed pool in annotation : [PR 3675](https://github.com/spidernet-io/spiderpool/pull/3675)

* fix: fail to access NodePort when pod owning multiple network card : [PR 3814](https://github.com/spidernet-io/spiderpool/pull/3814)

* pod launched by unexpected CNI when the health checking of the agent fails and multus.conf is lost  : [PR 3812](https://github.com/spidernet-io/spiderpool/pull/3812)

* init-pod: fix installtion block for agent pods can't running : [PR 3811](https://github.com/spidernet-io/spiderpool/pull/3811)

* [cherry-pick] fix: Spiderpool GC incorrect IP address during statefulset Pod scale up/down, causing IP conflict : [PR 3912](https://github.com/spidernet-io/spiderpool/pull/3912)

* coordinator should only set rp_filter for pod not the node : [PR 3968](https://github.com/spidernet-io/spiderpool/pull/3968)

* coordinator: Fix error policy routing table when pod has multi nics : [PR 3969](https://github.com/spidernet-io/spiderpool/pull/3969)



***

## Total 

Pull request number: 22

[ Commits ](https://github.com/spidernet-io/spiderpool/compare/v0.9.4...v0.9.5)
