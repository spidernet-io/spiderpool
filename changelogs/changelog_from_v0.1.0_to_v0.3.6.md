
# v0.3.6

***

## Feature

* feat:Add webhook of SpiderSubnet : [PR 728](https://github.com/spidernet-io/spiderpool/pull/728)

* installation: init subnet : [PR 750](https://github.com/spidernet-io/spiderpool/pull/750)

* Add the cert auto gen mode : [PR 767](https://github.com/spidernet-io/spiderpool/pull/767)

* feat:Add the informer of SpiderSubnet : [PR 771](https://github.com/spidernet-io/spiderpool/pull/771)

* add k8s informers for subnet manager : [PR 749](https://github.com/spidernet-io/spiderpool/pull/749)

* Optimize certificate checking logic : [PR 777](https://github.com/spidernet-io/spiderpool/pull/777)

* feat:Add K8s event recorder : [PR 833](https://github.com/spidernet-io/spiderpool/pull/833)

* Cascade delete SpiderSubnet and SpiderIPPools controlled by it : [PR 824](https://github.com/spidernet-io/spiderpool/pull/824)

* Return more detailed error message when the Spiderpool CNI bin fails to call : [PR 836](https://github.com/spidernet-io/spiderpool/pull/836)

* feat:Add retry mechanism for updating 'status.freeIPs' of SpiderSubnet : [PR 892](https://github.com/spidernet-io/spiderpool/pull/892)

* add debug information and set go max procs : [PR 910](https://github.com/spidernet-io/spiderpool/pull/910)

* feat:Add the field `status.controlledIPPools` in SpiderSubnet to record the IP pre-allocation details of the IPPools controlled by it : [PR 911](https://github.com/spidernet-io/spiderpool/pull/911)

* feat:Add mutating webhook for SpiderEndpoint : [PR 933](https://github.com/spidernet-io/spiderpool/pull/933)

* feat:Enable the RESYNC mechanism of SpiderSubnet Informer : [PR 931](https://github.com/spidernet-io/spiderpool/pull/931)

* support to scale up and down the IP in auto-created spiderippool  : [PR 834](https://github.com/spidernet-io/spiderpool/pull/834)

* add event records for SpiderSubnet  : [PR 967](https://github.com/spidernet-io/spiderpool/pull/967)

* bump github.com/go-swagger/go-swagger from 0.29.0 to 0.30.3 : [PR 1025](https://github.com/spidernet-io/spiderpool/pull/1025)

* separate controller to v4/v6 to accelerate auto-ippool event : [PR 1013](https://github.com/spidernet-io/spiderpool/pull/1013)

* add metircs for spider subnet : [PR 1009](https://github.com/spidernet-io/spiderpool/pull/1009)



***

## Fix

* remove validating webhook post-install : [PR 654](https://github.com/spidernet-io/spiderpool/pull/654)

* fix:Failed to correctly judge whether the subnets of the two IP pools overlap each other : [PR 675](https://github.com/spidernet-io/spiderpool/pull/675)

* fix:The gateway in CIDR format passed the validating webhook by mistake : [PR 744](https://github.com/spidernet-io/spiderpool/pull/744)

* wrong service in certificates : [PR 774](https://github.com/spidernet-io/spiderpool/pull/774)

* fix:FreeIPs of newly created SpiderSubnet cannot be synchronized automatically : [PR 778](https://github.com/spidernet-io/spiderpool/pull/778)

* fix spider-init bug to read ipRange : [PR 806](https://github.com/spidernet-io/spiderpool/pull/806)

* spider-init parse issue : [PR 810](https://github.com/spidernet-io/spiderpool/pull/810)

* fix subnet no pointer bug : [PR 832](https://github.com/spidernet-io/spiderpool/pull/832)

* chart typo : [PR 843](https://github.com/spidernet-io/spiderpool/pull/843)

* fix:Missing resource type when cleaning SpiderEndpoint CRs in cleanCRD.sh : [PR 861](https://github.com/spidernet-io/spiderpool/pull/861)

* update vendor to fix CVE-2022-1996 : [PR 879](https://github.com/spidernet-io/spiderpool/pull/879)

* fix:Illegal creation of two SpiderSubnets with the same 'spec.subnet' : [PR 872](https://github.com/spidernet-io/spiderpool/pull/872)

* fix:Panic when failed to mark an IP allocation : [PR 929](https://github.com/spidernet-io/spiderpool/pull/929)

* fix:Failed to remove free IP addresees in SpiderSubnet : [PR 936](https://github.com/spidernet-io/spiderpool/pull/936)

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

* auto cherry pick PR 1135 to branch release-v0.3 : [PR 1152](https://github.com/spidernet-io/spiderpool/pull/1152)

* auto cherry pick PR 1155 to branch release-v0.3 : [PR 1159](https://github.com/spidernet-io/spiderpool/pull/1159)

* cherry pick pr 1662 to release v0.3 : [PR 1179](https://github.com/spidernet-io/spiderpool/pull/1179)

* auto cherry pick PR 1169 to branch release-v0.3 : [PR 1177](https://github.com/spidernet-io/spiderpool/pull/1177)

* bug fix: leak the spiderendpoint of StatefulSet : [PR 1186](https://github.com/spidernet-io/spiderpool/pull/1186)

* fix: The normal IP addresses are recycled by mistake when multiple Pods with the same namespace and name are created in a short time : [PR 1191](https://github.com/spidernet-io/spiderpool/pull/1191)



***

## Totoal PR

[ 275 PR](https://github.com/spidernet-io/spiderpool/compare/v0.1.0...v0.3.6)
