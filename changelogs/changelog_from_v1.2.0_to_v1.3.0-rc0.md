
# v1.3.0-rc0
Welcome to the v1.3.0-rc0 release of Spiderpool!
Compared with version:v1.2.0, version:v1.3.0-rc0 has the following updates.

***

## New Feature

* Add IaaS network provider integration support : [PR 5573](https://github.com/spidernet-io/spiderpool/pull/5573)

* feat: add configurable HTTP timeout for IaaS provider integration : [PR 5661](https://github.com/spidernet-io/spiderpool/pull/5661)

* feat: add configurable vethMTU for coordinator veth0 device : [PR 5681](https://github.com/spidernet-io/spiderpool/pull/5681)

* Add coordinator static routes configuration support : [PR 5683](https://github.com/spidernet-io/spiderpool/pull/5683)

* Add ENI slot device plugin for auxiliary ENI scheduling in provider mode : [PR 5678](https://github.com/spidernet-io/spiderpool/pull/5678)

* feat: add configurable logging options to IPAM and coordinator plugins : [PR 5730](https://github.com/spidernet-io/spiderpool/pull/5730)

* feat: support IaaS provider prewarm IP pools : [PR 5778](https://github.com/spidernet-io/spiderpool/pull/5778)



***

## Changed Feature

* coordinator: detect SLAAC v6 on iface when PrevResult.IPs lacks v6 : [PR 5619](https://github.com/spidernet-io/spiderpool/pull/5619)

* feat: add vlanMode support to vlan CNI configuration with auto/manual… : [PR 5693](https://github.com/spidernet-io/spiderpool/pull/5693)



***

## Fix

* docs: add X-Request-Timeout-Ms header to IaaS provider HTTP API  : [PR 5708](https://github.com/spidernet-io/spiderpool/pull/5708)

* fix: decouple coordinator DEL from Pod lookup : [PR 5727](https://github.com/spidernet-io/spiderpool/pull/5727)

* fix: include disabled calico ippools in spidercoordinator status : [PR 5739](https://github.com/spidernet-io/spiderpool/pull/5739)

* fix(ipam): release partial NIC allocations to prevent dual-stack cross-pod deadlock : [PR 5814](https://github.com/spidernet-io/spiderpool/pull/5814)



***

## Total 

Pull request number: 60

[ Commits ](https://github.com/spidernet-io/spiderpool/compare/v1.2.0...v1.3.0-rc0)
