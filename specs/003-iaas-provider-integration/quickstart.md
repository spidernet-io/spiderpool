# Quickstart: IaaS Network Provider Integration

**Feature**: IaaS Network Provider Integration  
**Phase**: Phase 1 - Configuration Infrastructure

---

## Overview

This quickstart guides you through configuring Spiderpool to integrate with an IaaS (Infrastructure as a Service) network provider. In Phase 1, we focus on setting up the configuration and TLS certificate infrastructure. The actual API integration (Phase 2) will be implemented in a future update.

---

## Prerequisites

1. Kubernetes cluster (v1.21+)
2. Spiderpool v0.9+ installed via Helm
3. IaaS provider service endpoint accessible from the cluster
4. TLS client certificate and key for mTLS authentication with the IaaS provider

---

## Step 1: Prepare TLS Certificates

Obtain the client certificate and private key from your IaaS provider. You should have:
- `client.crt`: X.509 client certificate
- `client.key`: RSA or EC private key

---

## Step 2: Create Kubernetes Secret

Create a TLS secret in your Spiderpool namespace (default: `spiderpool`):

```bash
# Create the secret
kubectl create secret tls iaas-provider-client-cert \
  --cert=client.crt \
  --key=client.key \
  -n spiderpool

# Verify the secret was created
kubectl get secret iaas-provider-client-cert -n spiderpool
```

**Output**:
```
NAME                         TYPE                DATA   AGE
iaas-provider-client-cert    kubernetes.io/tls   2      10s
```

---

## Step 3: Configure Spiderpool

Update your Spiderpool Helm values with the IaaS provider configuration:

### Option A: Using Helm CLI

```bash
helm upgrade spiderpool spiderpool/spiderpool \
  --namespace spiderpool \
  --set iaasNetworkProvider.url="iaas-network-provider:444" \
  --set iaasNetworkProvider.tlsSecret.name="iaas-provider-client-cert" \
  --set iaasNetworkProvider.tlsSecret.namespace="spiderpool"
```

### Option B: Using values.yaml

Edit your `values.yaml`:

```yaml
iaasNetworkProvider:
  # URL of the IaaS provider service (host:port)
  url: "iaas-network-provider:444"
  
  # TLS certificate configuration
  tlsSecret:
    name: "iaas-provider-client-cert"
    namespace: "spiderpool"
```

Then apply:

```bash
helm upgrade spiderpool spiderpool/spiderpool \
  --namespace spiderpool \
  -f values.yaml
```

---

## Step 4: Verify Configuration

### 4.1 Check Agent Pods

```bash
# List agent pods
kubectl get pods -n spiderpool -l app.kubernetes.io/component=spiderpool-agent

# Check agent logs for IaaS configuration
kubectl logs -n spiderpool ds/spiderpool-agent | grep -i iaas
```

**Expected output**:
```
IaaS provider configuration loaded: url=iaas-network-provider:444
IaaS TLS secret: name=iaas-provider-client-cert, namespace=spiderpool
```

### 4.2 Verify Secret Mount

```bash
# Check certificate is mounted
kubectl exec -n spiderpool ds/spiderpool-agent -- ls -la /etc/spiderpool/iaas-tls/
```

**Expected output**:
```
total 4
drwxrwxrwt 3 root root  120 Apr 27 12:00 .
drwxr-xr-x 1 root root 4096 Apr 27 10:00 ..
drwxr-xr-x 2 root root   40 Apr 27 12:00 ..2025_04_27_12_00_00
drwxr-xr-x 9 root root  180 Apr 27 12:00 ..data
drwxrwxr-x 2 root root   60 Apr 27 12:00 tls.crt
drwxrwxr-x 2 root root   60 Apr 27 12:00 tls.key
```

### 4.3 Check Controller

```bash
# Check controller logs
kubectl logs -n spiderpool deployment/spiderpool-controller | grep -i iaas
```

---

