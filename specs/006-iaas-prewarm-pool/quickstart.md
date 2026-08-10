# Quickstart: IaaS Provider Prewarm IP Pool Support

This quickstart validates the Spiderpool-side (P0) behavior of this feature
using only `kubectl` against a cluster with Spiderpool installed — no real
IaaS provider component is required, since ledger data
(`status.iaasReadyIPs`/`status.iaasFailedIPs`) can be simulated directly for
validation purposes.

## Prerequisites

- A Kubernetes cluster with Spiderpool installed (webhook + CRDs from this
  feature branch built and deployed).
- `kubectl` access with permission to create/patch `SpiderIPPool` and Pods.

## 1. Create a paired IaaS pool pair

```bash
kubectl apply -f - <<'EOF'
apiVersion: spiderpool.spidernet.io/v2beta1
kind: SpiderIPPool
metadata:
  name: node1-app-a-v4
  annotations:
    ipam.spidernet.io/iaas-pool: "true"
    ipam.spidernet.io/pair-pool: node1-app-a-v6
spec:
  ipVersion: 4
  subnet: 192.168.0.0/16
  ips: ["192.168.1.10-192.168.1.11"]
  nodeName: ["node1"]
  podAffinity:
    matchLabels: {app: app-a}
---
apiVersion: spiderpool.spidernet.io/v2beta1
kind: SpiderIPPool
metadata:
  name: node1-app-a-v6
  annotations:
    ipam.spidernet.io/iaas-pool: "true"
    ipam.spidernet.io/pair-pool: node1-app-a-v4
spec:
  ipVersion: 6
  subnet: fd00::/112
  ips: ["fd00::10-fd00::11"]
  nodeName: ["node1"]
  podAffinity:
    matchLabels: {app: app-a}
EOF
```

**Expect**: both pools admitted successfully;
`kubectl get sp node1-app-a-v4 -o jsonpath='{.metadata.labels}'` shows
`ipam.spidernet.io/iaas-pool: "true"` even though only the annotation was set.

## 2. Verify pairing validation rejects bad pairings

```bash
# Self-reference — must be rejected
kubectl annotate sppool node1-app-a-v4 ipam.spidernet.io/pair-pool=node1-app-a-v4 --overwrite
```

**Expect**: request rejected with a validation error citing self-referential
pairing.

```bash
# Capacity mismatch (v4 > v6) — must be rejected
kubectl patch sppool node1-app-a-v4 --type=merge -p \
  '{"spec":{"ips":["192.168.1.10-192.168.1.13"]}}'  # 4 v4 addresses vs 2 v6 addresses
```

**Expect**: rejected — v4 static capacity (4) exceeds v6 static capacity (2).

## 3. Simulate provider-written ledger status

```bash
kubectl patch sppool node1-app-a-v4 --type=merge --subresource=status -p '{
  "status": {
    "iaasReadyIPs": [
      {"ipv4": "192.168.1.10", "ipv6": "fd00::10", "mac": "fa:16:3e:aa:bb:cc", "vlanID": 2014}
    ],
    "iaasFailedIPs": [
      {"ipv4": "192.168.1.11", "ipv6": "fd00::11"}
    ]
  }
}'
```

Note: `spec.ips` on `node1-app-a-v4` already lists both `192.168.1.10` and
`192.168.1.11` (from step 1) — the ledger does not replace `spec.ips`, it only
narrows which of those already-declared addresses are currently allocatable.

## 4. Request a single-family IP and verify auto-completion + atomic pairing

```bash
kubectl apply -f - <<'EOF'
apiVersion: v1
kind: Pod
metadata:
  name: app-a-0
  labels: {app: app-a}
  annotations:
    ipam.spidernet.io/ippool: '{"ipv4": ["node1-app-a-v4"]}'
spec:
  nodeName: node1
  containers:
  - name: app-a
    image: busybox
    command: ["sleep", "3600"]
EOF
```

**Expect**:
- The Pod is allocated `192.168.1.10` (the only address present in both
  `spec.ips` and `status.iaasReadyIPs`) for IPv4.
- Even though the Pod annotation named only the v4 pool, the resolved
  candidate pools included `node1-app-a-v6` (auto-completed), and — because
  this feature's clarified behavior allocates only the requested family for a
  single-stack Pod — no v6 address is force-allocated to this single-stack
  Pod; `fd00::10` remains available in the ledger for a future dual-stack
  Pod.
- `192.168.1.11`/`fd00::11` are never returned: `192.168.1.11` is in
  `spec.ips` but not in `status.iaasReadyIPs` (recorded in `iaasFailedIPs`
  instead), so it fails the readiness intersection.

## 5. Verify existing non-IaaS pools are unaffected

```bash
kubectl apply -f - <<'EOF'
apiVersion: spiderpool.spidernet.io/v2beta1
kind: SpiderIPPool
metadata:
  name: plain-pool-v4
spec:
  ipVersion: 4
  subnet: 10.0.0.0/24
  ips: ["10.0.0.10-10.0.0.20"]
EOF
```

**Expect**: pool creation, no label added, no pairing validation triggered,
and IP allocation from this pool behaves exactly as before this feature (no
`iaas-pool` label means `status.iaasReadyIPs`/`status.iaasFailedIPs` are never
consulted).

## Cleanup

```bash
kubectl delete pod app-a-0
kubectl delete sppool node1-app-a-v4 node1-app-a-v6 plain-pool-v4
```
