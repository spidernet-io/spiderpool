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
- For error like `failed to get IPPool candidates from Subnet: no matching auto-created IPPool candidate with matchLables`, you should check `spiderpool-controller` logs.
