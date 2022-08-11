# Architecture

![Architecture](./images/arch.jpg )

Spiderpool consists of following components:

* Spiderpool IPAM plugin, a binary installed on each host. It is called by a CNI plugin to assign and release IP for a pod

* spiderpool-agent, deployed as a daemonset. It receives IPAM requests from the IPAM plugin, assigns and releases IP from the ippool resource

* spiderpool-controller, deployed as a deployment.

  * It takes charge of reclaiming IP resource in ippool, to prevent IP from leaking after the pod does not take it. Refer to [Resource Reclaim](./gc.md) for details.

  * It uses a webhook to watch the ippool resource, help the administrator to validate creation, modification, and deletion.

* spiderpoolctl, a CLI tool for debugging

## CRDs

Spiderpool supports for the following CRDs:

* ippool CRD. It is used to store the IP resource for a subnet. Refer to [ippool](./ippool.md) for detail.

* workloadendpoint CRD. It is used to store the IP assigned to a pod. Refer to [workloadendpoint](./workloadendpoint.md) for detail.

* reservedip CRD. It is used to set the reserved IP, which will not be assigned to a pod even if you have set it in the ippool. Refer to [reservedip](./reservedip.md) for detail.
