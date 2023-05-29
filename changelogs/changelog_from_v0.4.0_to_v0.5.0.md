
# v0.5.0

***

## Feature

* make IP GC pod informer mechanism requeue if it failed to release IP : [PR 1591](https://github.com/spidernet-io/spiderpool/pull/1591)

* support third-party auto-created pool delete : [PR 1619](https://github.com/spidernet-io/spiderpool/pull/1619)

* add kubernetes StatefulSet Start ordinal support : [PR 1667](https://github.com/spidernet-io/spiderpool/pull/1667)

* add debug level metrics : [PR 1678](https://github.com/spidernet-io/spiderpool/pull/1678)

* add grafana dashaboard : [PR 1676](https://github.com/spidernet-io/spiderpool/pull/1676)

* SpiderSubnet auto-created IPPool reuse : [PR 1725](https://github.com/spidernet-io/spiderpool/pull/1725)

* title:	add cluster subnet flexible ip number support : [PR 1764](https://github.com/spidernet-io/spiderpool/pull/1764)



***

## Fix

* forbidden modifying auto-created IPPool by hand : [PR 1611](https://github.com/spidernet-io/spiderpool/pull/1611)

* fix IP GC health inaccurate check : [PR 1624](https://github.com/spidernet-io/spiderpool/pull/1624)

* fix: SpiderSubnet or SpiderIPPool can use `spec.routes` to configure duplicate routes or default route : [PR 1628](https://github.com/spidernet-io/spiderpool/pull/1628)

* add readiness initialDelaySeconds for spiderpool-controller : [PR 1660](https://github.com/spidernet-io/spiderpool/pull/1660)

* fix IP gc not in time : [PR 1661](https://github.com/spidernet-io/spiderpool/pull/1661)

* make it takes not effect when app subnet changed in SpiderSubnet feature : [PR 1664](https://github.com/spidernet-io/spiderpool/pull/1664)

* fix IP GC doesn't trace statefulset terminating pod issue : [PR 1666](https://github.com/spidernet-io/spiderpool/pull/1666)

* fix: Pod of spiderpool-agent restarts too slowly when node restart : [PR 1669](https://github.com/spidernet-io/spiderpool/pull/1669)

* fix: The gateway addresses of SpiderSubnet and SpiderIPPool conflict with `spec.ips` : [PR 1671](https://github.com/spidernet-io/spiderpool/pull/1671)

* fix auto-created pool reconcile algorithm : [PR 1693](https://github.com/spidernet-io/spiderpool/pull/1693)

* title:	fix third-party controller auto-pool reclaim ippool symbol : [PR 1748](https://github.com/spidernet-io/spiderpool/pull/1748)

* title:	improve iprange function to reduce memory allocation : [PR 1778](https://github.com/spidernet-io/spiderpool/pull/1778)

* title:	add ippool podAffinity validation : [PR 1807](https://github.com/spidernet-io/spiderpool/pull/1807)



***

## Totoal PR

[ 82 PR](https://github.com/spidernet-io/spiderpool/compare/v0.4.0...v0.5.0)
