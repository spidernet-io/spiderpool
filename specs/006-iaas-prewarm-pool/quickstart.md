# Quickstart: IaaS Provider Prewarm IP Pool Support

This quickstart validates the Spiderpool-side (P0) behavior of this feature
using only `kubectl` against a cluster with Spiderpool installed — no real
IaaS provider component is required, since metadata
(`status.ipMetaData`) can be simulated directly for
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
    ipam.spidernet.io/iaas-provider: "huaweicloud"
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
    ipam.spidernet.io/iaas-provider: "huaweicloud"
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
`ipam.spidernet.io/iaas-provider: "huaweicloud"` even though only the
annotation was set.

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

## 3. Simulate provider-written metadata status

```bash
GEN=$(kubectl get sppool node1-app-a-v4 -o jsonpath='{.metadata.generation}')
kubectl patch sppool node1-app-a-v4 --type=merge --subresource=status -p "{
  \"status\": {
    \"ipMetaData\": {
      \"metadata\": \"{\\\"parentNic\\\":\\\"eth0\\\",\\\"192.168.1.10\\\":{\\\"ipv6\\\":\\\"fd00::10\\\",\\\"mac\\\":\\\"fa:16:3e:aa:bb:cc\\\",\\\"vlan\\\":2014}}\",
      \"observedGeneration\": ${GEN},
      \"readyIPCount\": 1,
      \"unreadyIPCount\": 1
    }
  }
}"
```

Note: `spec.ips` on `node1-app-a-v4` already lists both `192.168.1.10` and
`192.168.1.11` (from step 1). The metadata string decodes to a map and does
not replace `spec.ips`; it only narrows which addresses are allocatable.
`observedGeneration` binds that decoded map to the current spec generation.
`192.168.1.11` has no decoded entry, expressing failed/pending prewarm.

## 4. Verify spec/status generation gating

Change the desired addresses without publishing new provider status:

```bash
kubectl patch sppool node1-app-a-v4 --type=merge -p \
  '{"spec":{"ips":["192.168.1.10-192.168.1.12"]}}'

kubectl get sppool node1-app-a-v4 \
  -o jsonpath='generation={.metadata.generation} observed={.status.ipMetaData.observedGeneration}{"\n"}'
```

**Expect**: `generation` is greater than `observed`. New allocation attempts
from this pool fail closed with a retryable metadata-not-reconciled error.
After a provider (or this simulation) atomically publishes the new metadata,
counters, and matching `observedGeneration`, the agent receives the status
Update, replaces its parsed cache snapshot, and allocation resumes.

Before continuing, republish metadata for the current generation using the
Step 3 command (and set `unreadyIPCount` appropriately for the expanded pool).

## 5. Request a single-family IP and verify atomic pairing

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
  `spec.ips` and the decoded `status.ipMetaData.metadata`) for IPv4.
- Even though the Pod annotation named only the v4 pool: in a dual-stack
  cluster the pair-or-nothing allocation (`AllocateIPPair`) returns both
  `192.168.1.10` and its paired `fd00::10` from the same metadata entry and
  records the v6 side into `node1-app-a-v6`'s `status.allocatedIPs`; in a
  single-stack (IPv4-only) cluster only the v4 address is allocated and
  `fd00::10` remains available in the metadata for a future dual-stack
  Pod. The sibling v6 pool is never a standalone v6 candidate.
- `192.168.1.11`/`fd00::11` are never returned: `192.168.1.11` is in
  `spec.ips` but has no `metadata` entry, so it fails the readiness
  intersection.

## 6. Verify existing non-IaaS pools are unaffected

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
`iaas-provider` label means `status.ipMetaData` is never consulted).

## 7. Global pool mode (realtime + sticky sub-ENI cache)

Create a global pool: `iaas-provider` label present, **no** `spec.nodeName`.

```bash
kubectl apply -f - <<EOF
apiVersion: spiderpool.spidernet.io/v2beta1
kind: SpiderIPPool
metadata:
  name: app-b-global-v4
  labels:
    ipam.spidernet.io/iaas-provider: "huaweicloud"
spec:
  ipVersion: 4
  subnet: 10.7.0.0/24
  ips: ["10.7.0.10-10.7.0.20"]
EOF
```

Simulate provider-written schema v2 metadata (`scope: ""` marks global mode;
per-entry `node` records where each sub-ENI is bound):

```bash
kubectl patch sppool app-b-global-v4 --subresource=status --type=merge -p '
status:
  ipMetaData:
    observedGeneration: 1
    metadata: |
      {"scope":"","parentNic":"eth0","ips":{
        "10.7.0.10":{"mac":"fa:16:3e:00:00:10","vlan":2010,"node":"node-1"},
        "10.7.0.11":{"mac":"fa:16:3e:00:00:11","vlan":2011,"node":"node-2"},
        "10.7.0.12":{"mac":"fa:16:3e:00:00:12","vlan":-1,"node":"node-2"}}}
'
```

**Hit walkthrough**: a Pod scheduled to `node-1` gets `10.7.0.10` with its
cached MAC/VLAN and **no** provider Allocate call (the entry is bound locally
with a trustworthy VLAN).

**Miss walkthrough**: a Pod scheduled to `node-3` has no local entry, so the
cold path runs: unbound addresses (`10.7.0.13+`, no entry yet) are preferred
over stealing `10.7.0.11` from `node-2`, and the synchronous provider
Allocate RPC supplies the authoritative MAC/VLAN.

**Detaching guard**: `10.7.0.12` has `vlan: -1` while still bound to
`node-2` — the provider is reclaiming it. It is never allocatable, neither as
a hit nor as a cold-path candidate, until the provider finishes the detach
(drops `node`) or re-binds it.

**Mode invariants**: a `scope: ""` payload on a pool that sets
`spec.nodeName`, or a v2 `ips` payload without a `scope` key, fails closed
with `ErrIPMetadataNotReady`.

**DEL stickiness**: deleting the Pod releases the IP in Spiderpool but keeps
the cloud-side sub-ENI bound to the node (no provider Release call), so the
next Pod on that node hits the cache.

## Cleanup

```bash
kubectl delete pod app-a-0
kubectl delete sppool node1-app-a-v4 node1-app-a-v6 plain-pool-v4 app-b-global-v4
```
