# Contract: Pod Resource Injection

## Eligibility

A Pod is eligible for `spidernet.io/eni-slot` injection when:

- Provider mode is enabled.
- ENI slot device plugin support is enabled.
- Existing Multus default-network or attachment-network annotations reference at least one VLAN-type SpiderMultusConfig whose VLAN ID is nil.
- The Pod does not already declare the configured ENI slot resource.

## Mutation

For an eligible Pod, Spiderpool injects the configured resource into the first container unless existing Spiderpool resource injection rules choose another container. The injected quantity equals the number of eligible VLAN SpiderMultusConfigs referenced by the Pod.

```yaml
resources:
  limits:
    spidernet.io/eni-slot: "2"
```

In this example, the Pod references two eligible VLAN SpiderMultusConfigs, so the injected quantity is `2`.

Extended resources are represented as limits. Kubernetes treats extended resource requests and limits consistently for scheduling when only limits are set.

## Non-Overwrite Rule

If any container already declares the configured resource, Spiderpool must not overwrite, duplicate, increment, or recalculate the quantity.

## Non-Eligible Pods

Pods that do not require provider-mode auxiliary ENIs must not receive the resource. Current provider-mode behavior for those Pods remains unchanged.

## Diagnostics

If Spiderpool cannot determine eligibility or cannot inject the resource due to invalid configuration, the admission response must explain the reason using existing webhook error conventions.
