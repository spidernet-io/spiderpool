# Quick Start

## install spiderpool

```shell
tools/cert/generateCert.sh "/tmp/tls"
CA=`cat /tmp/tls/ca.crt  | base64 -w0 | tr -d '\n' `
SERVER_CERT=` cat /tmp/tls/server.crt | base64 -w0 | tr -d '\n' `
SERVER_KEY=` cat /tmp/tls/server.key | base64 -w0 | tr -d '\n' `
helm install spiderpool spidernet/spiderpool --namespace kube-system \
  --set spiderpoolController.tls.server.cert="${SERVER_CERT}" \
  --set spiderpoolController.tls.server.key="${SERVER_KEY}" \
  --set spiderpoolController.tls.server.ca="${CA}" 
```

## create ippool

## create application

## get metrics
