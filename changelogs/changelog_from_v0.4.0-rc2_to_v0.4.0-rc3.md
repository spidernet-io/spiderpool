
# v0.4.0-rc3

***

## Feature

* feat: Add the 'status.default' column to the 'kubectl get' output of SpiderSubnet and SpiderIPPool : [PR 1377](https://github.com/spidernet-io/spiderpool/pull/1377)

* Refactor IPAM main process : [PR 1398](https://github.com/spidernet-io/spiderpool/pull/1398)

* Change `status.default` to `spec.default` in the CRDs of SpiderIPPool : [PR 1403](https://github.com/spidernet-io/spiderpool/pull/1403)

* feat: Use `spec.default` to make the cluster default IPPools effective instead of configuring Comfigmap spiderpool-conf : [PR 1418](https://github.com/spidernet-io/spiderpool/pull/1418)

* Add validation for `spec.default` in the webhook of SpiderSubnet and SpiderIPPool : [PR 1421](https://github.com/spidernet-io/spiderpool/pull/1421)

* IP garbage collection : [PR 1422](https://github.com/spidernet-io/spiderpool/pull/1422)

* IP garbage collection : [PR 1424](https://github.com/spidernet-io/spiderpool/pull/1424)

* refactor SpiderSubnet feature : [PR 1434](https://github.com/spidernet-io/spiderpool/pull/1434)

* feat: Add queuer timeout mechanism based on context for pkg limiter : [PR 1437](https://github.com/spidernet-io/spiderpool/pull/1437)

* add third-party application support for SpiderSubnet feature : [PR 1438](https://github.com/spidernet-io/spiderpool/pull/1438)

* Bump Spiderpool CRD form v1 to v2beta1 : [PR 1443](https://github.com/spidernet-io/spiderpool/pull/1443)



***

## Fix

* fix: Pod UID is an empty string when calling CNI DEL : [PR 1417](https://github.com/spidernet-io/spiderpool/pull/1417)

* fix: The IP allocation of Endpoint is incomplete in multi-NIC mode : [PR 1425](https://github.com/spidernet-io/spiderpool/pull/1425)



***

## Totoal PR

[ 43 PR](https://github.com/spidernet-io/spiderpool/compare/v0.4.0-rc2...v0.4.0-rc3)
