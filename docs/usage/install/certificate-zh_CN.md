# 证书

[**English**](./certificate.md) | **简体中文**

Spiderpool-controller 需要 TLS 证书来运行 webhook 服务。您可以通过以下几种方式进行配置。

## 自动生成证书

使用 Helm 的模板函数 [genSignedCert](https://helm.sh/docs/chart_template_guide/function_list/#gensignedcert) 生成 TLS 证书。这是最简单、最常用的配置方式：

```bash
helm install spiderpool spiderpool/spiderpool --namespace kube-system \
  --set spiderpoolController.tls.method=auto
```

注意，参数 `spiderpoolController.tls.method` 的默认值就是 `auto`。

## 使用自己提供的证书

如果您希望使用自签名证书运行 spiderpool-controller，可以选择 `provided` 模式。您可以使用 OpenSSL 生成证书，也可以运行以下脚本：

```bash
wget https://raw.githubusercontent.com/spidernet-io/spiderpool/main/tools/cert/generateCert.sh
```

生成证书：

```bash
chmod +x generateCert.sh && ./generateCert.sh "/tmp/tls"

CA=`cat /tmp/tls/ca.crt | base64 -w0 | tr -d '\n'`
SERVER_CERT=`cat /tmp/tls/server.crt | base64 -w0 | tr -d '\n'`
SERVER_KEY=`cat /tmp/tls/server.key | base64 -w0 | tr -d '\n'`
```

然后，以 `provided` 模式部署 Spiderpool：

```bash
helm install spiderpool spiderpool/spiderpool --namespace kube-system \
  --set spiderpoolController.tls.method=provided \
  --set spiderpoolController.tls.provided.tlsCa=${CA} \
  --set spiderpoolController.tls.provided.tlsCert=${SERVER_CERT} \
  --set spiderpoolController.tls.provided.tlsKey=${SERVER_KEY}
```

## 使用 cert-manager 证书

**不建议直接使用此模式**，因为 Spiderpool 需要 cert-manager 提供的 TLS 证书，而 cert-manager 又需要 Spiderpool 分配的 IP 地址（循环依赖）。

因此，如有可能，您必须先在 Kubernetes 集群中使用其他 IPAM CNI [部署 cert-manager](https://cert-manager.io/docs/installation/)，然后再部署 Spiderpool。

```bash
helm install spiderpool spiderpool/spiderpool --namespace kube-system \
  --set spiderpoolController.tls.method=certmanager \
  --set spiderpoolController.tls.certmanager.issuerName=${CERT_MANAGER_ISSUER_NAME}
```
