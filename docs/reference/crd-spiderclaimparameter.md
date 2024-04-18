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
  rdmaAcc: false
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
| rdmaAcc               |  TODO                                                                                                                                                                                                                                                                      | bool  | optional    |         true,false                 | false                         |
