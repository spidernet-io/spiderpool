# Multi-Interfaces-Annotation

We can create multiple Interfaces with [Multus CNI](https://github.com/k8snetworkplumbingwg/multus-cni),
and let's use Multus CRD to achieve it.

## Setup Spiderpool

If you have not set up Spiderpool yet, follow the guide [Quick Installation](https://github.com/spidernet-io/spiderpool/blob/main/docs/usage/install.md) for instructions on how to install and simply configure Spiderpool.

## Setup Multus

If you have not set up Multus yet, follow the guide [Quick Installation](https://github.com/k8snetworkplumbingwg/multus-cni/blob/master/docs/quickstart.md#installation) for instructions on how to install and simply configure Multus.

## Get Started

Following the [Multus CRD configuration](https://github.com/k8snetworkplumbingwg/multus-cni/blob/master/docs/quickstart.md#storing-a-configuration-as-a-custom-resource) steps to implement multus CRD ``NetworkAttachmentDefinition``,
and create pod with multiple Interfaces.

