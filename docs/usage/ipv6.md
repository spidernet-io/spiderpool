# Spiderpool IPv6 support

## Description

Configure spiderpool IP address allocation to use dual stack or IPv4 only.

## Features

spiderpool supports:

- **Dual stack** (default)

  Each workload can get IPv4 and IPv6 addresses, and can communicate over IPv4 or IPv6.

- **IPv4 only**

  Each workload can acquire IPv4 addresses, and can communicate over IPv4.

- **IPv6 only**

  Each workload can acquire IPv6 addresses, and can communicate over IPv6.

## Get Started

### Enable IPv4 only

Firstly, please ensure you have installed the spiderpool and configure the CNI file, refer [install](./install.md) for details

Check configmap `spiderpool-conf` property `enableIPv4` whether is already set to `true` and property `enableIPv6` whether is set to `false`.

```shell
kubectl -n kube-system get configmap spiderpool-conf -o yaml
```

If you want to update it `true`, just execute `helm upgrade --set enableIPv4=true --set enableIPv6=false`

### Enable dual stack

Same with the upper steps, just execute `helm upgrade --set enableIPv4=true --set enableIPv6=true`
