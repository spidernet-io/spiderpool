# Chart Management

the '/' of branch 'github_pages' is used as github page

each version of chart package will be automatically created by CI,
they will be pushed to '/chart' of branch 'github_pages'.

the '/index.yaml' of branch 'github_pages' will also be updated by CI.

so, you could use following command to get the chart

```
helm repo add spiderpool  https://spidernet-io.github.io/spiderpool
```
