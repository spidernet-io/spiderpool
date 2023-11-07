
# v0.8.0

***

## New Feature

* set chart enableSpiderSubnet to be true : [PR 2302](https://github.com/spidernet-io/spiderpool/pull/2302)

* spidercoordinator: allow set ip addr to hijackCIDR : [PR 2314](https://github.com/spidernet-io/spiderpool/pull/2314)

* spidermultusconfig: IPAM can be disabled : [PR 2317](https://github.com/spidernet-io/spiderpool/pull/2317)

* update controller-runtime,pyroscope sdk : [PR 2316](https://github.com/spidernet-io/spiderpool/pull/2316)

* spidercoordinator: add new podCIDRType "auto" : [PR 2326](https://github.com/spidernet-io/spiderpool/pull/2326)

* integrate rdma cni and rdma device plugin : [PR 2382](https://github.com/spidernet-io/spiderpool/pull/2382)

* update opentelemetry to v1.19.0 : [PR 2387](https://github.com/spidernet-io/spiderpool/pull/2387)

* chart: sriov network operator : [PR 2386](https://github.com/spidernet-io/spiderpool/pull/2386)

* support kubevirt vm static ip  : [PR 2360](https://github.com/spidernet-io/spiderpool/pull/2360)

* init: create default spidermultusconfig network if it isn't empty : [PR 2451](https://github.com/spidernet-io/spiderpool/pull/2451)

* docker-image: build new image for all plugins  : [PR 2457](https://github.com/spidernet-io/spiderpool/pull/2457)

* spiderAgent: install cni,ovs and rdma in init-container : [PR 2466](https://github.com/spidernet-io/spiderpool/pull/2466)

* coordinator: fix the eth0 source IP for the packet going through veth0 : [PR 2489](https://github.com/spidernet-io/spiderpool/pull/2489)



***

## Changed Feature

* spidermultusconfig: make defaultCniCRName to empty by default : [PR 2362](https://github.com/spidernet-io/spiderpool/pull/2362)

* improve GetTopController method : [PR 2370](https://github.com/spidernet-io/spiderpool/pull/2370)

* spidermultusconfig: fix panic if spidermultusconfig.spec is empty : [PR 2444](https://github.com/spidernet-io/spiderpool/pull/2444)

* spidercoordinator: fix auto fetch podCIDRType : [PR 2434](https://github.com/spidernet-io/spiderpool/pull/2434)

* fix ipam block with ip address used out : [PR 2518](https://github.com/spidernet-io/spiderpool/pull/2518)

* spiderpool-controller readiness health check failure : [PR 2532](https://github.com/spidernet-io/spiderpool/pull/2532)

* fix multusName usage bug with wrong net-attach-def namespace : [PR 2514](https://github.com/spidernet-io/spiderpool/pull/2514)



***

## Totoal 

Pull request number: 116

[ Commits ](https://github.com/spidernet-io/spiderpool/compare/v0.7.0...v0.8.0)
