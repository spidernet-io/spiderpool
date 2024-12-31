
# v1.0.0-rc4
Welcome to the v1.0.0-rc4 release of Spiderpool!
Compared with version:v1.0.0-rc3, version:v1.0.0-rc4 has the following updates.

***

## New Feature

* chart: update plugins image : [PR 4406](https://github.com/spidernet-io/spiderpool/pull/4406)

* Add cluster dropdown menu for grafana dashboard : [PR 4409](https://github.com/spidernet-io/spiderpool/pull/4409)

* fix: multus fails to reach api server when the old service account is out of data && update multus to v4 : [PR 4393](https://github.com/spidernet-io/spiderpool/pull/4393)



***

## Fix

* Reduce excessive WARN logs for forbidden resource access : [PR 4356](https://github.com/spidernet-io/spiderpool/pull/4356)

* Fix:  one NIC's IP pool shortage depleted IPs of other NICs in a multi-NIC setup. : [PR 4379](https://github.com/spidernet-io/spiderpool/pull/4379)

* Fix: statefulSet applications failed to create in multi-NIC scenarios. : [PR 4359](https://github.com/spidernet-io/spiderpool/pull/4359)

* Fix: the pod fails to run because the certificate of the pod webhook  is not up to data after helm upgrading : [PR 4420](https://github.com/spidernet-io/spiderpool/pull/4420)

* fix: fail to get podCIDR and serviceCIDR : [PR 4366](https://github.com/spidernet-io/spiderpool/pull/4366)

* Fix controller panic in cilium ipam is multi-pool : [PR 4433](https://github.com/spidernet-io/spiderpool/pull/4433)

* Detect IP conflicts before gateway detection to fix communication fail : [PR 4474](https://github.com/spidernet-io/spiderpool/pull/4474)



***

## Total 

Pull request number: 48

[ Commits ](https://github.com/spidernet-io/spiderpool/compare/v1.0.0-rc3...v1.0.0-rc4)
