# Install

the spiderpool need install webhook of kube-apiserver, so it needs tls certificates.

there are two ways to install it, the one is with cert-manager, the other one is generating self-signed certificate

## By Self-signed Certificates

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
helm install spiderpool spidernet/spiderpool --namespace spidernet-system \
  --set spiderpoolController.tls.method=provided \
  --set spiderpoolController.tls.server.cert="${SERVER_CERT}" \
  --set spiderpoolController.tls.server.key="${SERVER_KEY}" \
  --set spiderpoolController.tls.server.ca="${CA}" \
  --set ipFamily.enableIPv4=true --set ipFamily.enableIPv6=false \
  --set clusterDefaultPool.installIPv4IPPool=true  \
  --set clusterDefaultPool.ipv4Subnet=${Ipv4Subnet} --set clusterDefaultPool.ipv4IPRanges={${Ipv4Range}}
```

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
helm install spiderpool spiderpool/spiderpool --namespace kube-system \
  --set spiderpoolController.tls.method=provided \
  --set spiderpoolController.tls.server.cert="${SERVER_CERT}" \
  --set spiderpoolController.tls.server.key="${SERVER_KEY}" \
  --set spiderpoolController.tls.server.ca="${CA}" \
  --set ipFamily.enableIPv4=true --set ipFamily.enableIPv6=true \
  --set clusterDefaultPool.installIPv4IPPool=true  --set clusterDefaultPool.installIPv6IPPool=true  \
  --set clusterDefaultPool.ipv4Subnet=${Ipv4Subnet} --set clusterDefaultPool.ipv4IPRanges={${Ipv4Range}} \
  --set clusterDefaultPool.ipv6Subnet=${Ipv6Subnet} --set clusterDefaultPool.ipv6IPRanges={${Ipv6Range}}
```

## By Cert-manager

the way is not a common situation, because cert-manager needs CNI to create its pod,
but as IPAM, spiderpool is still not installed to provide IP resource. It means cert-manager and spiderpool need each other to finish installation.

Therefore, the way may implement on following situation:

- after spiderpool is installed by self-signed certificates, and the cert-manager is deployed, then it could change to cert-manager scheme
- on cluster with [Multus CNI](https://github.com/k8snetworkplumbingwg/multus-cni), the cert-manager pods is deployed by other CNI, then spiderpool could be deployed by cert-manager

```shell
heml repo add spiderpool https://spidernet-io.github.io/spiderpool

# for default ipv4 ippool
# CIDR
ipv4_subnet="172.20.0.0/16"
# available IP resource
ipv4_range="172.20.0.10-172.20.0.200"

helm install spiderpool spiderpool/spiderpool --namespace kube-system \
  --set spiderpoolController.tls.method=certmanager \
  --set spiderpoolController.tls.certmanager.issuerName=${CERT_MANAGER_ISSUER_NAME} \
  --set ipFamily.enableIPv4=true --set ipFamily.enableIPv6=false \
  --set clusterDefaultPool.installIPv4IPPool=true --set clusterDefaultPool.installIPv6IPPool=false \
  --set clusterDefaultPool.ipv4Subnet=${ipv4_subnet} \
  --set clusterDefaultPool.ipv4IPRanges={${ipv4_ip_range}}
```
