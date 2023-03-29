
# v0.4.0

***

## Feature

* add event records for SpiderSubnet  : [PR 967](https://github.com/spidernet-io/spiderpool/pull/967)

* bump github.com/go-swagger/go-swagger from 0.29.0 to 0.30.3 : [PR 1025](https://github.com/spidernet-io/spiderpool/pull/1025)

* separate controller to v4/v6 to accelerate auto-ippool event : [PR 1013](https://github.com/spidernet-io/spiderpool/pull/1013)

* add metircs for spider subnet : [PR 1009](https://github.com/spidernet-io/spiderpool/pull/1009)

* change the label of auto-created ippool for GC needs : [PR 1162](https://github.com/spidernet-io/spiderpool/pull/1162)

* subnet: accelerate application controllers processing : [PR 1151](https://github.com/spidernet-io/spiderpool/pull/1151)

* Add ClusterDefaultSubnet and SpiderSubnet multiple interfaces support : [PR 1205](https://github.com/spidernet-io/spiderpool/pull/1205)

* fix default subnet generate ippool for non-spiderpool pod : [PR 1254](https://github.com/spidernet-io/spiderpool/pull/1254)

* add third party controller application support with SpiderSubnet feature : [PR 1272](https://github.com/spidernet-io/spiderpool/pull/1272)

* update otel to release-1.13.0 : [PR 1320](https://github.com/spidernet-io/spiderpool/pull/1320)

* aggreate orphan pod auto-created IPPools deletion operation with SpiderSubnet feature : [PR 1334](https://github.com/spidernet-io/spiderpool/pull/1334)

* ippool informer workqueue modify to decrease IPPool operation conflicts : [PR 1332](https://github.com/spidernet-io/spiderpool/pull/1332)

* modify ipam fetch subnet IPPool retry operation times and duration : [PR 1346](https://github.com/spidernet-io/spiderpool/pull/1346)

* feat: Add filed `status.default` to the CRDs of SpiderSubnet and SpiderIPPool : [PR 1365](https://github.com/spidernet-io/spiderpool/pull/1365)

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

* feat: Remove field `spec.status` from SpiderSubnet CRD : [PR 1479](https://github.com/spidernet-io/spiderpool/pull/1479)

* decrease auto-created ippool name : [PR 1504](https://github.com/spidernet-io/spiderpool/pull/1504)

* metrics adjustment : [PR 1505](https://github.com/spidernet-io/spiderpool/pull/1505)

* modify default IP GC workers and make IP GC scan all dual stack concurrency : [PR 1525](https://github.com/spidernet-io/spiderpool/pull/1525)

* supplement spiderpool controller readiness probe : [PR 1531](https://github.com/spidernet-io/spiderpool/pull/1531)

* improve statrup : [PR 1542](https://github.com/spidernet-io/spiderpool/pull/1542)

* add limiter for IP GC pod informer architecture : [PR 1532](https://github.com/spidernet-io/spiderpool/pull/1532)

* support CNI V1.0.0 : [PR 1549](https://github.com/spidernet-io/spiderpool/pull/1549)



***

## Fix

* fix spiderpool-agent oom : [PR 966](https://github.com/spidernet-io/spiderpool/pull/966)

* fix ipam get subnet duplicated ippools : [PR 962](https://github.com/spidernet-io/spiderpool/pull/962)

* fix IPPool scale up but subnet no more IPs issue : [PR 982](https://github.com/spidernet-io/spiderpool/pull/982)

* remove workqueue discarding object incase that the event is lost : [PR 989](https://github.com/spidernet-io/spiderpool/pull/989)

* fix panic in AllocateIP() api when allocate IP for pod : [PR 999](https://github.com/spidernet-io/spiderpool/pull/999)

* fix:Invalid validation of `spec.ips` and `spec.excludeIPs` fields of SpiderSubnet : [PR 1001](https://github.com/spidernet-io/spiderpool/pull/1001)

* add ipam pool candidate validation for subnet : [PR 1021](https://github.com/spidernet-io/spiderpool/pull/1021)

* fix ipool workequeue panic : [PR 1068](https://github.com/spidernet-io/spiderpool/pull/1068)

* improve statefulset ipam cmd delete : [PR 1046](https://github.com/spidernet-io/spiderpool/pull/1046)

* fix:Docker automatic ARG `TARGETARCH` is missing : [PR 1113](https://github.com/spidernet-io/spiderpool/pull/1113)

* fix: The architecture of base image mismatch : [PR 1117](https://github.com/spidernet-io/spiderpool/pull/1117)

* add auto-created ippool gc : [PR 1135](https://github.com/spidernet-io/spiderpool/pull/1135)

* fix spider subnet leader lost and never work if it get a leader later : [PR 1155](https://github.com/spidernet-io/spiderpool/pull/1155)

* fix: The field `status.allocatedIPCount` of SpiderIPPool was incorrectly updated to -1 : [PR 1169](https://github.com/spidernet-io/spiderpool/pull/1169)

* bug fix: leak the spiderendpoint of StatefulSet : [PR 1182](https://github.com/spidernet-io/spiderpool/pull/1182)

* fix: The normal IP addresses are recycled by mistake when multiple Pods with the same namespace and name are created in a short time : [PR 1190](https://github.com/spidernet-io/spiderpool/pull/1190)

* fix: The "tags" plugin is not installed : [PR 1228](https://github.com/spidernet-io/spiderpool/pull/1228)

* fix potential panic with SpiderSubnet feature : [PR 1242](https://github.com/spidernet-io/spiderpool/pull/1242)

* auto-created IPPool podAffinity adjustment : [PR 1260](https://github.com/spidernet-io/spiderpool/pull/1260)

* fix: An incorrect GC occurred during IP allocation : [PR 1342](https://github.com/spidernet-io/spiderpool/pull/1342)

* add metrics prefix and fix docs : [PR 1359](https://github.com/spidernet-io/spiderpool/pull/1359)

* fix: Event loss for updating 'spec.ips' of SpiderIPPool : [PR 1368](https://github.com/spidernet-io/spiderpool/pull/1368)

* fix: Pod UID is an empty string when calling CNI DEL : [PR 1417](https://github.com/spidernet-io/spiderpool/pull/1417)

* fix: The IP allocation of Endpoint is incomplete in multi-NIC mode : [PR 1425](https://github.com/spidernet-io/spiderpool/pull/1425)

* fix application controller empty APIVersion string : [PR 1480](https://github.com/spidernet-io/spiderpool/pull/1480)

* add apiversion check for statefulset pod : [PR 1476](https://github.com/spidernet-io/spiderpool/pull/1476)

* add application informer delete hook workqueue operation : [PR 1491](https://github.com/spidernet-io/spiderpool/pull/1491)

* fix: The field deletionGracePeriodSeconds of Pod may be nil in K8s <=1.22 : [PR 1492](https://github.com/spidernet-io/spiderpool/pull/1492)

* fix IP GC scan all not work issue : [PR 1512](https://github.com/spidernet-io/spiderpool/pull/1512)

* fix: Limiter may deadlock when the queuer's ctx is about to deadline : [PR 1513](https://github.com/spidernet-io/spiderpool/pull/1513)

* fix: SubnetInformer executes SYNC too many times : [PR 1519](https://github.com/spidernet-io/spiderpool/pull/1519)

* improve IP GC pod informer startup : [PR 1517](https://github.com/spidernet-io/spiderpool/pull/1517)

* fix helm IP GC env varibale : [PR 1521](https://github.com/spidernet-io/spiderpool/pull/1521)

* fix auto-pool shrink failure issue : [PR 1518](https://github.com/spidernet-io/spiderpool/pull/1518)

* fix: Subnet status IPPool allocations disappears but the IPPool still exist : [PR 1526](https://github.com/spidernet-io/spiderpool/pull/1526)



***

## Totoal PR

[ 321 PR](https://github.com/spidernet-io/spiderpool/compare/v0.3.0...v0.4.0)
