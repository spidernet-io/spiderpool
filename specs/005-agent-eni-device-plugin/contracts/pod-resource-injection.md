# Contract: Pod Resource Injection

## Eligibility

A Pod is eligible for `spidernet.io/sub-eni` injection when:

- Provider mode is enabled.
- `spiderpoolAgent.networkResourcePlugin.enabled=true`.
- `spiderpoolController.podResourceInject.enabled=true`.
- `resourceAdvertisement.subENI.rules` contains at least one rule.
- Existing Multus default-network or attachment-network annotations reference at least one VLAN-type SpiderMultusConfig whose VLAN ID is nil.
- The Pod does not already declare the configured sub-ENI resource.

A Pod is eligible for `spidernet.io/<master>-nic` injection when:

- `spiderpoolAgent.networkResourcePlugin.enabled=true`.
- `spiderpoolController.podResourceInject.enabled=true`.
- `resourceAdvertisement.masterNIC.rules` contains at least one rule.
- The Pod's selected Spiderpool network configuration requires a concrete master NIC.
- The Pod does not already declare the matching master NIC resource.

## Mutation

For an eligible Pod, Spiderpool injects the configured resource into the first container unless existing Spiderpool resource injection rules choose another container. The injected `spidernet.io/sub-eni` quantity equals the number of eligible VLAN SpiderMultusConfigs referenced by the Pod.

```yaml
resources:
  limits:
    spidernet.io/sub-eni: "2"
```

In this example, the Pod references two eligible VLAN SpiderMultusConfigs, so the injected quantity is `2`.

For master NIC injection, Spiderpool injects the concrete master NIC resource with quantity `1`.

```yaml
resources:
  limits:
    spidernet.io/eth1-nic: "1"
```

Extended resources are represented as limits. Kubernetes treats extended resource requests and limits consistently for scheduling when only limits are set.

## Non-Overwrite Rule

If any container already declares a configured Spiderpool network resource, Spiderpool must not overwrite, duplicate, increment, or recalculate the quantity.

## Non-Eligible Pods

Pods that do not require provider-mode auxiliary ENIs must not receive `spidernet.io/sub-eni`. Pods that do not require a concrete master NIC must not receive `spidernet.io/<master>-nic`. Current provider-mode behavior for those Pods remains unchanged.

## Diagnostics

If Spiderpool cannot determine eligibility or cannot inject the resource due to invalid configuration, the admission response must explain the reason using existing webhook error conventions.
