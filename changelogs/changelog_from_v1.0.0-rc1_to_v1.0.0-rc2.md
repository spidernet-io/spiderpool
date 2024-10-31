
# v1.0.0-rc2
Welcome to the v1.0.0-rc2 release of Spiderpool!
Compared with version:v1.0.0-rc1, version:v1.0.0-rc2 has the following updates.

***

## New Feature

* spiderpool-agent: support to configure the sysctl config for node : [PR 3772](https://github.com/spidernet-io/spiderpool/pull/3772)

* doc: ai with macvlan : [PR 3870](https://github.com/spidernet-io/spiderpool/pull/3870)

* agent set rp_filter to 0 for the each node : [PR 3898](https://github.com/spidernet-io/spiderpool/pull/3898)

* Support ipv6 subnet with CIDR mask bigger than 64 : [PR 3804](https://github.com/spidernet-io/spiderpool/pull/3804)

* Add chainCNI support for spidermultusconfig : [PR 3918](https://github.com/spidernet-io/spiderpool/pull/3918)

* Add a pod mutating webhook to auto inject the pod network resources : [PR 4179](https://github.com/spidernet-io/spiderpool/pull/4179)

* Add a flag to configure an link-local address to veth0 for istio : [PR 4206](https://github.com/spidernet-io/spiderpool/pull/4206)

* Add RDAM metrics : [PR 4112](https://github.com/spidernet-io/spiderpool/pull/4112)



***

## Fix

* pod launched by unexpected CNI when the health checking of the agent fails and multus.conf is lost : [PR 3758](https://github.com/spidernet-io/spiderpool/pull/3758)

* rbac: remove permissions for patch/update nodes and webhook resources : [PR 3880](https://github.com/spidernet-io/spiderpool/pull/3880)

* fix: Spiderpool GC incorrect IP address during statefulset Pod scale up/down, causing IP conflict : [PR 3778](https://github.com/spidernet-io/spiderpool/pull/3778)

* coordinator should only set rp_filter for pod but not the node : [PR 3906](https://github.com/spidernet-io/spiderpool/pull/3906)

* coordinator: fix wrong policy route when there is more than 1 secondary nics : [PR 3873](https://github.com/spidernet-io/spiderpool/pull/3873)

* Update GOMAXPROCS configuration : [PR 4013](https://github.com/spidernet-io/spiderpool/pull/4013)

* fix: cannot uninstall spiderpool when sriovOperatorConfig is installed : [PR 3925](https://github.com/spidernet-io/spiderpool/pull/3925)

* Fix panic when create spidermultusconfig with nil podRPFilter in validate webhook : [PR 4062](https://github.com/spidernet-io/spiderpool/pull/4062)

* Fix wrong validate for unicast podMACPrefix when creating spiderMultusConfig : [PR 4098](https://github.com/spidernet-io/spiderpool/pull/4098)

* fix: avoid failures when cleaning up spiderpool resources due to resourceVersion conflicts. : [PR 4130](https://github.com/spidernet-io/spiderpool/pull/4130)

* fix: optimize the cleanup code judgment of NotFound resources : [PR 4156](https://github.com/spidernet-io/spiderpool/pull/4156)

* fix: same-name conflict check specified by multus.spidernet.io/cr-name : [PR 4168](https://github.com/spidernet-io/spiderpool/pull/4168)



***

## Total 

Pull request number: 84

[ Commits ](https://github.com/spidernet-io/spiderpool/compare/v1.0.0-rc1...v1.0.0-rc2)
