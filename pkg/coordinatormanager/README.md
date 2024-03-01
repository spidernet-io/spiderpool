# SpiderCoordinator Informer

The responsibility of the SpiderCoordinator Informer is to monitor changes in the Pod and Service CIDR of the cluster and automatically update them in the Status of SpiderCoordinator. However, due to different sources of Pod and Service CIDR in different cluster environments, a uniform approach cannot be adopted. Additionally, to avoid coroutine leaks and confusion in updating status, this document outlines the approach for easier maintenance of the code in the future.

## How to work

* Before starting the SpiderCoordinator Informer, check the CNI type of the cluster.

* If it is Calico, start a persistent coroutine to watch for changes in its IPPools. If the PodCIDRType of the SpiderCoordinator default instance is auto or calico, increment the SpiderCoordinator workqueue by 1 to notify SpiderCoordinator to update.

> Because Calico only provides the NewShareInformer method for v3 API, while the cluster stores ippools for v1. Therefore, only watch calico V1 API resources through the controller-runtime approach.

* If it is Cilium, start a persistent coroutine to watch for changes in ciliumpodippools.cilium.io. If the PodCIDRType of the SpiderCoordinator default instance is auto or calico, increment the SpiderCoordinator workqueue by 1 to notify SpiderCoordinator to update.

* If the cluster supports the ServiceCIDR feature, start a persistent coroutine to watch for changes in its serviceCIDR. If any changes occur, increment the SpiderCoordinator workqueue by 1 to notify SpiderCoordinator to update.

* If the PodCIDRType of the SpiderCoordinator default instance is cluster, obtain the Pod and Service CIDR from kubeadm-config or kube-controller-manager Pod.

* All updates to Status are done through the Spidercoordinator Informer, and do not synchronize status within respective coroutines.

![work_flow](../../docs/images/spidercoordinator_informer.png)
