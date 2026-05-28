# Data Model: IaaS Network Provider Integration

**Feature**: IaaS Network Provider Integration  
**Phase**: Phase 1 - Configuration Infrastructure

---

## 1. Configuration Entities

### IaaSProviderConfig

The top-level configuration for IaaS provider integration.

| Field | Type | Required | Default | Description |
|-------|------|----------|---------|-------------|
| URL | string | No | "" | IaaS provider endpoint URL (host:port). Empty disables integration. |
| TLSSecret | TLSSecretConfig | No | {} | TLS certificate configuration for mTLS authentication. Required if URL is set. |

### TLSSecretConfig

TLS certificate secret reference.

| Field | Type | Required | Default | Description |
|-------|------|----------|---------|-------------|
| Name | string | Yes* | "" | Kubernetes secret name containing tls.crt and tls.key |
| Namespace | string | Yes* | "" | Kubernetes namespace where the secret exists |

\* Required when `URL` is non-empty.

---

## 2. Validation Rules

### Rule 1: Empty URL Disables Integration

```
IF URL == "" THEN
    IaaS integration is disabled
    TLSSecret is ignored
    No validation required
END IF
```

### Rule 2: Non-Empty URL Requires TLS Config

```
IF URL != "" THEN
    IF TLSSecret.Name == "" THEN
        ERROR: "TLS secret name is required when IaaS provider URL is configured"
    END IF
    
    IF TLSSecret.Namespace == "" THEN
        ERROR: "TLS secret namespace is required when IaaS provider URL is configured"
    END IF
END IF
```

### Rule 3: Secret Format Validation

The referenced secret must contain:
- `tls.crt`: Base64-encoded X.509 client certificate
- `tls.key`: Base64-encoded RSA/EC private key

```
VALIDATE secret:
    - Secret exists in specified namespace
    - Secret contains key "tls.crt"
    - Secret contains key "tls.key"
    - Certificate is valid (not expired)
    - Key matches certificate
```

---

## 3. Environment Variable Mapping

Configuration is passed via environment variables:

| Environment Variable | Source Field | Example Value |
|----------------------|--------------|---------------|
| `SPIDERPOOL_IAAS_PROVIDER_URL` | `iaasNetworkProvider.url` | `iaas-provider:444` |
| `SPIDERPOOL_IAAS_TLS_SECRET_NAME` | `iaasNetworkProvider.tlsSecret.name` | `iaas-provider-client-cert` |
| `SPIDERPOOL_IAAS_TLS_SECRET_NAMESPACE` | `iaasNetworkProvider.tlsSecret.namespace` | `spiderpool` |
| `SPIDERPOOL_IAAS_TLS_CERT_PATH` | Hardcoded | `/etc/spiderpool/iaas-tls/tls.crt` |
| `SPIDERPOOL_IAAS_TLS_KEY_PATH` | Hardcoded | `/etc/spiderpool/iaas-tls/tls.key` |

---

## 4. Kubernetes Resource Model

### User-Created Secret

```yaml
apiVersion: v1
kind: Secret
metadata:
  name: <tlsSecret.name>
  namespace: <tlsSecret.namespace>
type: kubernetes.io/tls
data:
  tls.crt: <base64-encoded-certificate>
  tls.key: <base64-encoded-private-key>
```

### Spiderpool Agent Volume Mount

```yaml
apiVersion: apps/v1
kind: DaemonSet
spec:
  template:
    spec:
      containers:
      - name: spiderpool-agent
        volumeMounts:
        - name: iaas-tls
          mountPath: /etc/spiderpool/iaas-tls
          readOnly: true
      volumes:
      - name: iaas-tls
        secret:
          secretName: <tlsSecret.name>
          namespace: <tlsSecret.namespace>
          items:
          - key: tls.crt
            path: tls.crt
          - key: tls.key
            path: tls.key
```

### Spiderpool Controller Volume Mount

Same structure as Agent.

---

## 5. State Transitions

### Configuration Loading State Machine

```
┌─────────────────┐
│  Start/Reload   │
└────────┬────────┘
         │
         ▼
┌─────────────────┐
│  Load Config    │
│  (env vars)     │
└────────┬────────┘
         │
         ▼
    ┌─────────┐
    │ URL set?│
    └────┬────┘
         │
      ┌──┴──┐
    No│     │Yes
      │     │
      ▼     ▼
┌─────────┐ ┌─────────────────┐
│ Disabled│ │ Validate Secret │
│  State  │ │   Reference     │
└─────────┘ └────────┬────────┘
                     │
                ┌────┴────┐
              OK│         │Error
                │         │
                ▼         ▼
        ┌─────────────┐ ┌──────────────┐
        │   Enabled   │ │    Error     │
        │    State    │ │     State    │
        │ (Ready for  │ │ (Log error,  │
        │   Phase 2)  │ │  keep retry) │
        └─────────────┘ └──────────────┘
```

---

## 6. Error States

| Error Code | Description | Recovery |
|------------|-------------|----------|
| `CONFIG_INVALID` | Missing required TLS secret config | Fix Helm values and redeploy |
| `SECRET_NOT_FOUND` | Referenced secret doesn't exist | Create secret or fix reference |
| `SECRET_INVALID` | Secret missing tls.crt or tls.key | Recreate secret with correct keys |
| `CERT_EXPIRED` | TLS certificate is expired | Update secret with valid certificate |

---

## 7. Phase 2 Data Model (Future)

For reference, Phase 2 will add:

### IaaSAllocateRequest

```json
{
  "podName": "p1",
  "podNamespace": "ns1",
  "podUID": "1234567890",
  "nodeName": "worker-01",
  "iaasIPsAllocationRequest": [
    {
      "ipAddress": "10.0.0.10",
      "subnet": "10.0.0.0/24",
      "parentNicMac": "fa:16:3e:xx:xx:xx"
    }
  ]
}
```

### IaaSAllocateResponse

```json
{
  "podName": "p1",
  "podNamespace": "ns1",
  "nodeName": "worker-01",
  "iaasIPsAllocationResponse": [
    {
      "parentNicMac": "fa:16:3e:xx:xx:xx",
      "subnet": "10.251.0.0/24",
      "ipAddress": "10.0.0.10",
      "macAddress": "fa:16:3e:xx:xx:xx",
      "vlanId": 100
    }
  ]
}
```

These will be used by the API client implemented in Phase 2.
