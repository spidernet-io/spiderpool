# Installation

*This guide shows how to install Spiderpool using [Helm](https://helm.sh/).* 

## Generic

Set up the Helm repository.

```bash
helm repo add spiderpool https://spidernet-io.github.io/spiderpool
```

Deploy Spiderpool using the default configuration options via Helm:

```bash
helm install spiderpool spiderpool/spiderpool --namespace kube-system
```

More details about [Spiderpool charts parameters](https://github.com/spidernet-io/spiderpool/blob/main/charts/spiderpool/README.md#parameters).

>It should be noted that the Pods of the spiderpool-controller have to run in the hostnetwork mode, because there may be no other IPAM CNI in the Kubernetes cluster that can allocate some IP addresses to them. In this regard, we used `podAntiAffinity` to ensure that different replicas of spiderpool-controller will not run on the same Node to avoid host port conflicts.
>
>The replicas of spiderpool-controller can be adjusted by setting parameter `spiderpoolController.replicas`, but please ensure that you have enough Nodes to run them.

## IP version

Spiderpool can work in IPv4 only, IPv6 only or dual-stack case. For example, you can deploy Spiderpool in this way to enable dual stack:

```bash
helm install spiderpool spiderpool/spiderpool --namespace kube-system \
  --set feature.enableIPv4=true \
  --set feature.enableIPv6=true
```

By default, `feature.enableIPv4` is enabled and `feature.enableIPv6` is not.

## Certificates

Spiderpool-controller needs TLS certificates to run webhook server. You can configure it in several ways.

### Auto

Use Helm's template function [genSignedCert](https://helm.sh/docs/chart_template_guide/function_list/#gensignedcert) to generate TLS certificates. This is the simplest and most common way to configure:

```bash
helm install spiderpool spiderpool/spiderpool --namespace kube-system \
  --set spiderpoolController.tls.method=auto
```

Note that the default value of parameter `spiderpoolController.tls.method` is `auto`.

### Provided

If you want to run spiderpool-controller with a self-signed certificate, `provided` would be a good choice. You can use OpenSSL to generate certificates, or run the following script:

```bash
wget https://raw.githubusercontent.com/spidernet-io/spiderpool/main/tools/cert/generateCert.sh
```

Generate the certificates:

```bash
chmod +x generateCert.sh && ./generateCert.sh "/tmp/tls"

CA=`cat /tmp/tls/ca.crt | base64 -w0 | tr -d '\n'`
SERVER_CERT=`cat /tmp/tls/server.crt | base64 -w0 | tr -d '\n'`
SERVER_KEY=`cat /tmp/tls/server.key | base64 -w0 | tr -d '\n'`
```

Then, deploy Spiderpool in the `provided` mode:

```bash
helm install spiderpool spiderpool/spiderpool --namespace kube-system \
  --set spiderpoolController.tls.method=provided \
  --set spiderpoolController.tls.provided.tlsCa=${CA} \
  --set spiderpoolController.tls.provided.tlsCert=${SERVER_CERT} \
  --set spiderpoolController.tls.provided.tlsKey=${SERVER_KEY}
```

### Cert-manager

It is **not recommended to use this mode directly**, because the Spiderpool requires the TLS certificates provided by cert-manager, while the cert-manager requires the IP address provided by Spiderpool (cycle reference).

Therefore, if possible, you must first [deploy cert-manager](https://cert-manager.io/docs/installation/) using other IPAM CNI in the Kubernetes cluster, and then deploy Spiderpool.

```bash
helm install spiderpool spiderpool/spiderpool --namespace kube-system \
  --set spiderpoolController.tls.method=certmanager \
  --set spiderpoolController.tls.certmanager.issuerName=${CERT_MANAGER_ISSUER_NAME}
```

## Cluster default IPPool

The cluster default IPPool usually serves some components that need IP addresses when the Kubernetes cluster is initialized, such as CoreDNS.

Prepare the IP address ranges you need to use next:

```bash
# "CIDR"
IPV4_SUBNET_YOU_EXPECT="172.18.40.0/24"
# "IP" or "IP-IP"
IPV4_IPRANGES_YOU_EXPECT="172.18.40.40-172.20.40.200"
```

Let's create an IPv4 IPPool while deploying Spiderpool, and configure it as the cluster default IPPool in the [global Configmap configuration](https://github.com/spidernet-io/spiderpool/blob/main/docs/concepts/config.md#configmap-configuration). 

```bash
helm install spiderpool spiderpool/spiderpool --namespace kube-system \
  --set feature.enableIPv4=true \
  --set clusterDefaultPool.installIPv4IPPool=true \
  --set clusterDefaultPool.ipv4Subnet=${IPV4_SUBNET_YOU_EXPECT} \
  --set clusterDefaultPool.ipv4IPRanges={${IPV4_IPRANGES_YOU_EXPECT}}
```

IPv6 IPPool is similar:

```bash
helm install spiderpool spiderpool/spiderpool --namespace kube-system \
  --set feature.enableIPv6=true \
  --set clusterDefaultPool.installIPv6IPPool=true \
  --set clusterDefaultPool.ipv6Subnet=${IPV6_SUBNET_YOU_EXPECT} \
  --set clusterDefaultPool.ipv6IPRanges={${IPV6_IPRANGES_YOU_EXPECT}}
```

## Full example

Here is a general deployment example, which satisfies the following conditions:

- Dual stack.
- Generate TLS certificates in `auto` mode.
- Create and configure the cluster default IPv4/6 IPPool.

```bash
helm install spiderpool spiderpool/spiderpool --namespace kube-system \
  --set spiderpoolController.tls.method=auto \
  --set feature.enableIPv4=true \
  --set feature.enableIPv6=true \
  --set clusterDefaultPool.installIPv4IPPool=true \
  --set clusterDefaultPool.installIPv6IPPool=true \
  --set clusterDefaultPool.ipv4Subnet=${IPV4_SUBNET_YOU_EXPECT} \
  --set clusterDefaultPool.ipv4IPRanges={${IPV4_IPRANGES_YOU_EXPECT}} \
  --set clusterDefaultPool.ipv6Subnet=${IPV6_SUBNET_YOU_EXPECT} \
  --set clusterDefaultPool.ipv6IPRanges={${IPV6_IPRANGES_YOU_EXPECT}}
```

## Update CNI network configuration

Finally, you should edit the [CNI network configuration](https://www.cni.dev/docs/spec/#section-1-network-configuration-format) with the default path `/etc/cni/net.d` on the related Nodes so that the Main CNI can use Spiderpool IPAM CNI to allocate IP addresses. Replace the `ipam` object with the following:

```json
"ipam":{
    "type":"spiderpool"
}
```

The following is an example for [macvlan CNI](https://www.cni.dev/plugins/current/main/macvlan/):

```json
{
    "cniVersion":"0.4.0",
    "type":"macvlan",
    "mode":"bridge",
    "master":"eth0",
    "name":"macvlan",
    "ipam":{
        "type":"spiderpool"
    }
}
```

## Uninstall

Generally, you can uninstall Spiderpool release in this way:

```bash
helm uninstall spiderpool -n kube-system
```

However, there are [finalizers](https://kubernetes.io/docs/concepts/overview/working-with-objects/finalizers/) in some CRs of Spiderpool, the `helm uninstall` cmd may not clean up all relevant CRs. Get this cleanup script and execute it to ensure that unexpected errors will not occur when deploying Spiderpool next time.

```bash
wget https://raw.githubusercontent.com/spidernet-io/spiderpool/main/tools/scripts/cleanCRD.sh
chmod +x cleanCRD.sh && ./cleanCRD.sh
```
