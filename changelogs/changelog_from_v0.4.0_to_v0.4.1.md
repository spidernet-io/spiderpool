
# v0.4.1

***

## Feature

* make IP GC pod informer mechanism requeue if it failed to release IP : [PR 1591](https://github.com/spidernet-io/spiderpool/pull/1591)

* support third-party auto-created pool delete : [PR 1619](https://github.com/spidernet-io/spiderpool/pull/1619)

* title:	add kubernetes StatefulSet Start ordinal support : [PR 1668](https://github.com/spidernet-io/spiderpool/pull/1668)

* title:	add grafana dashaboard : [PR 1684](https://github.com/spidernet-io/spiderpool/pull/1684)



***

## Fix

* forbidden modifying auto-created IPPool by hand : [PR 1611](https://github.com/spidernet-io/spiderpool/pull/1611)

* fix IP GC health inaccurate check : [PR 1624](https://github.com/spidernet-io/spiderpool/pull/1624)

* fix: SpiderSubnet or SpiderIPPool can use `spec.routes` to configure duplicate routes or default route : [PR 1628](https://github.com/spidernet-io/spiderpool/pull/1628)

* add readiness initialDelaySeconds for spiderpool-controller : [PR 1660](https://github.com/spidernet-io/spiderpool/pull/1660)

* fix IP gc not in time : [PR 1661](https://github.com/spidernet-io/spiderpool/pull/1661)

* title:	make it takes not effect when app subnet changed in SpiderSubnet feature : [PR 1665](https://github.com/spidernet-io/spiderpool/pull/1665)

* title:	fix: Pod of spiderpool-agent restarts too slowly when node restart : [PR 1670](https://github.com/spidernet-io/spiderpool/pull/1670)

* title:	fix: The gateway addresses of SpiderSubnet and SpiderIPPool conflict with `spec.ips` : [PR 1673](https://github.com/spidernet-io/spiderpool/pull/1673)

* title:	fix auto-created pool reconcile algorithm : [PR 1694](https://github.com/spidernet-io/spiderpool/pull/1694)



***

## Totoal PR

[ 48 PR](https://github.com/spidernet-io/spiderpool/compare/v0.4.0...v0.4.1)
