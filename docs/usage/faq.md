# FAQ

Frequently asked questions

**English** | [**简体中文**](./faq-zh_CN.md)

## General

### What is Spiderpool?

Spiderpool project is consist of several plugins include: `spiderpool`, `coordinator`, `ifacer`. The `spiderpool` basically is a IPAM plugin works for CNI main plugin to manage IP addresses for the container. The `coordinator` is a plugin that coordinate the routes. The `ifacer` plugin help you to create vlan sub-interface or create bond interfaces. The `coordinator` and `ifacer` plugin are used in CNI plugin chaining and they also optional to use.

## Configuration

### Why doesn't changing configmap configuration update the behavior of Spiderpool?

If you change the configmap `spiderpool-conf` configurations, you need to restart `spiderpool-agent` and `spiderpool-controller` components

## Operation

### Why SpiderSubnet feature not works well?

- For error like `Internal error occurred: failed calling webhook "spidersubnet.spiderpool.spidernet.io": the server could not find the requested resource`, you need to update configmap `spiderpool-conf` to enable SpiderSubnet feature and restart `spiderpool-agent` and  `spiderpool-controller` components.
- For error like `failed to get IPPool candidates from Subnet: no matching auto-created IPPool candidate with matchLables`, you should check `spiderpool-controller` logs. The spiderpool-controller component requires that the kubernetes cluster has kubernetes version not lower than `v1.21` once using the SpiderSubnet feature. The following error logs means your kubernetes cluster version is too low:

    ```text
    W1220 05:44:16.129916       1 reflector.go:535] k8s.io/client-go/informers/factory.go:150: failed to list *v1.CronJob: the server could not find the requested resource
    E1220 05:44:16.129978       1 reflector.go:147] k8s.io/client-go/informers/factory.go:150: Failed to watch *v1.CronJob: failed to list *v1.CronJob: the server could not find the requested resource
    ```

### Does Spiderpool IPAM relies on spiderpool-controller component?

spiderpool-controller component implements the webhook for the `Spec` property of SpiderSubnet, SpiderIPPool resources. And the spiderpool-agent component is the core of implementing the IPAM, once allocating the IP addresses it will update the SpiderIPPool resource `Status` property. The property belongs to [subresource](https://kubernetes.io/docs/tasks/extend-kubernetes/custom-resources/custom-resource-definitions/#subresources), so the request would not be intercepted by the spiderpool-controller webhook. Therefore, the IPAM doesn't rely on spiderpool-controller component.
