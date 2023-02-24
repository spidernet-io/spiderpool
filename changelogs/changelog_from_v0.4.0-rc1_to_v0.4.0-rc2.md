
# v0.4.0-rc2

***

## Feature

* fix default subnet generate ippool for non-spiderpool pod : [PR 1254](https://github.com/spidernet-io/spiderpool/pull/1254)

* add third party controller application support with SpiderSubnet feature : [PR 1272](https://github.com/spidernet-io/spiderpool/pull/1272)

* update otel to release-1.13.0 : [PR 1320](https://github.com/spidernet-io/spiderpool/pull/1320)

* aggreate orphan pod auto-created IPPools deletion operation with SpiderSubnet feature : [PR 1334](https://github.com/spidernet-io/spiderpool/pull/1334)

* ippool informer workqueue modify to decrease IPPool operation conflicts : [PR 1332](https://github.com/spidernet-io/spiderpool/pull/1332)

* modify ipam fetch subnet IPPool retry operation times and duration : [PR 1346](https://github.com/spidernet-io/spiderpool/pull/1346)

* feat: Add filed `status.default` to the CRDs of SpiderSubnet and SpiderIPPool : [PR 1365](https://github.com/spidernet-io/spiderpool/pull/1365)



***

## Fix

* fix potential panic with SpiderSubnet feature : [PR 1242](https://github.com/spidernet-io/spiderpool/pull/1242)

* auto-created IPPool podAffinity adjustment : [PR 1260](https://github.com/spidernet-io/spiderpool/pull/1260)

* fix: An incorrect GC occurred during IP allocation : [PR 1342](https://github.com/spidernet-io/spiderpool/pull/1342)

* add metrics prefix and fix docs : [PR 1359](https://github.com/spidernet-io/spiderpool/pull/1359)

* fix: Event loss for updating 'spec.ips' of SpiderIPPool : [PR 1368](https://github.com/spidernet-io/spiderpool/pull/1368)



***

## Totoal PR

[ 67 PR](https://github.com/spidernet-io/spiderpool/compare/v0.4.0-rc1...v0.4.0-rc2)
