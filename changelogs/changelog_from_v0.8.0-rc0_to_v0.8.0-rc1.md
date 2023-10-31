
# v0.8.0-rc1

***

## New Feature

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

* improve GetTopController method : [PR 2370](https://github.com/spidernet-io/spiderpool/pull/2370)

* spidermultusconfig: fix panic if spidermultusconfig.spec is empty : [PR 2444](https://github.com/spidernet-io/spiderpool/pull/2444)

* spidercoordinator: fix auto fetch podCIDRType : [PR 2434](https://github.com/spidernet-io/spiderpool/pull/2434)



***

## Totoal 

Pull request number: 53

[ Commits ](https://github.com/spidernet-io/spiderpool/compare/v0.8.0-rc0...v0.8.0-rc1)
