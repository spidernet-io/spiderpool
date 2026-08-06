# Contract: SpiderIPPool IaaS-Pool Extension

This feature has no HTTP/gRPC/CLI-facing API surface of its own; its
"contract" is the Kubernetes CRD field/annotation shape shared between
Spiderpool (writer of `status.allocatedIPs`, reader of `status.iaasIPs`) and
the external, private IaaS provider controller (writer of `status.iaasIPs`/
`status.conditions`). This document is the authoritative shape both sides must
honor. It is the CRD-field equivalent of an API contract for this project type
(Kubernetes controller + IPAM library), per plan.md step "Define interface
contracts... for the project type".

## Annotations (input contract — operator/provider writes, Spiderpool reads & validates)

```yaml
metadata:
  annotations:
    ipam.spidernet.io/iaas-pool: "true"            # required to opt a pool into all behavior in this feature
    ipam.spidernet.io/pair-pool: "<sibling-pool-name>"  # optional; required only for dual-stack paired pools
```

- `iaas-pool` MUST be exactly the string `"true"` to enable the feature's
  behavior for a pool (any other value, including absence, means "not an IaaS
  pool" — full backward compatibility, FR-006).
- `pair-pool` MUST name another `SpiderIPPool` object (may not yet exist at
  admission time). Validation rules are enforced only once both pools exist
  (see below).

## Label (output contract — Spiderpool writes, provider/external watchers read)

```yaml
metadata:
  labels:
    ipam.spidernet.io/iaas-pool: "true"   # kept in sync with the annotation of the same key by the mutating webhook
```

Consumers (e.g., the external provider's `informer`) MUST use this label
(not the annotation) for server-side watch filtering, since Kubernetes watch
label selectors are server-side but annotation selectors are not.

## Validating Webhook Contract (SpiderIPPool create/update)

| Condition | Result |
|---|---|
| `pair-pool` == own pool name | `403 Forbidden` / `field.Invalid` — self-reference not allowed |
| `pair-pool` refers to an existing pool with the same `spec.ipVersion` | Rejected — same-version pairing not allowed |
| `pair-pool` refers to a pool that does not exist | Allowed (no rejection) |
| Both pools exist, v4 pool's (`spec.ips` - `spec.excludeIPs`) count > v6 pool's | Rejected |
| Both pools exist, v4 count <= v6 count | Allowed |
| Both pools exist, `spec.nodeName` or `spec.podAffinity` differ | Rejected |
| Both pools exist, `spec.nodeName`/`spec.podAffinity` identical | Allowed |
| No `pair-pool` annotation at all | No pairing validation applied (existing behavior) |

## Status Field Contract (`status.iaasIPs`, `status.conditions`)

**Writer**: external IaaS provider controller, via Server-Side Apply with a
dedicated field manager (per proposal §3.3 single-writer principle).
**Reader**: Spiderpool IPAM (`AllocateIP`), read-only.

```yaml
status:
  allocatedIPs: '{...}'          # existing, Spiderpool-owned, unchanged shape
  totalIPCount: 64               # existing, unchanged
  allocatedIPCount: 12           # existing, unchanged
  iaasIPs:                       # NEW, provider-owned
    - ipv4: 192.168.1.10
      ipv6: fd00::10             # only present for paired pools; absent for single-stack pools
      mac: "fa:16:3e:xx:xx:xx"
      vlanID: 2014
      phase: Ready                # Ready | NotReady | Releasing
      lastError: ""
  conditions:                    # NEW, provider-owned
    - type: IaasReady
      status: "True"              # "True" only when ALL iaasIPs entries are Ready
      reason: AllReady             # or e.g. PartialPrewarmFailed
      message: "64/64 ready"
```

**Consumption rules Spiderpool MUST follow** (this is the binding part of the
contract for this feature's implementation):

1. Never write to `status.iaasIPs` or `status.conditions` — these fields are
   provider-owned. Spiderpool's own SSA/update calls MUST NOT include these
   fields (avoid field-manager conflicts).
2. An entry is "available for allocation" iff `phase == "Ready"` AND its
   `ipv4`/`ipv6` value(s) are absent from the pool's own
   `status.allocatedIPs` map (existing Spiderpool-owned occupancy source of
   truth).
3. If a pool's `status.iaasIPs` is empty/absent, Spiderpool MUST use the
   pre-existing allocation algorithm unchanged (this is the primary backward
   -compatibility guarantee, FR-011).
4. `status.conditions[].type == "IaasReady"` is observational only — it MUST
   NOT gate or block allocation decisions (spec §5.3 / FR requirement: gate
   per-IP, not per-pool).

## Backward Compatibility Statement

Every field/annotation/label in this contract is additive and optional.
Existing `SpiderIPPool` objects, existing Helm-rendered manifests, and
existing controllers/clients that do not know about `iaasIPs`/`conditions`
continue to function unchanged. CRD schema changes MUST be validated with
`make manifests generate-k8s-api` producing only additive diffs to
`charts/spiderpool/crds/spiderpool.spidernet.io_spiderippools.yaml`.
