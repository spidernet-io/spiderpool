# Spiderpool IPv6 support

## Description

This page describes how to configure and allocate IP addresses with Spiderpool by using dual stack, IPv4 only, or IPv6 only.

## Features

Spiderpool supports:

- **Dual stack** (default)

  Each workload can get IPv4 and IPv6 addresses, and can communicate over IPv4 or IPv6.

- **IPv4 only**

  Each workload can acquire IPv4 addresses, and can communicate over IPv4.

- **IPv6 only**

  Each workload can acquire IPv6 addresses, and can communicate over IPv6.

## Get Started

### Enable IPv4 only

Firstly, please ensure you have installed the spiderpool and configured the CNI file, refer to [install](./install.md) for details

Check whether the property `enableIPv4` of the configmap `spiderpool-conf` is already set to `true` and whether the property `enableIPv6` is set to `false`.

```shell
kubectl -n kube-system get configmap spiderpool-conf -o yaml
```

If you want to update it with `true`, run `helm upgrade spiderpool spiderpool/spiderpool --set feature.enableIPv4=true --set feature.enableIPv6=false -n kube-system`.

### Enable dual stack

Same as the above, run `helm upgrade spiderpool spiderpool/spiderpool --set feature.enableIPv4=true --set feature.enableIPv6=true -n kube-system`.
