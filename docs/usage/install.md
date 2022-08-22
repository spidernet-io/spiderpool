# Install

the spiderpool needs install webhook of kube-apiserver, so it needs tls certificates.

there are two ways to install it, the one is with cert-manager, the other one is to generate self-signed certificate.

## install spiderpool

### install By Self-signed Certificates

this way is simple, there is no any dependency. The project provides a script to generate tls certificate

the following is a ipv4-only example

```shell
helm repo add spiderpool https://spidernet-io.github.io/spiderpool

git clone https://github.com/spidernet-io/spiderpool.git
cd spiderpool

# generate the certificates
tools/cert/generateCert.sh "/tmp/tls"
CA=`cat /tmp/tls/ca.crt  | base64 -w0 | tr -d '\n' `
SERVER_CERT=` cat /tmp/tls/server.crt | base64 -w0 | tr -d '\n' `
SERVER_KEY=` cat /tmp/tls/server.key | base64 -w0 | tr -d '\n' `

# for default ipv4 ippool
# CIDR
Ipv4Subnet="172.20.0.0/16"
# available IP resource
Ipv4Range="172.20.0.10-172.20.0.200"

# deploy the spiderpool
helm install spiderpool spiderpool/spiderpool --wait --namespace kube-system \
  --set spiderpoolController.tls.method=provided \
  --set spiderpoolController.tls.provided.tlsCert="${SERVER_CERT}" \
  --set spiderpoolController.tls.provided.tlsKey="${SERVER_KEY}" \
  --set spiderpoolController.tls.provided.tlsCa="${CA}" \
  --set feature.enableIPv4=true --set feature.enableIPv6=false \
  --set clusterDefaultPool.installIPv4IPPool=true  \
  --set clusterDefaultPool.ipv4Subnet=${Ipv4Subnet} --set clusterDefaultPool.ipv4IPRanges={${Ipv4Range}}
```

> NOTICE:
>
> (1) if default ippool is installed by helm, please add '--wait' parament in the helm command. Because, the spiderpool will install
> webhook for checking spiderippool CRs, if the spiderpool controller pod is not running, the default ippool will fail to apply and the helm install command fails
> Or else, you could create default ippool after helm installation.
>
> (2) spiderpool-controller pod is running as hostnetwork mode, and it needs take host port,
> it is set with podAntiAffinity to make sure that a node will only run a spiderpool-controller pod.
> so, if you set the replicas number of spiderpool-controller to be bigger than 2, make sure there is enough nodes

the following is a dual-stack example

```shell
helm repo add spiderpool https://spidernet-io.github.io/spiderpool

# generate the certificates
tools/cert/generateCert.sh "/tmp/tls"
CA=`cat /tmp/tls/ca.crt  | base64 -w0 | tr -d '\n' `
SERVER_CERT=` cat /tmp/tls/server.crt | base64 -w0 | tr -d '\n' `
SERVER_KEY=` cat /tmp/tls/server.key | base64 -w0 | tr -d '\n' `

# for default ipv4 ippool
# CIDR
Ipv4Subnet="172.20.0.0/16"
# available IP resource
Ipv4Range="172.20.0.10-172.20.0.200"
# for default ipv6 ippool
# CIDR
Ipv6Subnet="fd00::/112"
# available IP resource
Ipv6Range="fd00::10-fd00::200"

# deploy the spiderpool
helm install spiderpool spiderpool/spiderpool --wait --namespace kube-system \
  --set spiderpoolController.tls.method=provided \
  --set spiderpoolController.tls.provided.tlsCert="${SERVER_CERT}" \
  --set spiderpoolController.tls.provided.tlsKey="${SERVER_KEY}" \
  --set spiderpoolController.tls.provided.tlsCa="${CA}" \
  --set feature.enableIPv4=true --set feature.enableIPv6=true \
  --set clusterDefaultPool.installIPv4IPPool=true  --set clusterDefaultPool.installIPv6IPPool=true  \
  --set clusterDefaultPool.ipv4Subnet=${Ipv4Subnet} --set clusterDefaultPool.ipv4IPRanges={${Ipv4Range}} \
  --set clusterDefaultPool.ipv6Subnet=${Ipv6Subnet} --set clusterDefaultPool.ipv6IPRanges={${Ipv6Range}}
```

### install By Cert-manager

the way is not a common situation, because cert-manager needs CNI to create its pod,
but as IPAM, spiderpool is still not installed to provide IP resource. It means cert-manager and spiderpool need each other to finish installation.

Therefore, the way may implement on following situation:

- after spiderpool is installed by self-signed certificates, and the cert-manager is deployed, then it could change to cert-manager scheme
- on cluster with [Multus CNI](https://github.com/k8snetworkplumbingwg/multus-cni), the cert-manager pods is deployed by other CNI, then spiderpool could be deployed by cert-manager

```shell
helm repo add spiderpool https://spidernet-io.github.io/spiderpool

# for default ipv4 ippool
# CIDR
ipv4_subnet="172.20.0.0/16"
# available IP resource
ipv4_range="172.20.0.10-172.20.0.200"

helm install spiderpool spiderpool/spiderpool --wait --namespace kube-system \
  --set spiderpoolController.tls.method=certmanager \
  --set spiderpoolController.tls.certmanager.issuerName=${CERT_MANAGER_ISSUER_NAME} \
  --set feature.enableIPv4=true --set feature.enableIPv6=false \
  --set clusterDefaultPool.installIPv4IPPool=true --set clusterDefaultPool.installIPv6IPPool=false \
  --set clusterDefaultPool.ipv4Subnet=${ipv4_subnet} \
  --set clusterDefaultPool.ipv4IPRanges={${ipv4_ip_range}}
```

## configure CNI  

after installation of the spiderpool, please edit CNI configuration file under /etc/cni/net.d/ .

The following is an example for macvlan CNI

```
{
  "cniVersion": "0.3.1",
  "type": "macvlan",
  "mode": "bridge",
  "master": "eth0",
  "name": "macvlan-cni-default",
  "ipam": {
    "type": "spiderpool"
  }
}
```

you cloud refer [config](../concepts/config.md) for the detail of the IPAM configuration
