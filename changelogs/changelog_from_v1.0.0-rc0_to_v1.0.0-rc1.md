
# v1.0.0-rc1
Welcome to the v1.0.0-rc1 release of Spiderpool!
Compared with version:v1.0.0-rc0, version:v1.0.0-rc1 has the following updates.

***

## New Feature

* fix(charts): Remove unnecessary sensitive permissions for DaemonSet agent and Pod init : [PR 3522](https://github.com/spidernet-io/spiderpool/pull/3522)

* update crds for spiderpool dra feature : [PR 3527](https://github.com/spidernet-io/spiderpool/pull/3527)

* spiderclaimparameter: add webhook to verify the create and update : [PR 3668](https://github.com/spidernet-io/spiderpool/pull/3668)

* update version of CNI plugins : [PR 3733](https://github.com/spidernet-io/spiderpool/pull/3733)

* update sriov-operator version to v1.3.0 : [PR 3716](https://github.com/spidernet-io/spiderpool/pull/3716)



***

## Changed Feature

* coordinator: Use ARP to detect the gateway rather than ICMP : [PR 3584](https://github.com/spidernet-io/spiderpool/pull/3584)



***

## Fix

* DRA: fix error start of agent : [PR 3504](https://github.com/spidernet-io/spiderpool/pull/3504)

* RBAC: avoiding too high permissions leading to potential CVE risks : [PR 3608](https://github.com/spidernet-io/spiderpool/pull/3608)

* Fix: it is so slow to create a Pod requiring IP from a super big CIDR : [PR 3583](https://github.com/spidernet-io/spiderpool/pull/3583)

* add link-local IP to veth0 for istio : [PR 3588](https://github.com/spidernet-io/spiderpool/pull/3588)

* Fix: Statefulset pod should change IP when recreating with a changed pool in annotation : [PR 3669](https://github.com/spidernet-io/spiderpool/pull/3669)

* Optimize clean job to use host network : [PR 3692](https://github.com/spidernet-io/spiderpool/pull/3692)

* Optimize clean job to use host network : [PR 3697](https://github.com/spidernet-io/spiderpool/pull/3697)

* fix: fail to access NodePort when pod owning multiple network cards : [PR 3686](https://github.com/spidernet-io/spiderpool/pull/3686)

* Optimize clean scripts : [PR 3706](https://github.com/spidernet-io/spiderpool/pull/3706)

* fix: Missing GLIBC dynamic dependency makes ovs binary unavailable : [PR 3752](https://github.com/spidernet-io/spiderpool/pull/3752)

* remove CRD  installed by sriov-network-operator when uninstalling : [PR 3726](https://github.com/spidernet-io/spiderpool/pull/3726)

* init-pod:  support helm wait and fix installation block when agent pods fails to run : [PR 3732](https://github.com/spidernet-io/spiderpool/pull/3732)



***

## Totoal 

Pull request number: 85

[ Commits ](https://github.com/spidernet-io/spiderpool/compare/v1.0.0-rc0...v1.0.0-rc1)
