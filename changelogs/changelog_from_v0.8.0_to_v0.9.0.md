
# v0.9.0

***

## New Feature

* no interface name in annotation to support multiple NIC : [PR 2618](https://github.com/spidernet-io/spiderpool/pull/2618)

* spidermultusconfig: It's able to configure bandwidth for sriov config : [PR 2637](https://github.com/spidernet-io/spiderpool/pull/2637)

* add e2e ovs installation and ovs net-attach-def configurations : [PR 2469](https://github.com/spidernet-io/spiderpool/pull/2469)

* support spidersubnet single IP in dual stack : [PR 2821](https://github.com/spidernet-io/spiderpool/pull/2821)

* feature: support infiniband with ib-sriov and ipoib cni : [PR 2815](https://github.com/spidernet-io/spiderpool/pull/2815)

* SpiderMultusConfig: support empty config with custom type : [PR 2862](https://github.com/spidernet-io/spiderpool/pull/2862)

* SpiderMultusConfig: support empty config with custom type : [PR 2933](https://github.com/spidernet-io/spiderpool/pull/2933)

* coordinator: Add a new filed "txQueueLen" : [PR 2650](https://github.com/spidernet-io/spiderpool/pull/2650)



***

## Changed Feature

* ifacer: Fix the slave with bond was not set if vlanId was set to 0 : [PR 2639](https://github.com/spidernet-io/spiderpool/pull/2639)

* fix path typo in spiderpool-agent yaml : [PR 2667](https://github.com/spidernet-io/spiderpool/pull/2667)

* init-pod: don't init multus CR if multus is disable : [PR 2756](https://github.com/spidernet-io/spiderpool/pull/2756)

* don't update multus configMap if multus don't install : [PR 2759](https://github.com/spidernet-io/spiderpool/pull/2759)

* coordinator: ensure detect gateway and ip conflict in pod's netns : [PR 2738](https://github.com/spidernet-io/spiderpool/pull/2738)

* e2e-fix: Unbound variable DEFAULT_CALICO_VERSION : [PR 2831](https://github.com/spidernet-io/spiderpool/pull/2831)

* add validation for IPAM IPPools annotation usage : [PR 2902](https://github.com/spidernet-io/spiderpool/pull/2902)

* spidercoordinator: It should update the status to NotReady if any errors occur : [PR 2929](https://github.com/spidernet-io/spiderpool/pull/2929)

* CI workflow: Updated obsolete method set-output. : [PR 2824](https://github.com/spidernet-io/spiderpool/pull/2824)

* fix:  spiderpool-agent crashes when kubevirt static IP feature is off  : [PR 2971](https://github.com/spidernet-io/spiderpool/pull/2971)

* fix chart: Values.multus.multusCNI.uninstall does not take effect : [PR 2974](https://github.com/spidernet-io/spiderpool/pull/2974)

* fix chart: Values.multus.multusCNI.uninstall does not take effect : [PR 2986](https://github.com/spidernet-io/spiderpool/pull/2986)

* single POD without controller is forbidden to use  SpiderSubnet feature : [PR 2952](https://github.com/spidernet-io/spiderpool/pull/2952)

* fix inherit subnet properties for ippool failure  : [PR 3011](https://github.com/spidernet-io/spiderpool/pull/3011)

* spidercoordinator:  fetch the serviceCIDR from  kubeControllerManager pod : [PR 3020](https://github.com/spidernet-io/spiderpool/pull/3020)



***

## Totoal 

Pull request number: 154

[ Commits ](https://github.com/spidernet-io/spiderpool/compare/v0.8.0...v0.9.0)
