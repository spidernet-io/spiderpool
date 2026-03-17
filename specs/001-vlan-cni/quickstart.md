# Quick Start: VLAN CNI in Spiderpool

**Generated**: 2026-03-17  
**Feature**: VLAN CNI Support (specs/001-vlan-cni/spec.md)

## Prerequisites

Before using VLAN CNI with Spiderpool, ensure:

1. **Spiderpool is installed** and running in your cluster
2. **VLAN CNI plugin** is installed on all nodes (`/opt/cni/bin/vlan`)
3. **Multus CNI** is installed and configured
4. **Parent network interfaces** (e.g., eth0) are configured and support VLAN tagging on nodes

## Installation Check

```bash
# Check if VLAN CNI plugin exists on nodes
kubectl get nodes -o name | head -1 | xargs -I {} \
  kubectl debug {} -it --image=alpine -- \
  ls /opt/cni/bin/ | grep vlan

# Check Spiderpool version
kubectl get deployment -n spiderpool spiderpool-controller -o jsonpath='{.spec.template.spec.containers[0].image}'
```

## Basic Usage

### 1. Single NIC VLAN Configuration

Create a simple VLAN network using a single parent interface:

```yaml
apiVersion: spiderpool.spidernet.io/v2beta1
kind: SpiderMultusConfig
metadata:
  name: vlan-100
  namespace: default
spec:
  cniType: vlan
  vlan:
    master: ["eth0"]
    vlanID: 100
    ippools:
      ipv4: ["vlan100-pool"]
```

Apply and verify:

```bash
kubectl apply -f vlan-100.yaml

# Verify SpiderMultusConfig was created
kubectl get spidermultusconfig vlan-100

# Verify NetworkAttachmentDefinition was generated
kubectl get net-attach-def vlan-100
```

### 2. Use in a Pod

```yaml
apiVersion: v1
kind: Pod
metadata:
  name: test-vlan-pod
  annotations:
    k8s.v1.cni.cncf.io/networks: default/vlan-100
spec:
  containers:
  - name: test
    image: alpine
    command: ["sleep", "infinity"]
```

Verify the Pod has the VLAN interface:

```bash
kubectl exec test-vlan-pod -- ip link show
# Should show interface with VLAN tag
```

## Advanced Usage

### Multi-NIC Bond Configuration

For high availability, create a VLAN network on a bond device:

```yaml
apiVersion: spiderpool.spidernet.io/v2beta1
kind: SpiderMultusConfig
metadata:
  name: vlan-bond-200
  namespace: default
spec:
  cniType: vlan
  vlan:
    master: ["eth0", "eth1"]
    vlanID: 200
    mtu: 1500
    bond:
      name: "bond0"
      mode: 4  # 802.3ad LACP
    ippools:
      ipv4: ["vlan200-pool"]
```

### RDMA-Accelerated VLAN

For AI/ML workloads requiring RDMA:

```yaml
apiVersion: spiderpool.spidernet.io/v2beta1
kind: SpiderMultusConfig
metadata:
  name: vlan-rdma
  namespace: default
  annotations:
    ipam.spidernet.io/pod-resource-inject: "true"
spec:
  cniType: vlan
  vlan:
    master: ["eth0"]
    vlanID: 300
    rdmaResourceName: "rdma_shared_device"
    ippools:
      ipv4: ["vlan300-pool"]
```

## Generated NAD Example

For single NIC, the informer generates:

```json
{
  "cniVersion": "0.3.1",
  "name": "vlan-100",
  "plugins": [
    {
      "type": "vlan",
      "master": "eth0",
      "vlanId": 100,
      "ipam": {
        "type": "spiderpool",
        "defaultIPv4IPPool": ["vlan100-pool"]
      }
    }
  ]
}
```

For multi-NIC with bond:

```json
{
  "cniVersion": "0.3.1",
  "name": "vlan-bond-200",
  "plugins": [
    {
      "type": "ifacer",
      "interfaces": ["eth0", "eth1"],
      "bond": {
        "name": "bond0",
        "mode": 4
      }
    },
    {
      "type": "vlan",
      "master": "bond0",
      "vlanId": 200,
      "mtu": 1500,
      "ipam": {
        "type": "spiderpool"
      }
    }
  ]
}
```

## Validation Errors

Common validation errors and fixes:

### VLAN ID out of range

```
Error: vlanID must be in range [0, 4094]
```

**Fix**: Ensure vlanID is between 0 and 4094.

### Missing bond configuration

```
Error: bond configuration required when using multiple masters
```

**Fix**: Add bond config when specifying multiple master interfaces:

```yaml
bond:
  name: "bond0"
  mode: 4
```

### Missing vlan Config

```
Error: vlan required when cniType is vlan
```

**Fix**: Ensure vlan block is present when cniType is vlan.

## Troubleshooting

### Pod fails to start with VLAN network

1. **Check VLAN CNI plugin exists on node**:
   ```bash
   ls /opt/cni/bin/vlan
   ```

2. **Verify parent interface exists**:
   ```bash
   ip link show eth0
   ```

3. **Check VLAN support**:
   ```bash
   ip link add link eth0 name eth0.100 type vlan id 100
   ```

4. **View SpiderMultusConfig status**:
   ```bash
   kubectl describe spidermultusconfig vlan-100
   ```

5. **Check NAD configuration**:
   ```bash
   kubectl get net-attach-def vlan-100 -o jsonpath='{.spec.config}' | jq
   ```

### No IP assigned to VLAN interface

1. **Verify IPPool exists and has available IPs**:
   ```bash
   kubectl get spiderippool vlan100-pool
   ```

2. **Check Spiderpool agent logs**:
   ```bash
   kubectl logs -n spiderpool -l app=spiderpool-agent
   ```

## Next Steps

- See [full documentation](../docs/usage/spider-multus-config.md) for detailed configuration options
- Learn about [IPPool configuration](../docs/usage/spider-ippool.md) for IP management
- Review [troubleshooting guide](../docs/usage/debug.md) for common issues
