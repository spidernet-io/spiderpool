# Chart Management

the '/docs' of branch 'webserver' is used by github page

each version of chart package will be automatically created by CI,
they will pushed to '/docs/chart' of branch 'webserver'.

the index.yaml of chart registry will also be updated by CI, and published to '/docs' of branch 'webserver'.

so, you could use following command to get the chart

```
helm repo add spiderpool  https://spidernet-io.github.io/spiderpool
```
