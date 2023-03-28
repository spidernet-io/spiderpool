# Certificates

Spiderpool-controller needs TLS certificates to run webhook server. You can configure it in several ways.

## Auto certificates

Use Helm's template function [genSignedCert](https://helm.sh/docs/chart_template_guide/function_list/#gensignedcert) to generate TLS certificates. This is the simplest and most common way to configure:

```bash
helm install spiderpool spiderpool/spiderpool --namespace kube-system \
  --set spiderpoolController.tls.method=auto
```

Note that the default value of parameter `spiderpoolController.tls.method` is `auto`.

## Provided certificates

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

## Cert-manager certificates

It is **not recommended to use this mode directly**, because the Spiderpool requires the TLS certificates provided by cert-manager, while the cert-manager requires the IP address provided by Spiderpool (cycle reference).

Therefore, if possible, you must first [deploy cert-manager](https://cert-manager.io/docs/installation/) using other IPAM CNI in the Kubernetes cluster, and then deploy Spiderpool.

```bash
helm install spiderpool spiderpool/spiderpool --namespace kube-system \
  --set spiderpoolController.tls.method=certmanager \
  --set spiderpoolController.tls.certmanager.issuerName=${CERT_MANAGER_ISSUER_NAME}
```