## Step 5: Validation Tests

### Test 1: Certificate Readable

```bash
# Verify certificate content
kubectl exec -n spiderpool ds/spiderpool-agent -- \
  cat /etc/spiderpool/iaas-tls/tls.crt | head -5
```

**Expected**: X.509 certificate content starting with `-----BEGIN CERTIFICATE-----`

### Test 2: Configuration Loaded in Agent/Controller

```bash
# Check Agent logs for IaaS configuration
kubectl logs -n spiderpool ds/spiderpool-agent | grep -i "iaas"

# Check Controller logs for IaaS configuration
kubectl logs -n spiderpool deployment/spiderpool-controller | grep -i "iaas"
```

**Expected output**:
```
IaaS provider configuration detected {"url": "iaas-network-provider:444"}
IaaS provider TLS configuration {"secretName": "iaas-provider-client-cert", "secretNamespace": "spiderpool"}
```

### Test 3: Configuration via ConfigMap

```bash
# Verify ConfigMap contains IaaS configuration
kubectl get configmap -n spiderpool spiderpool-conf -o yaml | grep -A10 "iaasNetworkProvider"
```

**Expected output**:
```yaml
iaasNetworkProvider:
  url: "iaas-network-provider:444"
  tlsSecret:
    name: "iaas-provider-client-cert"
    namespace: "spiderpool"
  tlsCertPath: "/etc/spiderpool/iaas-tls/tls.crt"
  tlsKeyPath: "/etc/spiderpool/iaas-tls/tls.key"
```

---

## Disabling IaaS Integration

To disable the IaaS integration:

```bash
helm upgrade spiderpool spiderpool/spiderpool \
  --namespace spiderpool \
  --set iaasNetworkProvider.url=""
```

Or in `values.yaml`:

```yaml
iaasNetworkProvider:
  url: ""
```

**Note**: When URL is empty, the TLS secret configuration is ignored and no secret mounting occurs.

---

## Troubleshooting

### Issue: Secret not found

**Error**:
```
Error: secret "iaas-provider-client-cert" not found
```

**Solution**:
1. Verify secret exists: `kubectl get secret -n spiderpool`
2. Check namespace: Ensure secret is in the same namespace as specified in `tlsSecret.namespace`
3. Create secret: Follow Step 2

### Issue: Certificate not mounted

**Symptom**: `/etc/spiderpool/iaas-tls/` directory is empty

**Solution**:
1. Check Helm values are applied: `helm get values spiderpool -n spiderpool`
2. Verify URL is not empty: `iaasNetworkProvider.url` must be set
3. Restart pods: `kubectl rollout restart daemonset/spiderpool-agent -n spiderpool`

### Issue: Agent fails to start

**Error**:
```
Failed to validate IaaS configuration: TLS secret name is required
```

**Solution**: Set both `tlsSecret.name` and `tlsSecret.namespace` when `url` is configured.

---

## Next Steps

After Phase 1 configuration is complete and validated:

1. **Monitor**: Watch for Phase 2 implementation updates
2. **API Integration**: Phase 2 will add the actual IaaS API client
3. **MAC Storage**: Returned MAC addresses from IaaS provider will be stored in SpiderEndpoint

---

## Example: Complete values.yaml

```yaml
# Spiderpool Helm values with IaaS provider configuration

spiderpoolAgent:
  image:
    tag: "v0.9.0"

spiderpoolController:
  image:
    tag: "v0.9.0"

# IaaS provider integration configuration
iaasNetworkProvider:
  # Required: IaaS provider URL (host:port)
  # Set to empty string "" to disable integration
  url: "iaas-network-provider:444"
  
  # Required when URL is set: TLS secret configuration
  tlsSecret:
    # Kubernetes secret name containing tls.crt and tls.key
    name: "iaas-provider-client-cert"
    
    # Kubernetes namespace where the secret exists
    namespace: "spiderpool"
```
