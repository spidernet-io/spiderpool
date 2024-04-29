# SpiderClaimParameter

A SpiderClaimParameter resource is used to describe the resourceclaim and affects the generated CDI file. this CRD only works when the [dra feature](https://kubernetes.io/docs/concepts/scheduling-eviction/dynamic-resource-allocation/) is enabled.

## Sample YAML

```yaml
apiVersion: spiderpool.spidernet.io/v2beta1
kind: SpiderClaimParameter
metadata:
  name: demo
  namespace: default
  annotations:
    dra.spidernet.io/cdi-version: 0.6.0
spec:
  netResources:
    spidernet.io/shared-rdma-device: 1
  ippools:
  - pool
```

## Spidercoordinators definition

### Metadata

| Field     | Description                                       | Schema | Validation |
|-----------|---------------------------------------------------|--------|------------|
| name      | The name of this Spidercoordinators resource      | string | required   |

### Spec

This is the Spidercoordinators spec for users to configure.

| Field              | Description                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                              | Schema  | Validation | Values                                        | Default                      |
|--------------------|------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------|---------|------------|-----------------------------------------------|------------------------------|
| netResources               |     Used for device-plugin declaration resources                                                                                                                                                                                                                                                                   | map[string]string  | optional    |         nil                 | nil                         |
ippools                  |  A list of subnets used by the pod for scheduling purposes. | []string | optional |  []string{} | empty |
