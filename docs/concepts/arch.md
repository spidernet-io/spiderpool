# Architect

## architecture

![ architecture ](./images/arch.jpg )

the spiderpool consists of following components:

* spiderpool IPAM plugin, a binary installed on each host. It is called by CNI plugin to assign and release IP for a pod

* spiderpool agent, deployed as a daemonset. It receives IPAM request from IPAM plugin, assign and release IP from ippool resource

* spiderpool controller, deployed as a deployment.

  * It takes charge of reclaiming IP resource in ippool, to prevent IP from leaking after the pod does not take it. It refers to [Resource Reclaim](./gc.md) for details.

  * It uses webhook to watch ippool resource, help the administrator validate the creation, modification, deletion.

* spiderpoolctl, CLI tool for debugging

## CRD

the spiderpool designs following CRD:

* ippool CRD. It is used to store the IP resource for a subnet. It refers to [ippool](./ippool.md) for detail.

* workloadendpoint CRD. It is used to store the IP assigned to a pod. It refers to [workloadendpoint](./workloadendpoint.md) for detail.

* reservedip CRD. Is ti used to set reserved IP, who will not assign to pod, even set in the ippool. It refers to [reservedip](./reservedip.md) for detail.
