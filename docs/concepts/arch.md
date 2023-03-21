# Architecture

![Architecture](./images/arch.jpg )

Spiderpool consists of following components:

* Spiderpool IPAM plugin, a binary installed on each host. It is called by a CNI plugin to assign and release IP for a pod.

* spiderpool-agent, deployed as a daemonset. It receives IPAM requests from the IPAM plugin, assigns and releases IP from the ippool resource.

* spiderpool-controller, deployed as a deployment.

  * It takes charge of reclaiming IP resource in ippool, to prevent IP from leaking after the pod does not take it. Refer to [Resource Reclaim](./gc.md) for details.

  * It uses a webhook to watch the ippool resource, help the administrator to validate creation, modification, and deletion.

* spiderpoolctl, a CLI tool for debugging.

## CRDs

Spiderpool supports for the following CRDs:

* SpiderSubnet CRD. It is used to represent a collection of IP addresses from which Spiderpool expects SpiderIPPool IPs to be assigned. Refer to [SpiderSubnet](./spidersubnet.md) for detail.

* SpiderReservedIP CRD. It is used to represent a collection of IP addresses that Spiderpool expects not to be allocated. Refer to [SpiderReservedIP](./spiderreservedip.md) for detail.

* SpiderIPPool CRD. It is used to represent a collection of IP addresses from which Spiderpool expects endpoint IPs to be assigned. Refer to [SpiderIPPool](./spiderippool.md) for detail.

* SpiderEndpoint CRD. It is used to represent IP address allocation details for a specific endpoint object. Refer to [SpiderEndpoint](./spiderendpoint.md) for detail.
