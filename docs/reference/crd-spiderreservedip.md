# SpiderReservedIP

A SpiderReservedIP resource represents a collection of IP addresses that Spiderpool expects not to be allocated.

For details on using this CRD, please read the [SpiderReservedIP guide](./../usage/reserved-ip.md).

## Sample YAML

```yaml
apiVersion: spiderpool.spidernet.io/v2beta1
kind: SpiderIPPool
metadata:
  name: exclude-ips
spec:
  subnet: 172.18.41.0/24
  ips:
    - 172.18.41.40-172.18.41.44
    - 172.18.41.46-172.18.41.50
```

## SpiderReservedIP definition

### Metadata

| Field | Description                                | Schema | Validation |
|-------|--------------------------------------------|--------|------------|
| name  | the name of this SpiderReservedIP resource | string | required   |

### Spec

This is the SpiderReservedIP spec for users to configure.

| Field             | Description                                           | Schema                                   | Validation | Values                                   |
|-------------------|-------------------------------------------------------|------------------------------------------|------------|------------------------------------------|
| ipVersion         | IP version of this resource                           | int                                      | optional   | 4,6                                      |
| ips               | IP ranges for this resource that we expect not to use | list of strings                          | optional   | array of IP ranges and single IP address |
