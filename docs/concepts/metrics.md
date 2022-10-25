# Metric

Spiderpool supports metrics reporting

## spiderpool controller

The metrics of spiderpool controller is set by the following pod environment:

| environment                   | description    | default |
|-------------------------------|----------------|---------|
| SPIDERPOOL_ENABLED_METRIC     | enable metrics | false   |
| SPIDERPOOL_METRIC_HTTP_PORT   | metrics port   | 5721    |

## spiderpool agent

The metrics of spiderpool agent is set by the following pod environment:

| environment                   | description    | default |
|-------------------------------|----------------|---------|
| SPIDERPOOL_ENABLED_METRIC     | enable metrics | false   |
| SPIDERPOOL_METRIC_HTTP_PORT   | metrics port   | 5721    |

## Metric reference

Refer to [metrics](./../../pkg/metric/README.md) for details.
