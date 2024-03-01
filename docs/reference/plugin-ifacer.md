# CNI meta-plugin: ifacer

## Introduction

When Pods use VLAN networks, network administrators may need to manually configure various VLAN or Bond interfaces on the nodes in advance. This process can be tedious and error-prone. Spiderpool provides a CNI meta-plugin called `ifacer`.
This plugin dynamically creates VLAN sub-interfaces or Bond interfaces on the nodes during Pod creation, based on the provided `ifacer` configuration, greatly simplifying the configuration workload. In the following sections, we will delve into this plugin.

## Feature

- Support dynamic creation of VLAN sub-interfaces
- Support dynamic creation of Bond interfaces

## Notes

1. The VLAN/Bond interfaces created by this plugin will be lost when the node restarts, but they will be automatically recreated upon the Pod restarts.
2. Deleting existed VLAN/Bond interfaces is not supported.
3. Configuring the address of VLAN/Bond interfaces during creation is not supported.
4. If your OS(such as Fedora, CentOS, etc.) uses NetworkManager, Highly recommend configuring following configuration file at `/etc/NetworkManager/conf.d/spidernet.conf` to prevent interference from NetworkManager with Vlan and Bond interfaces created by `Ifacer`:

    ```shell
    ~# INTERFACE=<your_interface_name>
    ~# cat > /etc/NetworkManager/conf.d/spidernet.conf <<EOF
    [keyfile]
    unmanaged-devices=interface-name:^veth*;interface-name:${IFACER_INTERFACE}
    EOF
    ~# systemctl restart NetworkManager
    ```

## Prerequisite

There are no specific requirements including Kubernetes or Kernel versions for using this plugin. During the installation of Spiderpool, the plugin will be automatically installed in the `/opt/cni/bin/` directory on each host. You can verify by checking for the presence of the `ifacer` binary in that directory on each host.

## How to use

Examples please see [Ifacer Configuration](../usage/spider-multus-config.md#Ifacer-Configurations)
