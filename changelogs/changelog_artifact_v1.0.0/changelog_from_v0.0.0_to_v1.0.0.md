
# v1.0.0
Welcome to the v1.0.0 release of Spiderpool!
Compared with version:v0.0.0, version:v1.0.0 has the following updates.

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

* remove crd subnet/ippool deprecated property vlan default value : [PR 2576](https://github.com/spidernet-io/spiderpool/pull/2576)

* no interface name in annotation to support multiple NIC : [PR 2618](https://github.com/spidernet-io/spiderpool/pull/2618)

* spidermultusconfig: It's able to configure bandwidth for sriov config : [PR 2637](https://github.com/spidernet-io/spiderpool/pull/2637)

* add e2e ovs installation and ovs net-attach-def configurations : [PR 2469](https://github.com/spidernet-io/spiderpool/pull/2469)

* support spidersubnet single IP in dual stack : [PR 2821](https://github.com/spidernet-io/spiderpool/pull/2821)

* feature: support infiniband with ib-sriov and ipoib cni : [PR 2815](https://github.com/spidernet-io/spiderpool/pull/2815)

* SpiderMultusConfig: support empty config with custom type : [PR 2862](https://github.com/spidernet-io/spiderpool/pull/2862)

* SpiderMultusConfig: support empty config with custom type : [PR 2933](https://github.com/spidernet-io/spiderpool/pull/2933)

* coordinator: Add a new filed "txQueueLen" : [PR 2650](https://github.com/spidernet-io/spiderpool/pull/2650)

* IP reclaim:  differentiate stateless workload under deleting-timeout state on ready node and not-ready node : [PR 3002](https://github.com/spidernet-io/spiderpool/pull/3002)

* docs: bandwidth for ipvlan datapath : [PR 3137](https://github.com/spidernet-io/spiderpool/pull/3137)

* synchronize clusterIP CIDR from serviceCIDR to support k8s 1.29 : [PR 3132](https://github.com/spidernet-io/spiderpool/pull/3132)

* release conflicted ip of stateless workload to trigger assigning a new one : [PR 3081](https://github.com/spidernet-io/spiderpool/pull/3081)

* subnet feature: support to turn on or off the feature of managing automatic-ippool : [PR 3241](https://github.com/spidernet-io/spiderpool/pull/3241)

* Rework spidercoordinator informer to update pod and service cidr : [PR 3249](https://github.com/spidernet-io/spiderpool/pull/3249)

* chart: Support configure ifNames for rdmaSharedDevicePlugin : [PR 3335](https://github.com/spidernet-io/spiderpool/pull/3335)

* feature: support wildcard match for IPPool : [PR 3262](https://github.com/spidernet-io/spiderpool/pull/3262)

* feature: run a clean-up job when uninstalling : [PR 3339](https://github.com/spidernet-io/spiderpool/pull/3339)

* DRA: Integrates with DRA and CDI : [PR 3329](https://github.com/spidernet-io/spiderpool/pull/3329)

* fix(charts): Remove unnecessary sensitive permissions for DaemonSet agent and Pod init : [PR 3522](https://github.com/spidernet-io/spiderpool/pull/3522)

* update crds for spiderpool dra feature : [PR 3527](https://github.com/spidernet-io/spiderpool/pull/3527)

* spiderclaimparameter: add webhook to verify the create and update : [PR 3668](https://github.com/spidernet-io/spiderpool/pull/3668)

* update version of CNI plugins : [PR 3733](https://github.com/spidernet-io/spiderpool/pull/3733)

* update sriov-operator version to v1.3.0 : [PR 3716](https://github.com/spidernet-io/spiderpool/pull/3716)

* spiderpool-agent: support to configure the sysctl config for node : [PR 3772](https://github.com/spidernet-io/spiderpool/pull/3772)

* doc: ai with macvlan : [PR 3870](https://github.com/spidernet-io/spiderpool/pull/3870)

* agent set rp_filter to 0 for the each node : [PR 3898](https://github.com/spidernet-io/spiderpool/pull/3898)

* Support ipv6 subnet with CIDR mask bigger than 64 : [PR 3804](https://github.com/spidernet-io/spiderpool/pull/3804)

* Add chainCNI support for spidermultusconfig : [PR 3918](https://github.com/spidernet-io/spiderpool/pull/3918)

* Add a pod mutating webhook to auto inject the pod network resources : [PR 4179](https://github.com/spidernet-io/spiderpool/pull/4179)

* Add a flag to configure an link-local address to veth0 for istio : [PR 4206](https://github.com/spidernet-io/spiderpool/pull/4206)

* Add RDAM metrics : [PR 4112](https://github.com/spidernet-io/spiderpool/pull/4112)

* Add pod owner attributes for RDMA metrics : [PR 4294](https://github.com/spidernet-io/spiderpool/pull/4294)

* chart: update plugins image : [PR 4406](https://github.com/spidernet-io/spiderpool/pull/4406)

* Add cluster dropdown menu for grafana dashboard : [PR 4409](https://github.com/spidernet-io/spiderpool/pull/4409)

* fix: multus fails to reach api server when the old service account is out of data && update multus to v4 : [PR 4393](https://github.com/spidernet-io/spiderpool/pull/4393)

* add annotation cni.spidernet.io/network-resource-inject : [PR 4421](https://github.com/spidernet-io/spiderpool/pull/4421)

* Remove deprecated fields for v2beta1 APIs : [PR 4549](https://github.com/spidernet-io/spiderpool/pull/4549)



***

## Changed Feature

* [docs]: optimize doc format for spiderpool : [PR 2312](https://github.com/spidernet-io/spiderpool/pull/2312)

* charts default value adjustment : [PR 2306](https://github.com/spidernet-io/spiderpool/pull/2306)

* spidermultusconfig: set default value if it not set : [PR 2603](https://github.com/spidernet-io/spiderpool/pull/2603)

* coordinator: Use ARP to detect the gateway rather than ICMP : [PR 3584](https://github.com/spidernet-io/spiderpool/pull/3584)



***

## Fix

* spidermultusconfig: make defaultCniCRName to empty by default : [PR 2362](https://github.com/spidernet-io/spiderpool/pull/2362)

* improve GetTopController method : [PR 2370](https://github.com/spidernet-io/spiderpool/pull/2370)

* spidermultusconfig: fix panic if spidermultusconfig.spec is empty : [PR 2444](https://github.com/spidernet-io/spiderpool/pull/2444)

* spidercoordinator: fix auto fetch podCIDRType : [PR 2434](https://github.com/spidernet-io/spiderpool/pull/2434)

* fix ipam block with ip address used out : [PR 2518](https://github.com/spidernet-io/spiderpool/pull/2518)

* spiderpool-controller readiness health check failure : [PR 2532](https://github.com/spidernet-io/spiderpool/pull/2532)

* fix multusName usage bug with wrong net-attach-def namespace : [PR 2514](https://github.com/spidernet-io/spiderpool/pull/2514)

* coordinator: set random mac addres for veth device when creating it : [PR 2580](https://github.com/spidernet-io/spiderpool/pull/2580)

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

* fix: the parent interface is down, set it to up before creating the vlan sub-interface : [PR 3088](https://github.com/spidernet-io/spiderpool/pull/3088)

* Spidercoordinator: It able to get CIDR from kubeadm-config : [PR 3062](https://github.com/spidernet-io/spiderpool/pull/3062)

* fix coordinator upgrade panic with CRD property empty : [PR 3118](https://github.com/spidernet-io/spiderpool/pull/3118)

* enable coordinate to support serviceCIDR according to a matched k8s version. : [PR 3168](https://github.com/spidernet-io/spiderpool/pull/3168)

* use helm charts value control coordinator components startup : [PR 3182](https://github.com/spidernet-io/spiderpool/pull/3182)

* Support getting serviceCIDR from spec.Containers[0].Args of kube-controller-manager Pod : [PR 3243](https://github.com/spidernet-io/spiderpool/pull/3243)

* Fix panic in spidercoordinator informer : [PR 3269](https://github.com/spidernet-io/spiderpool/pull/3269)

* spidercoordinator: Enhance the edge case : [PR 3284](https://github.com/spidernet-io/spiderpool/pull/3284)

* spidermultusconfig: add missing filed for generateCoordinatorCNIConf : [PR 3283](https://github.com/spidernet-io/spiderpool/pull/3283)

* Spidercoordinator: sync kubeadm-config event to trigger the status update : [PR 3291](https://github.com/spidernet-io/spiderpool/pull/3291)

* coordinator: rework GetDefaultRouteInterface to get pod's default route nic : [PR 3302](https://github.com/spidernet-io/spiderpool/pull/3302)

* coordinator: ensure hijickRoute's gw is from hostIPRouteForPod : [PR 3358](https://github.com/spidernet-io/spiderpool/pull/3358)

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

* Fix miss rdma dependency : [PR 4263](https://github.com/spidernet-io/spiderpool/pull/4263)

* Remove ServiceMonitor dup namespaceSelector : [PR 4271](https://github.com/spidernet-io/spiderpool/pull/4271)

* Update GrafanaDashboard : [PR 4264](https://github.com/spidernet-io/spiderpool/pull/4264)

* Reduce excessive WARN logs for forbidden resource access : [PR 4356](https://github.com/spidernet-io/spiderpool/pull/4356)

* Fix:  one NIC's IP pool shortage depleted IPs of other NICs in a multi-NIC setup. : [PR 4379](https://github.com/spidernet-io/spiderpool/pull/4379)

* Fix: statefulSet applications failed to create in multi-NIC scenarios. : [PR 4359](https://github.com/spidernet-io/spiderpool/pull/4359)

* Fix: the pod fails to run because the certificate of the pod webhook  is not up to data after helm upgrading : [PR 4420](https://github.com/spidernet-io/spiderpool/pull/4420)

* fix: fail to get podCIDR and serviceCIDR : [PR 4366](https://github.com/spidernet-io/spiderpool/pull/4366)

* Fix controller panic in cilium ipam is multi-pool : [PR 4433](https://github.com/spidernet-io/spiderpool/pull/4433)

* Detect IP conflicts before gateway detection to fix communication fail : [PR 4474](https://github.com/spidernet-io/spiderpool/pull/4474)

* coodirnator: optimize the detectiong timeout  for ip conflict and gateway detection : [PR 4504](https://github.com/spidernet-io/spiderpool/pull/4504)

* Detect IPConflicting and gatewayReachable in ipam rather than coordinator : [PR 4560](https://github.com/spidernet-io/spiderpool/pull/4560)



***

## Total 

Pull request number: 1867

[ Commits ](https://github.com/spidernet-io/spiderpool/compare/v0.0.0...v1.0.0)
