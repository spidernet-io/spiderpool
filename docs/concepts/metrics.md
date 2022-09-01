# Metric

Spiderpool supports metrics reporting

## spiderpool controller

the metric of spiderpool controller is set by following pod environment

| environment                   | description    | default |
|-------------------------------|----------------|---------|
| SPIDERPOOL_ENABLED_METRIC     | enable metrics | false   |
| SPIDERPOOL_METRIC_HTTP_PORT   | metrics port   | 5721    |

## spiderpool agent

the metric of spiderpool agent is set by following pod environment

| environment                   | description    | default |
|-------------------------------|----------------|---------|
| SPIDERPOOL_ENABLED_METRIC     | enable metrics | false   |
| SPIDERPOOL_METRIC_HTTP_PORT   | metrics port   | 5721    |

## Metric Reference

Refer to [metrics](./../../pkg/metric/README.md) for details.
